package fractaltree

import (
	"cmp"
	"iter"
	"slices"
	"sync"
)

// Tree is the interface implemented by both BETree (in-memory) and
// DiskBETree (with disk flush hooks).
type Tree[K any, V any] interface {
	// Put inserts or overwrites the value for key.
	Put(key K, value V)

	// Get returns the value for key and true, or the zero value and false.
	Get(key K) (V, bool)

	// Delete removes key. Returns true if the key existed.
	Delete(key K) bool

	// DeleteRange removes all keys in [lo, hi). Returns the count removed.
	DeleteRange(lo, hi K) int

	// Contains reports whether key exists in the tree.
	Contains(key K) bool

	// Len returns the number of key-value pairs in the tree.
	Len() int

	// Clear removes all entries from the tree.
	Clear()

	// Upsert applies fn atomically to the value at key.
	Upsert(key K, fn UpsertFn[V])

	// PutIfAbsent inserts value only if key does not exist. Returns true if inserted.
	PutIfAbsent(key K, value V) bool

	// All returns an iterator over all key-value pairs in ascending order.
	All() iter.Seq2[K, V]

	// Ascend returns an iterator over all key-value pairs in ascending order.
	Ascend() iter.Seq2[K, V]

	// Descend returns an iterator over all key-value pairs in descending order.
	Descend() iter.Seq2[K, V]

	// Range returns an iterator over keys in [lo, hi) in ascending order.
	Range(lo, hi K) iter.Seq2[K, V]

	// DescendRange returns an iterator over keys in (lo, hi] in descending order.
	DescendRange(hi, lo K) iter.Seq2[K, V]

	// Cursor returns a positioned cursor for manual bidirectional iteration.
	Cursor() *Cursor[K, V]

	// Close releases resources held by the tree.
	Close() error
}

// BETree is an in-memory B-epsilon-tree. It is safe for concurrent use.
type BETree[K any, V any] struct {
	root           *node[K, V]
	cmp            func(K, K) int
	params         treeParams
	size           int
	pendingDeletes int // count of buffered MsgDelete not yet applied to leaves
	closed         bool
	mu             sync.RWMutex
}

// New creates a BETree for keys that satisfy [cmp.Ordered].
// The comparator is derived automatically via [cmp.Compare].
func New[K cmp.Ordered, V any](opts ...Option) (*BETree[K, V], error) {
	return NewWithCompare[K, V](cmp.Compare, opts...)
}

// NewWithCompare creates a BETree with a user-supplied comparator.
// The comparator must return a negative value when a < b, zero when a == b,
// and a positive value when a > b.
func NewWithCompare[K any, V any](compare func(K, K) int, opts ...Option) (*BETree[K, V], error) {
	if compare == nil {
		return nil, ErrNilCompare
	}

	o := resolveOptions(opts)
	if err := validateOptions(o); err != nil {
		return nil, err
	}

	params := deriveParams(o)
	t := &BETree[K, V]{
		cmp:    compare,
		params: params,
	}
	t.root = newLeaf[K, V](params.leafCap)
	return t, nil
}

// Put inserts or overwrites the value for key.
func (t *BETree[K, V]) Put(key K, value V) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.putLocked(key, value, false)
}

// putLocked is the lock-free core of Put. Caller must hold t.mu.
// When knownNew is true the caller has already verified the key does not exist
// and the leaf check is skipped.
func (t *BETree[K, V]) putLocked(key K, value V, knownNew bool) {
	if t.root.leaf {
		if t.root.leafInsert(key, value, t.cmp) {
			t.size++
		}
		if len(t.root.keys) > t.params.leafCap {
			t.splitRoot()
		}
		return
	}

	// Fast path (no pending deletes): check only the leaves — skips the
	// expensive O(bufferCap) linear buffer scan at each internal node.
	// Duplicate-key messages still in buffers are corrected in applyToLeaf
	// via the counted flag.
	//
	// Slow path (pending deletes in buffers): fall back to full getFromNode
	// because a buffered MsgDelete may have logically removed a key that
	// existsInLeaf would still see in the leaf.
	var isNew bool
	switch {
	case knownNew:
		isNew = true
	case t.pendingDeletes > 0:
		_, exists := t.getFromNode(t.root, key)
		isNew = !exists
	default:
		isNew = !t.existsInLeaf(t.root, key)
	}
	if isNew {
		t.size++
	}

	t.root.appendToBuffer(Message[K, V]{Kind: MsgPut, Key: key, Value: value, counted: isNew})
	if len(t.root.buffer) > t.params.bufferCap {
		t.flushNode(t.root)
		if len(t.root.children) > t.params.fanout {
			t.splitRoot()
		}
	}
}

