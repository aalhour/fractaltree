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

	// Grow the reusable bucket slice if the child count increased (due to splits).
	for len(n.flushBuckets) < numChildren {
		n.flushBuckets = append(n.flushBuckets, nil)
	}
	buckets := n.flushBuckets[:numChildren]
	for i := range buckets {
		buckets[i] = buckets[i][:0]
	}

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

	// Compact buffer in-place keeping only unflushed messages.
	k := 0
	for i, bucket := range buckets {
		if i != heaviest {
			k += copy(n.buffer[k:], bucket)
		}
	}
	clear(n.buffer[k:]) // zero trailing slots for GC
	n.buffer = n.buffer[:k]
	n.bufferSorted = false
	n.sortBuffer(t.cmp)

	child := n.children[heaviest]
	if child.leaf {
		t.applyToLeaf(child, buckets[heaviest])
		if len(child.keys) > t.params.leafCap {
			t.splitChild(n, heaviest)
		}
	} else {
		child.buffer = append(child.buffer, buckets[heaviest]...)
		child.bufferSorted = false
		child.sortBuffer(t.cmp)
		if len(child.buffer) > t.params.bufferCap {
			t.flushNode(child)
		}
		if len(child.children) > t.params.fanout {
			t.splitChild(n, heaviest)
		}
	}
}

// applyToLeaf applies a batch of messages to a leaf node.
//
// Small batches (≤3 messages) use per-message insert/delete. Larger batches
// use a sorted merge: stable-sort by key, deduplicate (last writer wins),
// then merge with the leaf in a single O(L+N) pass instead of N individual
// O(L) memmoves from slices.Insert.
//
// Size correction: putLocked/deleteLocked optimistically adjust t.size at
// insertion time using heuristics (existsInLeaf, pendingDeletes). The merge
// path corrects for any mismatch by comparing the leaf's actual key-count
// delta against the sum of those optimistic adjustments.
func (t *BETree[K, V]) applyToLeaf(leaf *node[K, V], msgs []Message[K, V]) {
	if len(msgs) == 0 {
		return
	}

	// Small batches: per-message path (avoids sort + merge overhead).
	if len(msgs) <= 3 {
		t.applyToLeafSmall(leaf, msgs)
		return
	}

	// --- Batch merge path ---

	// Phase 1: Tally optimistic adjustments from putLocked / deleteLocked.
	preCounted := 0
	numDeletes := 0
	for i := range msgs {
		if msgs[i].Kind == MsgPut && msgs[i].counted {
			preCounted++
		} else if msgs[i].Kind == MsgDelete {
			numDeletes++
		}
	}
	t.pendingDeletes -= numDeletes

	// Phase 2: Stable-sort by key; deduplicate and fold upsert chains.
	msgs = t.resolveMessages(msgs)

	// Phase 2b: Resolve remaining MsgUpsert messages against leaf values.
	for i := range msgs {
		if msgs[i].Kind != MsgUpsert {
			continue
		}
		idx, found := leaf.leafSearch(msgs[i].Key, t.cmp)
		if found {
			msgs[i].Value = msgs[i].Fn(&leaf.values[idx], true)
		} else {
			msgs[i].Value = msgs[i].Fn(nil, false)
		}
		msgs[i].Kind = MsgPut
		msgs[i].Fn = nil
	}

	// Phase 3: Merge resolved messages into the leaf.
	oldLen := len(leaf.keys)
	switch {
	case numDeletes == 0 && oldLen > 0 && t.cmp(msgs[0].Key, leaf.keys[oldLen-1]) > 0:
		// Append fast-path: all messages sort after the last leaf key.
		// Common for sequential inserts — skip binary search entirely.
		t.appendToLeaf(leaf, msgs)
	case numDeletes == 0:
		t.mergeLeafPuts(leaf, msgs)
	default:
		t.mergeLeafWithDeletes(leaf, msgs)
	}

	// Phase 4: Correct size — undo optimistic tallies, apply actual delta.
	// putLocked incremented size for each counted MsgPut (+preCounted).
	// deleteLocked decremented size for each MsgDelete (-numDeletes).
	// Actual change is (newLen - oldLen). Correction = actual - optimistic.
	t.size += (len(leaf.keys) - oldLen) - preCounted + numDeletes
}

