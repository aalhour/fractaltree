package fractaltree

import (
	"cmp"
	"iter"
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

// --- Stubs for operations implemented in later chunks ---

// DeleteRange removes all keys in [lo, hi). Returns the count removed.
func (t *BETree[K, V]) DeleteRange(_, _ K) int {
	// TODO: implement in a later chunk.
	return 0
}

// Upsert applies fn atomically to the value at key.
func (t *BETree[K, V]) Upsert(_ K, _ UpsertFn[V]) {
	// TODO: implement in a later chunk.
}

// PutIfAbsent inserts value only if key does not exist. Returns true if inserted.
func (t *BETree[K, V]) PutIfAbsent(_ K, _ V) bool {
	// TODO: implement in a later chunk.
	return false
}

// All returns an iterator over all key-value pairs in ascending order.
func (t *BETree[K, V]) All() iter.Seq2[K, V] {
	// TODO: implement in a later chunk.
	return func(_ func(K, V) bool) {}
}

// Ascend returns an iterator over all key-value pairs in ascending order.
func (t *BETree[K, V]) Ascend() iter.Seq2[K, V] {
	return t.All()
}

// Descend returns an iterator over all key-value pairs in descending order.
func (t *BETree[K, V]) Descend() iter.Seq2[K, V] {
	// TODO: implement in a later chunk.
	return func(_ func(K, V) bool) {}
}

// Range returns an iterator over keys in [lo, hi) in ascending order.
func (t *BETree[K, V]) Range(_, _ K) iter.Seq2[K, V] {
	// TODO: implement in a later chunk.
	return func(_ func(K, V) bool) {}
}

// DescendRange returns an iterator over keys in (lo, hi] in descending order.
func (t *BETree[K, V]) DescendRange(_, _ K) iter.Seq2[K, V] {
	// TODO: implement in a later chunk.
	return func(_ func(K, V) bool) {}
}

// Cursor returns a positioned cursor for manual bidirectional iteration.
func (t *BETree[K, V]) Cursor() *Cursor[K, V] {
	// TODO: implement in a later chunk.
	return &Cursor[K, V]{}
}
