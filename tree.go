package fractaltree

import (
	"cmp"
	"iter"
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

	// Close releases resources held by the tree. For BETree this is a no-op
	// beyond marking the tree as closed. For DiskBETree it flushes pending
	// buffers and closes the underlying storage.
	Close() error
}

// BETree is an in-memory B-epsilon-tree. It is safe for concurrent use.
type BETree[K any, V any] struct {
	root   *node[K, V]
	cmp    func(K, K) int
	params treeParams
	size   int
	closed bool
}

// New creates a BETree for keys that satisfy [cmp.Ordered].
// The comparator is derived automatically via [cmp.Compare].
//
// Example:
//
//	t, err := fractaltree.New[int, string]()
func New[K cmp.Ordered, V any](opts ...Option) (*BETree[K, V], error) {
	return NewWithCompare[K, V](cmp.Compare, opts...)
}

// NewWithCompare creates a BETree with a user-supplied comparator.
// The comparator must return a negative value when a < b, zero when a == b,
// and a positive value when a > b.
//
// Use this for composite keys or custom orderings that do not satisfy
// [cmp.Ordered].
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
