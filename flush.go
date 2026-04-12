package fractaltree

import "slices"

// flushNode flushes the buffer of an internal node using the greedy algorithm:
// partition messages by child, then flush to the child with the most messages.
// This is required for the amortized O(log_B N / B^(1-epsilon)) write bound.
func (t *BETree[K, V]) flushNode(n *node[K, V]) {
	if len(n.buffer) == 0 || n.leaf {
		return
	}

	numChildren := len(n.children)
	buckets := make([][]Message[K, V], numChildren)
	for _, msg := range n.buffer {
		idx := n.findChildIndex(msg.Key, t.cmp)
		buckets[idx] = append(buckets[idx], msg)
	}

	// Greedy: pick the child that received the most messages.
	heaviest := 0
	for i := 1; i < numChildren; i++ {
		if len(buckets[i]) > len(buckets[heaviest]) {
			heaviest = i
		}
	}

	// Rebuild buffer keeping only unflushed messages.
	remaining := make([]Message[K, V], 0, len(n.buffer)-len(buckets[heaviest]))
	for i, bucket := range buckets {
		if i != heaviest {
			remaining = append(remaining, bucket...)
		}
	}
	n.buffer = remaining

	child := n.children[heaviest]
	if child.leaf {
		t.applyToLeaf(child, buckets[heaviest])
		if len(child.keys) > t.params.leafCap {
			t.splitChild(n, heaviest)
		}
	} else {
		child.buffer = append(child.buffer, buckets[heaviest]...)
		if len(child.buffer) > t.params.bufferCap {
			t.flushNode(child)
		}
		if len(child.children) > t.params.fanout {
			t.splitChild(n, heaviest)
		}
	}
}

// applyToLeaf applies a batch of messages to a leaf node.
// Messages are applied in order so that later messages override earlier ones.
func (t *BETree[K, V]) applyToLeaf(leaf *node[K, V], msgs []Message[K, V]) {
	for i := range msgs {
		switch msgs[i].Kind {
		case MsgPut:
			leaf.leafInsert(msgs[i].Key, msgs[i].Value, t.cmp)
		case MsgDelete:
			leaf.leafDelete(msgs[i].Key, t.cmp)
		}
	}
}

// splitChild splits parent.children[childIdx] into two halves and inserts
// a new pivot into the parent. Dispatches to leaf or internal split.
func (t *BETree[K, V]) splitChild(parent *node[K, V], childIdx int) {
	child := parent.children[childIdx]
	if child.leaf {
		t.splitLeafChild(parent, childIdx)
	} else {
		t.splitInternalChild(parent, childIdx)
	}
}

// splitLeafChild splits a leaf child at the midpoint.
// The middle key is promoted as a pivot in the parent.
func (t *BETree[K, V]) splitLeafChild(parent *node[K, V], childIdx int) {
	child := parent.children[childIdx]
	mid := len(child.keys) / 2

	right := &node[K, V]{
		leaf:   true,
		keys:   make([]K, len(child.keys[mid:])),
		values: make([]V, len(child.values[mid:])),
	}
	copy(right.keys, child.keys[mid:])
	copy(right.values, child.values[mid:])

	pivot := right.keys[0]

	child.keys = child.keys[:mid]
	child.values = child.values[:mid]

	parent.pivots = slices.Insert(parent.pivots, childIdx, pivot)
	parent.children = slices.Insert(parent.children, childIdx+1, right)
}

// splitInternalChild splits an internal child at the midpoint of its pivots.
// The middle pivot is promoted to the parent. Buffer messages are partitioned
// between the two halves based on the promoted pivot.
func (t *BETree[K, V]) splitInternalChild(parent *node[K, V], childIdx int) {
	child := parent.children[childIdx]
	mid := len(child.pivots) / 2
	pivot := child.pivots[mid]

	right := &node[K, V]{
		leaf:     false,
		pivots:   make([]K, len(child.pivots[mid+1:])),
		children: make([]*node[K, V], len(child.children[mid+1:])),
		buffer:   make([]Message[K, V], 0, t.params.bufferCap),
	}
	copy(right.pivots, child.pivots[mid+1:])
	copy(right.children, child.children[mid+1:])

	// Partition buffer between left and right halves.
	var leftBuf []Message[K, V]
	for _, msg := range child.buffer {
		if t.cmp(msg.Key, pivot) < 0 {
			leftBuf = append(leftBuf, msg)
		} else {
			right.buffer = append(right.buffer, msg)
		}
	}

	child.pivots = child.pivots[:mid]
	child.children = child.children[:mid+1]
	child.buffer = leftBuf

	parent.pivots = slices.Insert(parent.pivots, childIdx, pivot)
	parent.children = slices.Insert(parent.children, childIdx+1, right)
}

// splitRoot creates a new root with the old root as its only child,
// then splits that child. The tree grows one level taller.
func (t *BETree[K, V]) splitRoot() {
	oldRoot := t.root
	newRoot := &node[K, V]{
		leaf:     false,
		pivots:   make([]K, 0, t.params.fanout-1),
		children: make([]*node[K, V], 0, t.params.fanout),
		buffer:   make([]Message[K, V], 0, t.params.bufferCap),
	}
	newRoot.children = append(newRoot.children, oldRoot)
	t.root = newRoot
	t.splitChild(newRoot, 0)
}
