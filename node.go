package fractaltree

// node represents either an internal or leaf node in the B-epsilon-tree.
// Internal nodes hold pivots, child pointers, and a message buffer.
// Leaf nodes hold sorted key-value pairs.
type node[K any, V any] struct {
	leaf bool

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
