package fractaltree

import "slices"

// node represents either an internal or leaf node in the B-epsilon-tree.
//
// Internal nodes hold pivot keys, child pointers, and a message buffer.
// Leaf nodes hold sorted key-value pairs directly.
type node[K any, V any] struct {
	leaf bool

	// Internal node fields.
	pivots       []K
	children     []*node[K, V]
	buffer       []Message[K, V]
	flushBuckets [][]Message[K, V] // reused across flushes to avoid per-flush allocations

	// Leaf node fields.
	keys   []K
	values []V
}

// newLeaf creates an empty leaf node with preallocated capacity.
func newLeaf[K any, V any](capacity int) *node[K, V] {
	return &node[K, V]{
		leaf:   true,
		keys:   make([]K, 0, capacity),
		values: make([]V, 0, capacity),
	}
}

// findChildIndex returns the index of the child that key should route to.
// For pivots [p0, p1, ..., pN-1] and children [c0, c1, ..., cN]:
//
//	key < p0       -> child 0
//	p0 <= key < p1 -> child 1
//	key >= pN-1    -> child N
func (n *node[K, V]) findChildIndex(key K, compare func(K, K) int) int {
	lo, hi := 0, len(n.pivots)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if compare(n.pivots[mid], key) <= 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// leafSearch returns the index where key is or would be inserted,
// and whether the key was found.
func (n *node[K, V]) leafSearch(key K, compare func(K, K) int) (int, bool) {
	lo, hi := 0, len(n.keys)
	for lo < hi {
		mid := lo + (hi-lo)/2
		c := compare(n.keys[mid], key)
		if c == 0 {
			return mid, true
		}
		if c < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, false
}

// leafInsert inserts or overwrites key/value in a leaf node.
// Returns true if the key was newly inserted (not an overwrite).
func (n *node[K, V]) leafInsert(key K, value V, compare func(K, K) int) bool {
	i, found := n.leafSearch(key, compare)
	if found {
		n.values[i] = value
		return false
	}
	n.keys = slices.Insert(n.keys, i, key)
	n.values = slices.Insert(n.values, i, value)
	return true
}

// leafDelete removes key from a leaf node.
// Returns true if the key existed and was removed.
func (n *node[K, V]) leafDelete(key K, compare func(K, K) int) bool {
	i, found := n.leafSearch(key, compare)
	if !found {
		return false
	}
	n.keys = slices.Delete(n.keys, i, i+1)
	n.values = slices.Delete(n.values, i, i+1)
	return true
}
