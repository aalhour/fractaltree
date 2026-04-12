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
	var candidates []K
	t.collectAllCandidateKeys(t.root, &candidates)

	slices.SortFunc(candidates, t.cmp)
	candidates = slices.CompactFunc(candidates, func(a, b K) bool {
		return t.cmp(a, b) == 0
	})

	pairs := make([]kvPair[K, V], 0, len(candidates))
	for _, key := range candidates {
		if v, ok := t.getFromNode(t.root, key); ok {
			pairs = append(pairs, kvPair[K, V]{key: key, value: v})
		}
	}
	return pairs
}

// materializeRange returns live key-value pairs in [lo, hi) in ascending order.
// Caller must hold at least t.mu.RLock.
func (t *BETree[K, V]) materializeRange(lo, hi K) []kvPair[K, V] {
	if t.cmp(lo, hi) >= 0 {
		return nil
	}

	var candidates []K
	t.collectCandidateKeys(t.root, lo, hi, &candidates)

	slices.SortFunc(candidates, t.cmp)
	candidates = slices.CompactFunc(candidates, func(a, b K) bool {
		return t.cmp(a, b) == 0
	})

	pairs := make([]kvPair[K, V], 0, len(candidates))
	for _, key := range candidates {
		if v, ok := t.getFromNode(t.root, key); ok {
			pairs = append(pairs, kvPair[K, V]{key: key, value: v})
		}
	}
	return pairs
}

// materializeDescendRange returns live key-value pairs in (lo, hi] in
// ascending order (caller reverses for descending iteration).
// Caller must hold at least t.mu.RLock.
func (t *BETree[K, V]) materializeDescendRange(hi, lo K) []kvPair[K, V] {
	if t.cmp(lo, hi) >= 0 {
		return nil
	}

	// Collect candidates in the wider range [lo, hi] then filter to (lo, hi].
	// We reuse collectCandidateKeys with a slightly wider range
	// since it collects [lo, hi) and we need (lo, hi].
	var candidates []K
	t.collectAllCandidateKeys(t.root, &candidates)

	slices.SortFunc(candidates, t.cmp)
	candidates = slices.CompactFunc(candidates, func(a, b K) bool {
		return t.cmp(a, b) == 0
	})

	pairs := make([]kvPair[K, V], 0)
	for _, key := range candidates {
		// Filter to (lo, hi]: key > lo && key <= hi
		if t.cmp(key, lo) <= 0 || t.cmp(key, hi) > 0 {
			continue
		}
		if v, ok := t.getFromNode(t.root, key); ok {
			pairs = append(pairs, kvPair[K, V]{key: key, value: v})
		}
	}
	return pairs
}

// collectAllCandidateKeys appends all keys from leaves and MsgPut buffer
// entries. The result may contain duplicates.
func (t *BETree[K, V]) collectAllCandidateKeys(n *node[K, V], out *[]K) {
	if n.leaf {
		*out = append(*out, n.keys...)
		return
	}
	for i := range n.buffer {
		if n.buffer[i].Kind == MsgPut {
			*out = append(*out, n.buffer[i].Key)
		}
	}
	for _, child := range n.children {
		t.collectAllCandidateKeys(child, out)
	}
}
