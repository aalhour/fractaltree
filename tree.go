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
	root   *node[K, V]
	cmp    func(K, K) int
	params treeParams
	size   int
	closed bool
	mu     sync.RWMutex
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
	t.putLocked(key, value)
}

// putLocked is the lock-free core of Put. Caller must hold t.mu.
func (t *BETree[K, V]) putLocked(key K, value V) {
	if t.root.leaf {
		if t.root.leafInsert(key, value, t.cmp) {
			t.size++
		}
		if len(t.root.keys) > t.params.leafCap {
			t.splitRoot()
		}
		return
	}

	// For internal root, check existence to track size accurately.
	if _, exists := t.getFromNode(t.root, key); !exists {
		t.size++
	}
	t.root.buffer = append(t.root.buffer, Message[K, V]{Kind: MsgPut, Key: key, Value: value})
	if len(t.root.buffer) > t.params.bufferCap {
		t.flushNode(t.root)
		if len(t.root.children) > t.params.fanout {
			t.splitRoot()
		}
	}
}

// Get returns the value for key and true, or the zero value and false.
func (t *BETree[K, V]) Get(key K) (V, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.getFromNode(t.root, key)
}

// getFromNode recursively searches for key starting at the given node.
// At each internal node, it checks the buffer for pending messages (newest
// first). If a definitive message is found, it resolves immediately.
// Otherwise it routes to the appropriate child.
func (t *BETree[K, V]) getFromNode(n *node[K, V], key K) (V, bool) {
	if n.leaf {
		i, found := n.leafSearch(key, t.cmp)
		if found {
			return n.values[i], true
		}
		var zero V
		return zero, false
	}

	// Scan buffer backwards (newest message first).
	for i := len(n.buffer) - 1; i >= 0; i-- {
		if t.cmp(n.buffer[i].Key, key) == 0 {
			switch n.buffer[i].Kind {
			case MsgPut:
				return n.buffer[i].Value, true
			case MsgDelete:
				var zero V
				return zero, false
			}
		}
	}

	childIdx := n.findChildIndex(key, t.cmp)
	return t.getFromNode(n.children[childIdx], key)
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
	t.root.buffer = append(t.root.buffer, Message[K, V]{Kind: MsgDelete, Key: key})
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
// Keys are collected eagerly from leaves and buffers, deduplicated, verified,
// and deleted individually. This is correct for in-memory trees; a disk-backed
// tree would buffer MsgDeleteRange messages instead.
func (t *BETree[K, V]) DeleteRange(lo, hi K) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cmp(lo, hi) >= 0 {
		return 0
	}

	// Collect all candidate keys that appear anywhere in the tree within [lo, hi).
	var candidates []K
	t.collectCandidateKeys(t.root, lo, hi, &candidates)

	// Sort and deduplicate so each key is attempted at most once.
	slices.SortFunc(candidates, t.cmp)
	candidates = slices.CompactFunc(candidates, func(a, b K) bool {
		return t.cmp(a, b) == 0
	})

	count := 0
	for _, key := range candidates {
		if t.deleteLocked(key) {
			count++
		}
	}
	return count
}

// collectCandidateKeys appends all keys from leaves and MsgPut buffer entries
// that fall within [lo, hi). The result may contain duplicates.
func (t *BETree[K, V]) collectCandidateKeys(n *node[K, V], lo, hi K, out *[]K) {
	if n.leaf {
		for _, k := range n.keys {
			if t.cmp(k, lo) >= 0 && t.cmp(k, hi) < 0 {
				*out = append(*out, k)
			}
		}
		return
	}
	for i := range n.buffer {
		if n.buffer[i].Kind == MsgPut &&
			t.cmp(n.buffer[i].Key, lo) >= 0 &&
			t.cmp(n.buffer[i].Key, hi) < 0 {
			*out = append(*out, n.buffer[i].Key)
		}
	}
	for _, child := range n.children {
		t.collectCandidateKeys(child, lo, hi, out)
	}
}

// Upsert applies fn atomically to the value at key. If the key exists, fn
// receives a pointer to the current value and exists=true. Otherwise fn
// receives nil and exists=false. The returned value is stored.
func (t *BETree[K, V]) Upsert(key K, fn UpsertFn[V]) {
	t.mu.Lock()
	defer t.mu.Unlock()

	v, exists := t.getFromNode(t.root, key)
	var newVal V
	if exists {
		newVal = fn(&v, true)
	} else {
		newVal = fn(nil, false)
	}
	t.putLocked(key, newVal)
}

// PutIfAbsent inserts value only if key does not exist. Returns true if inserted.
func (t *BETree[K, V]) PutIfAbsent(key K, value V) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.getFromNode(t.root, key); exists {
		return false
	}
	t.putLocked(key, value)
	return true
}

// Cursor returns a positioned cursor for manual bidirectional iteration.
func (t *BETree[K, V]) Cursor() *Cursor[K, V] {
	// TODO: implement in a later chunk.
	return &Cursor[K, V]{}
}
