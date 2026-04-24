package fractaltree

import (
	"iter"
	"slices"
)

// kvPair holds a materialized key-value pair from the tree.
type kvPair[K, V any] struct {
	key   K
	value V
}

// mergeEntry represents a key-value pair or deletion collected from one level
// of the tree during range iteration. Depth 0 = root (newest messages).
type mergeEntry[K, V any] struct {
	key     K
	value   V
	depth   int
	deleted bool
	fn      UpsertFn[V] // non-nil for MsgUpsert entries
}

// All returns an iterator over all key-value pairs in ascending order.
// The iterator operates on a snapshot taken at call time; concurrent
// mutations after All returns are not visible to the iterator.
func (t *BETree[K, V]) All() iter.Seq2[K, V] {
	t.mu.RLock()
	pairs := t.materializeAll()
	t.mu.RUnlock()

	return func(yield func(K, V) bool) {
		for _, p := range pairs {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

// Ascend returns an iterator over all key-value pairs in ascending order.
func (t *BETree[K, V]) Ascend() iter.Seq2[K, V] {
	return t.All()
}

// Descend returns an iterator over all key-value pairs in descending order.
func (t *BETree[K, V]) Descend() iter.Seq2[K, V] {
	t.mu.RLock()
	pairs := t.materializeAll()
	t.mu.RUnlock()

	return func(yield func(K, V) bool) {
		for i := len(pairs) - 1; i >= 0; i-- {
			if !yield(pairs[i].key, pairs[i].value) {
				return
			}
		}
	}
}

// Range returns an iterator over keys in [lo, hi) in ascending order.
func (t *BETree[K, V]) Range(lo, hi K) iter.Seq2[K, V] {
	t.mu.RLock()
	pairs := t.materializeRange(lo, hi)
	t.mu.RUnlock()

	return func(yield func(K, V) bool) {
		for _, p := range pairs {
			if !yield(p.key, p.value) {
				return
			}
		}
	}
}

// DescendRange returns an iterator over keys in (lo, hi] in descending order.
// Note: the first parameter is the high bound, the second is the low bound.
func (t *BETree[K, V]) DescendRange(hi, lo K) iter.Seq2[K, V] {
	t.mu.RLock()
	pairs := t.materializeDescendRange(hi, lo)
	t.mu.RUnlock()

	return func(yield func(K, V) bool) {
		for i := len(pairs) - 1; i >= 0; i-- {
			if !yield(pairs[i].key, pairs[i].value) {
				return
			}
		}
	}
}

// materializeAll returns all live key-value pairs in ascending order.
// Caller must hold at least t.mu.RLock.
func (t *BETree[K, V]) materializeAll() []kvPair[K, V] {
	var entries []mergeEntry[K, V]
	t.collectAll(t.root, 0, &entries)
	return t.resolveEntries(entries)
}

// materializeRange returns live key-value pairs in [lo, hi) in ascending order.
// Caller must hold at least t.mu.RLock.
func (t *BETree[K, V]) materializeRange(lo, hi K) []kvPair[K, V] {
	if t.cmp(lo, hi) >= 0 {
		return nil
	}
	var entries []mergeEntry[K, V]
	t.collectRange(t.root, lo, hi, true, false, 0, &entries)
	return t.resolveEntries(entries)
}

// materializeDescendRange returns live key-value pairs in (lo, hi] in
// ascending order (caller reverses for descending iteration).
// Caller must hold at least t.mu.RLock.
func (t *BETree[K, V]) materializeDescendRange(hi, lo K) []kvPair[K, V] {
	if t.cmp(lo, hi) >= 0 {
		return nil
	}
	var entries []mergeEntry[K, V]
	t.collectRange(t.root, lo, hi, false, true, 0, &entries)
	return t.resolveEntries(entries)
}

// collectAll traverses the tree collecting all entries for the merge iterator.
func (t *BETree[K, V]) collectAll(n *node[K, V], depth int, out *[]mergeEntry[K, V]) {
	if n.leaf {
		for i := range n.keys {
			*out = append(*out, mergeEntry[K, V]{key: n.keys[i], value: n.values[i], depth: depth})
		}
		return
	}
	t.collectBufferEntries(n.buffer, depth, out)
	for _, child := range n.children {
		t.collectAll(child, depth+1, out)
	}
}

// collectRange traverses the tree collecting entries whose keys satisfy the
// range defined by lo/hi with inclusivity flags. loInc=true means [lo, ...),
// loInc=false means (lo, ...). Similarly for hiInc.
func (t *BETree[K, V]) collectRange(
	n *node[K, V], lo, hi K, loInc, hiInc bool, depth int, out *[]mergeEntry[K, V],
) {
	if n.leaf {
		start, end := t.leafBounds(n, lo, hi, loInc, hiInc)
		for i := start; i < end; i++ {
			*out = append(*out, mergeEntry[K, V]{key: n.keys[i], value: n.values[i], depth: depth})
		}
		return
	}
	msgs := n.bufferSlice(lo, hi, loInc, hiInc, t.cmp)
	t.collectBufferEntries(msgs, depth, out)
	startChild := n.findChildIndex(lo, t.cmp)
	endChild := n.findChildIndex(hi, t.cmp)
	for i := startChild; i <= endChild && i < len(n.children); i++ {
		t.collectRange(n.children[i], lo, hi, loInc, hiInc, depth+1, out)
	}
}

// collectBufferEntries appends buffer messages to the entry list.
func (t *BETree[K, V]) collectBufferEntries(
	msgs []Message[K, V], depth int, out *[]mergeEntry[K, V],
) {
	for i := range msgs {
		switch msgs[i].Kind {
		case MsgPut:
			*out = append(*out, mergeEntry[K, V]{
				key: msgs[i].Key, value: msgs[i].Value, depth: depth,
			})
		case MsgDelete:
			*out = append(*out, mergeEntry[K, V]{
				key: msgs[i].Key, depth: depth, deleted: true,
			})
		case MsgUpsert:
			*out = append(*out, mergeEntry[K, V]{
				key: msgs[i].Key, depth: depth, fn: msgs[i].Fn,
			})
		}
	}
}

// leafBounds returns the start and end indices for a leaf range query.
func (t *BETree[K, V]) leafBounds(n *node[K, V], lo, hi K, loInc, hiInc bool) (int, int) {
	start, loFound := n.leafSearch(lo, t.cmp)
	if !loInc && loFound {
		start++
	}
	end, hiFound := n.leafSearch(hi, t.cmp)
	if hiInc && hiFound {
		end++
	}
	return start, end
}

// resolveEntries sorts entries by (key, depth), resolves same-key groups
// across depth levels (handling upsert chains), and returns live pairs in
// ascending key order.
func (t *BETree[K, V]) resolveEntries(entries []mergeEntry[K, V]) []kvPair[K, V] {
	if len(entries) == 0 {
		return nil
	}
	slices.SortStableFunc(entries, func(a, b mergeEntry[K, V]) int {
		if c := t.cmp(a.key, b.key); c != 0 {
			return c
		}
		return a.depth - b.depth
	})
	result := make([]kvPair[K, V], 0, len(entries))
	i := 0
	for i < len(entries) {
		j := i + 1
		for j < len(entries) && t.cmp(entries[j].key, entries[i].key) == 0 {
			j++
		}
		if v, exists := t.resolveKeyEntries(entries[i:j]); exists {
			result = append(result, kvPair[K, V]{key: entries[i].key, value: v})
		}
		i = j
	}
	return result
}

// resolveKeyEntries resolves a same-key group sorted by depth ascending.
// Processes from deepest (oldest) to shallowest (newest), applying upsert
// chains and handling Put/Delete overrides.
func (t *BETree[K, V]) resolveKeyEntries(entries []mergeEntry[K, V]) (V, bool) {
	var base V
	var exists bool
	// Process from deepest to shallowest. Within same depth, oldest→newest.
	k := len(entries) - 1
	for k >= 0 {
		depth := entries[k].depth
		groupEnd := k + 1
		for k >= 0 && entries[k].depth == depth {
			k--
		}
		groupStart := k + 1
		for g := groupStart; g < groupEnd; g++ {
			e := entries[g]
			switch {
			case e.deleted:
				base = *new(V)
				exists = false
			case e.fn != nil:
				if exists {
					base = e.fn(&base, true)
				} else {
					base = e.fn(nil, false)
					exists = true
				}
			default:
				base = e.value
				exists = true
			}
		}
	}
	return base, exists
}