// applyToLeafSmall applies a small batch of messages one by one.
func (t *BETree[K, V]) applyToLeafSmall(leaf *node[K, V], msgs []Message[K, V]) {
	for i := range msgs {
		switch msgs[i].Kind {
		case MsgPut:
			newKey := leaf.leafInsert(msgs[i].Key, msgs[i].Value, t.cmp)
			if newKey && !msgs[i].counted {
				t.size++
			} else if !newKey && msgs[i].counted {
				t.size--
			}
		case MsgDelete:
			leaf.leafDelete(msgs[i].Key, t.cmp)
			t.pendingDeletes--
		case MsgUpsert:
			idx, found := leaf.leafSearch(msgs[i].Key, t.cmp)
			if found {
				leaf.values[idx] = msgs[i].Fn(&leaf.values[idx], true)
			} else {
				leaf.leafInsert(msgs[i].Key, msgs[i].Fn(nil, false), t.cmp)
				t.size++
			}
		}
	}
}

// resolveMessages sorts messages by key and deduplicates. For same-key groups
// without upserts, the last message wins. For groups containing MsgUpsert,
// the chain is folded: Put+Upsert resolves to Put, Delete+Upsert resolves to
// Put, Upsert+Upsert composes into a single Upsert.
func (t *BETree[K, V]) resolveMessages(msgs []Message[K, V]) []Message[K, V] {
	sorted := true
	for i := 1; i < len(msgs); i++ {
		if t.cmp(msgs[i-1].Key, msgs[i].Key) >= 0 {
			sorted = false
			break
		}
	}
	if !sorted {
		slices.SortStableFunc(msgs, func(a, b Message[K, V]) int {
			return t.cmp(a.Key, b.Key)
		})
	}
	n := 0
	i := 0
	for i < len(msgs) {
		j := i + 1
		for j < len(msgs) && t.cmp(msgs[i].Key, msgs[j].Key) == 0 {
			j++
		}
		if j == i+1 {
			msgs[n] = msgs[i]
		} else {
			msgs[n] = t.resolveKeyGroup(msgs[i:j])
		}
		n++
		i = j
	}
	return msgs[:n]
}

// resolveKeyGroup folds a same-key message group (oldest→newest) into one
// message. Put/Delete are definitive; Upsert chains on the current state.
// MsgDeleteRange entries are kept separate (not folded with point messages).
func (t *BETree[K, V]) resolveKeyGroup(group []Message[K, V]) Message[K, V] {
	result := group[0]
	for i := 1; i < len(group); i++ {
		switch group[i].Kind {
		case MsgPut, MsgDelete:
			result = group[i]
		case MsgUpsert:
			switch result.Kind {
			case MsgPut:
				result.Value = group[i].Fn(&result.Value, true)
			case MsgDelete:
				result.Value = group[i].Fn(nil, false)
				result.Kind = MsgPut
				result.counted = false
			case MsgUpsert:
				prev := result.Fn
				curr := group[i].Fn
				result.Fn = func(existing *V, exists bool) V {
					base := prev(existing, exists)
					return curr(&base, true)
				}
			}
		}
	}
	return result
}

// appendToLeaf appends a sorted, deduplicated, puts-only batch whose first key
// is greater than every existing leaf key. This is the common path for
// sequential inserts: O(N) copies, no binary search, zero allocation when the
// leaf's backing array has sufficient capacity.
func (t *BETree[K, V]) appendToLeaf(leaf *node[K, V], msgs []Message[K, V]) {
	leaf.keys = slices.Grow(leaf.keys, len(msgs))
	leaf.values = slices.Grow(leaf.values, len(msgs))
	for i := range msgs {
		leaf.keys = append(leaf.keys, msgs[i].Key)
		leaf.values = append(leaf.values, msgs[i].Value)
	}
}