// existsInLeaf traverses to the leaf that would hold key, checking only the
// leaf's sorted keys via binary search. It skips all intermediate buffer scans,
// making it O(depth × log(fanout) + log(leafCap)) instead of the
// O(depth × bufferCap + log(leafCap)) cost of getFromNode.
func (t *BETree[K, V]) existsInLeaf(n *node[K, V], key K) bool {
	if n.leaf {
		_, found := n.leafSearch(key, t.cmp)
		return found
	}
	childIdx := n.findChildIndex(key, t.cmp)
	return t.existsInLeaf(n.children[childIdx], key)
}

// Get returns the value for key and true, or the zero value and false.
func (t *BETree[K, V]) Get(key K) (V, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.getFromNode(t.root, key)
}

// getFromNode recursively searches for key starting at the given node.
// At each internal node, it checks the buffer for pending messages.
// MsgPut/MsgDelete resolve immediately; MsgUpsert messages are collected
// and applied once a definitive base value is found.
func (t *BETree[K, V]) getFromNode(n *node[K, V], key K) (V, bool) {
	return t.getWithUpserts(n, key, nil)
}

func (t *BETree[K, V]) getWithUpserts(n *node[K, V], key K, fns []UpsertFn[V]) (V, bool) {
	if n.leaf {
		i, found := n.leafSearch(key, t.cmp)
		if found {
			return applyUpsertChain(n.values[i], true, fns)
		}
		if len(fns) > 0 {
			return applyUpsertChain(*new(V), false, fns)
		}
		var zero V
		return zero, false
	}

	// Check all messages for this key in the buffer (oldest→newest order).
	// Process newest→oldest: a definitive message resolves the chain.
	msgs := n.bufferMessagesForKey(key, t.cmp)
	for i := len(msgs) - 1; i >= 0; i-- {
		switch msgs[i].Kind {
		case MsgPut:
			return applyUpsertChain(msgs[i].Value, true, fns)
		case MsgDelete:
			if len(fns) > 0 {
				return applyUpsertChain(*new(V), false, fns)
			}
			var zero V
			return zero, false
		case MsgUpsert:
			fns = append(fns, msgs[i].Fn)
		}
	}

	childIdx := n.findChildIndex(key, t.cmp)
	return t.getWithUpserts(n.children[childIdx], key, fns)
}

// applyUpsertChain applies collected upsert functions to a base value.
// Functions are in newest-first order; they are applied oldest-first (reverse).
func applyUpsertChain[V any](base V, exists bool, fns []UpsertFn[V]) (V, bool) {
	if len(fns) == 0 {
		return base, exists
	}
	for i := len(fns) - 1; i >= 0; i-- {
		if exists {
			base = fns[i](&base, true)
		} else {
			base = fns[i](nil, false)
			exists = true
		}
	}
	return base, true
}

// Delete removes key from the tree. Returns true if the key existed.
func (t *BETree[K, V]) Delete(key K) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.deleteLocked(key)
}

// deleteLocked is the lock-free core of Delete. Caller must hold t.mu.
func (t *BETree[K, V]) deleteLocked(key K) bool {
	if t.root.leaf {
		if t.root.leafDelete(key, t.cmp) {
			t.size--
			return true
		}
		return false
	}

	if _, exists := t.getFromNode(t.root, key); !exists {
		return false
	}
	t.size--
	t.pendingDeletes++
	t.root.appendToBuffer(Message[K, V]{Kind: MsgDelete, Key: key})
	if len(t.root.buffer) > t.params.bufferCap {
		t.flushNode(t.root)
		if len(t.root.children) > t.params.fanout {
			t.splitRoot()
		}
	}
	return true
}

