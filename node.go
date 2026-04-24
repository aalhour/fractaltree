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
	bufferSorted bool              // true when buffer is sorted by key
	flushBuckets [][]Message[K, V] // reused across flushes to avoid per-flush allocations

	// Leaf node fields.
	keys   []K
	values []V
}

// sortBuffer sorts the buffer by key using stable sort, then marks it sorted.
// Caller must hold the tree write lock.
func (n *node[K, V]) sortBuffer(cmp func(K, K) int) {
	if n.bufferSorted || len(n.buffer) <= 1 {
		return
	}
	slices.SortStableFunc(n.buffer, func(a, b Message[K, V]) int {
		return cmp(a.Key, b.Key)
	})
	n.bufferSorted = true
}

// appendToBuffer inserts a message at its sorted position, maintaining the
// sorted invariant. Caller must hold the tree write lock.
func (n *node[K, V]) appendToBuffer(msg Message[K, V], cmp func(K, K) int) {
	if !n.bufferSorted || len(n.buffer) == 0 {
		n.buffer = append(n.buffer, msg)
		n.bufferSorted = len(n.buffer) <= 1
		if !n.bufferSorted {
			n.sortBuffer(cmp)
		}
		return
	}
	// Insert at sorted position. Same-key messages go after existing ones
	// (newest last) by using <= 0 comparison.
	pos, _ := slices.BinarySearchFunc(n.buffer, msg.Key, func(m Message[K, V], k K) int {
		return cmp(m.Key, k)
	})
	// Find the end of the key group to insert after all existing same-key msgs.
	for pos < len(n.buffer) && cmp(n.buffer[pos].Key, msg.Key) == 0 {
		pos++
	}
	n.buffer = slices.Insert(n.buffer, pos, msg)
}

// bufferMessagesForKey returns all messages for key in the sorted buffer.
// Within the returned slice, messages are in insertion order (oldest first).
func (n *node[K, V]) bufferMessagesForKey(key K, cmp func(K, K) int) []Message[K, V] {
	i, found := slices.BinarySearchFunc(n.buffer, key, func(m Message[K, V], k K) int {
		return cmp(m.Key, k)
	})
	if !found {
		return nil
	}
	end := i + 1
	for end < len(n.buffer) && cmp(n.buffer[end].Key, key) == 0 {
		end++
	}
	return n.buffer[i:end]
}

// bufferSlice returns the sub-slice of the sorted buffer containing messages
// whose keys fall in the range defined by lo/hi with inclusivity flags.
// The buffer must already be sorted.
func (n *node[K, V]) bufferSlice(lo, hi K, loInc, hiInc bool, cmp func(K, K) int) []Message[K, V] {
	if len(n.buffer) == 0 {
		return nil
	}
	start, _ := slices.BinarySearchFunc(n.buffer, lo, func(m Message[K, V], k K) int {
		return cmp(m.Key, k)
	})
	if !loInc {
		for start < len(n.buffer) && cmp(n.buffer[start].Key, lo) == 0 {
			start++
		}
	}
	end, _ := slices.BinarySearchFunc(n.buffer, hi, func(m Message[K, V], k K) int {
		return cmp(m.Key, k)
	})
	if hiInc {
		for end < len(n.buffer) && cmp(n.buffer[end].Key, hi) == 0 {
			end++
		}
	}
	if start >= end {
		return nil
	}
	return n.buffer[start:end]
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