// mergeLeafPuts merges a sorted, deduplicated, puts-only batch into a leaf
// using binary-search + chunk-copy. For each put (processed largest-first),
// a binary search locates its position in O(log L), then copy() shifts the
// chunk of old keys between insertion points via memmove. Total work is
// O(N log L) comparisons + O(L) memmove — the same comparisons as N
// individual inserts but with 1 memmove pass instead of N.
func (t *BETree[K, V]) mergeLeafPuts(leaf *node[K, V], msgs []Message[K, V]) {
	numPuts := len(msgs)
	oldLen := len(leaf.keys)

	leaf.keys = slices.Grow(leaf.keys, numPuts)[:oldLen+numPuts]
	leaf.values = slices.Grow(leaf.values, numPuts)[:oldLen+numPuts]

	w := oldLen + numPuts - 1 // write position (tail of grown slice)
	r := oldLen - 1           // rightmost unprocessed old key
	overwrites := 0

	for p := numPuts - 1; p >= 0; p-- {
		pos, found := slices.BinarySearchFunc(leaf.keys[:r+1], msgs[p].Key, t.cmp)

		chunkStart := pos
		if found {
			chunkStart = pos + 1
			overwrites++
		}

		// Shift old keys [chunkStart..r] to their final position via memmove.
		if chunk := r - chunkStart + 1; chunk > 0 {
			copy(leaf.keys[w-chunk+1:w+1], leaf.keys[chunkStart:r+1])
			copy(leaf.values[w-chunk+1:w+1], leaf.values[chunkStart:r+1])
			w -= chunk
		}

		leaf.keys[w] = msgs[p].Key
		leaf.values[w] = msgs[p].Value
		w--
		r = pos - 1
	}

	// Close gap left by overwrites: shift merged portion left.
	if overwrites > 0 {
		copy(leaf.keys[r+1:], leaf.keys[w+1:oldLen+numPuts])
		copy(leaf.values[r+1:], leaf.values[w+1:oldLen+numPuts])
		finalLen := oldLen + numPuts - overwrites
		clear(leaf.keys[finalLen : oldLen+numPuts])
		clear(leaf.values[finalLen : oldLen+numPuts])
		leaf.keys = leaf.keys[:finalLen]
		leaf.values = leaf.values[:finalLen]
	}
}

// mergeLeafWithDeletes merges a sorted, deduplicated batch that contains at
// least one MsgDelete into a leaf. Two-phase approach avoids allocation:
//
// Phase 1 — In-place compaction: walk leaf and msgs together left-to-right.
// Delete matches are skipped, put overwrites update in place. New-insert puts
// (msg key not in leaf) are collected into msgs[:numNew] by reusing the
// already-consumed prefix (numNew <= mi, so writes never overtake reads).
//
// Phase 2 — Insert new keys via mergeLeafPuts (zero allocation when the leaf's
// backing array has sufficient capacity).
func (t *BETree[K, V]) mergeLeafWithDeletes(leaf *node[K, V], msgs []Message[K, V]) {
	li, mi, w, numNew := 0, 0, 0, 0
	for li < len(leaf.keys) && mi < len(msgs) {
		c := t.cmp(leaf.keys[li], msgs[mi].Key)
		switch {
		case c < 0: // leaf key not in batch — keep it
			leaf.keys[w] = leaf.keys[li]
			leaf.values[w] = leaf.values[li]
			w++
			li++
		case c > 0: // msg key not in leaf — collect new-insert puts
			if msgs[mi].Kind == MsgPut {
				msgs[numNew] = msgs[mi]
				numNew++
			}
			mi++
		default: // match — apply overwrite or delete
			if msgs[mi].Kind == MsgPut {
				leaf.keys[w] = msgs[mi].Key
				leaf.values[w] = msgs[mi].Value
				w++
			}
			li++
			mi++
		}
	}
	// Copy remaining leaf keys that sort after all messages.
	for li < len(leaf.keys) {
		leaf.keys[w] = leaf.keys[li]
		leaf.values[w] = leaf.values[li]
		w++
		li++
	}
	// Collect remaining new-insert puts (keys beyond all leaf keys).
	for mi < len(msgs) {
		if msgs[mi].Kind == MsgPut {
			msgs[numNew] = msgs[mi]
			numNew++
		}
		mi++
	}
	clear(leaf.keys[w:])
	clear(leaf.values[w:])
	leaf.keys = leaf.keys[:w]
	leaf.values = leaf.values[:w]

	// Phase 2: merge any new-insert puts into the compacted leaf.
	if numNew > 0 {
		t.mergeLeafPuts(leaf, msgs[:numNew])
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
	// Both halves preserve sorted order since the partition is by a single pivot.
	child.bufferSorted = true
	right.bufferSorted = true

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