// Contains reports whether key exists in the tree.
func (t *BETree[K, V]) Contains(key K) bool {
	_, found := t.Get(key)
	return found
}

// Len returns the number of key-value pairs in the tree.
func (t *BETree[K, V]) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

// Clear removes all entries from the tree.
func (t *BETree[K, V]) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.root = newLeaf[K, V](t.params.leafCap)
	t.size = 0
	t.pendingDeletes = 0
}

// Close marks the tree as closed. Subsequent operations will still work
// but this method exists to satisfy the Tree interface. For BETree it is
// effectively a no-op; DiskBETree uses it to flush and close storage.
func (t *BETree[K, V]) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return nil
}

// DeleteRange removes all keys in [lo, hi). Returns the count removed.
// Keys in the range are collected via a read-only traversal, then buffered
// as individual MsgDelete entries at the root. This gives O(K) insertion
// time (K = keys in range) with the benefit of batched flush to leaves.
func (t *BETree[K, V]) DeleteRange(lo, hi K) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmp(lo, hi) >= 0 {
		return 0
	}

	// Direct leaf path: apply immediately when root is a leaf.
	if t.root.leaf {
		count := 0
		for i := len(t.root.keys) - 1; i >= 0; i-- {
			if t.cmp(t.root.keys[i], lo) >= 0 && t.cmp(t.root.keys[i], hi) < 0 {
				t.root.keys = slices.Delete(t.root.keys, i, i+1)
				t.root.values = slices.Delete(t.root.values, i, i+1)
				count++
			}
		}
		t.size -= count
		return count
	}

	// Collect live keys in range via the iterator infrastructure.
	var entries []mergeEntry[K, V]
	t.collectRange(t.root, lo, hi, true, false, 0, &entries)
	pairs := t.resolveEntries(entries)
	if len(pairs) == 0 {
		return 0
	}

	// Buffer individual deletes for each live key.
	count := len(pairs)
	t.size -= count
	t.pendingDeletes += count
	for _, p := range pairs {
		t.root.appendToBuffer(Message[K, V]{
			Kind: MsgDelete, Key: p.key,
		})
	}
	if len(t.root.buffer) > t.params.bufferCap {
		t.flushNode(t.root)
		if len(t.root.children) > t.params.fanout {
			t.splitRoot()
		}
	}
	return count
}

// Upsert applies fn atomically to the value at key. If the key exists, fn
// receives a pointer to the current value and exists=true. Otherwise fn
// receives nil and exists=false. The returned value is stored.
//
// The function is buffered as a MsgUpsert message and resolved lazily when
// the message reaches a leaf during flush, matching the B-epsilon-tree design.
func (t *BETree[K, V]) Upsert(key K, fn UpsertFn[V]) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.root.leaf {
		i, found := t.root.leafSearch(key, t.cmp)
		if found {
			t.root.values[i] = fn(&t.root.values[i], true)
		} else {
			newVal := fn(nil, false)
			t.root.leafInsert(key, newVal, t.cmp)
			t.size++
		}
		if len(t.root.keys) > t.params.leafCap {
			t.splitRoot()
		}
		return
	}

	t.root.appendToBuffer(Message[K, V]{Kind: MsgUpsert, Key: key, Fn: fn})
	if len(t.root.buffer) > t.params.bufferCap {
		t.flushNode(t.root)
		if len(t.root.children) > t.params.fanout {
			t.splitRoot()
		}
	}
}

// PutIfAbsent inserts value only if key does not exist. Returns true if inserted.
func (t *BETree[K, V]) PutIfAbsent(key K, value V) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.getFromNode(t.root, key); exists {
		return false
	}
	t.putLocked(key, value, true)
	return true
}

// Cursor returns a cursor for manual bidirectional iteration.
// The cursor operates on a snapshot taken at call time. It starts
// in an invalid state; call Next, Prev, or Seek to position it.
func (t *BETree[K, V]) Cursor() *Cursor[K, V] {
	t.mu.RLock()
	pairs := t.materializeAll()
	t.mu.RUnlock()

	return &Cursor[K, V]{
		pairs: pairs,
		cmp:   t.cmp,
	}
}
