package fractaltree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- All / Ascend ---

func TestAll_EmptyTree(t *testing.T) {
	tree := newTestTree(t)

	count := 0
	for range tree.All() {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestAll_AscendingOrder(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(3, "three")
	tree.Put(1, "one")
	tree.Put(2, "two")

	var keys []int
	var vals []string
	for k, v := range tree.All() {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	assert.Equal(t, []int{1, 2, 3}, keys)
	assert.Equal(t, []string{"one", "two", "three"}, vals)
}

func TestAll_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)
	n := 50
	for i := range n {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.All() {
		keys = append(keys, k)
	}

	assert.Equal(t, n, len(keys))
	for i := range n {
		assert.Equal(t, i, keys[i], "position %d", i)
	}
}

func TestAll_AfterDeletes(t *testing.T) {
	tree := newSmallTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}
	tree.Delete(3)
	tree.Delete(7)

	var keys []int
	for k := range tree.All() {
		keys = append(keys, k)
	}

	assert.Equal(t, []int{0, 1, 2, 4, 5, 6, 8, 9}, keys)
}

func TestAll_BreakEarly(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.All() {
		keys = append(keys, k)
		if k == 2 {
			break
		}
	}

	assert.Equal(t, []int{0, 1, 2}, keys)
}

func TestAscend_SameAsAll(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(3, "three")
	tree.Put(1, "one")
	tree.Put(2, "two")

	var allKeys, ascendKeys []int
	for k := range tree.All() {
		allKeys = append(allKeys, k)
	}
	for k := range tree.Ascend() {
		ascendKeys = append(ascendKeys, k)
	}

	assert.Equal(t, allKeys, ascendKeys)
}

// --- Descend ---

func TestDescend_EmptyTree(t *testing.T) {
	tree := newTestTree(t)

	count := 0
	for range tree.Descend() {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestDescend_DescendingOrder(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")
	tree.Put(2, "two")
	tree.Put(3, "three")

	var keys []int
	var vals []string
	for k, v := range tree.Descend() {
		keys = append(keys, k)
		vals = append(vals, v)
	}

	assert.Equal(t, []int{3, 2, 1}, keys)
	assert.Equal(t, []string{"three", "two", "one"}, vals)
}

func TestDescend_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)
	n := 30
	for i := range n {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Descend() {
		keys = append(keys, k)
	}

	assert.Equal(t, n, len(keys))
	for i := range n {
		assert.Equal(t, n-1-i, keys[i], "position %d", i)
	}
}

func TestDescend_BreakEarly(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Descend() {
		keys = append(keys, k)
		if k == 7 {
			break
		}
	}

	assert.Equal(t, []int{9, 8, 7}, keys)
}

// --- Range ---

func TestRange_EmptyTree(t *testing.T) {
	tree := newTestTree(t)

	count := 0
	for range tree.Range(0, 100) {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestRange_ExactSubset(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Range(3, 7) {
		keys = append(keys, k)
	}

	assert.Equal(t, []int{3, 4, 5, 6}, keys)
}

func TestRange_LoBelowMin(t *testing.T) {
	tree := newTestTree(t)
	for i := 5; i < 10; i++ {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Range(0, 8) {
		keys = append(keys, k)
	}

	assert.Equal(t, []int{5, 6, 7}, keys)
}

func TestRange_HiAboveMax(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Range(2, 100) {
		keys = append(keys, k)
	}

	assert.Equal(t, []int{2, 3, 4}, keys)
}

func TestRange_LoEqualsHi(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(5, "v")

	count := 0
	for range tree.Range(5, 5) {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestRange_LoGreaterThanHi(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(5, "v")

	count := 0
	for range tree.Range(10, 5) {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestRange_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)
	for i := range 50 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Range(10, 20) {
		keys = append(keys, k)
	}

	require.Equal(t, 10, len(keys))
	for i, k := range keys {
		assert.Equal(t, 10+i, k)
	}
}

func TestRange_BreakEarly(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.Range(0, 10) {
		keys = append(keys, k)
		if k == 3 {
			break
		}
	}

	assert.Equal(t, []int{0, 1, 2, 3}, keys)
}

// --- DescendRange ---

func TestDescendRange_EmptyTree(t *testing.T) {
	tree := newTestTree(t)

	count := 0
	for range tree.DescendRange(100, 0) {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestDescendRange_ExactSubset(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	// DescendRange(hi=7, lo=3) yields keys in (3, 7] = {4, 5, 6, 7} descending
	var keys []int
	for k := range tree.DescendRange(7, 3) {
		keys = append(keys, k)
	}

	assert.Equal(t, []int{7, 6, 5, 4}, keys)
}

func TestDescendRange_LoEqualsHi(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(5, "v")

	count := 0
	for range tree.DescendRange(5, 5) {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestDescendRange_HiBelowLo(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(5, "v")

	count := 0
	for range tree.DescendRange(3, 10) {
		count++
	}
	assert.Equal(t, 0, count)
}

func TestDescendRange_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)
	for i := range 50 {
		tree.Put(i, "v")
	}

	// (10, 20] descending = {20, 19, ..., 11}
	var keys []int
	for k := range tree.DescendRange(20, 10) {
		keys = append(keys, k)
	}

	require.Equal(t, 10, len(keys))
	for i, k := range keys {
		assert.Equal(t, 20-i, k)
	}
}

func TestDescendRange_BreakEarly(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	var keys []int
	for k := range tree.DescendRange(9, 0) {
		keys = append(keys, k)
		if k == 7 {
			break
		}
	}

	assert.Equal(t, []int{9, 8, 7}, keys)
}

// --- Snapshot semantics ---

func TestAll_SnapshotSemantics(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	// Create iterator, then modify tree. Iterator should see original state.
	it := tree.All()

	tree.Put(100, "new")
	tree.Delete(0)

	var keys []int
	for k := range it {
		keys = append(keys, k)
	}

	// Snapshot was taken before modifications.
	assert.Equal(t, []int{0, 1, 2, 3, 4}, keys)
}
