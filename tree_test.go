package fractaltree

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// --- Constructor tests ---

func TestNew_DefaultOptions(t *testing.T) {
	tree, err := New[int, string]()
	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Equal(t, DefaultEpsilon, tree.params.epsilon)
	assert.Equal(t, DefaultBlockSize, tree.params.blockSize)
	assert.Equal(t, 64, tree.params.fanout)
	assert.Equal(t, 64, tree.params.bufferCap)
	assert.Equal(t, DefaultBlockSize, tree.params.leafCap)
	assert.True(t, tree.root.leaf)
	assert.Equal(t, 0, tree.size)
	assert.False(t, tree.closed)
}

func TestNew_WithCustomOptions(t *testing.T) {
	tree, err := New[string, int](
		WithEpsilon(0.3),
		WithBlockSize(1000),
	)
	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.InDelta(t, 0.3, tree.params.epsilon, 0.001)
	assert.Equal(t, 1000, tree.params.blockSize)
}

func TestNew_InvalidEpsilon(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		_, err := New[int, int](WithEpsilon(0))
		assert.ErrorIs(t, err, ErrInvalidEpsilon)
	})

	t.Run("negative", func(t *testing.T) {
		_, err := New[int, int](WithEpsilon(-0.5))
		assert.ErrorIs(t, err, ErrInvalidEpsilon)
	})

	t.Run("greater than one", func(t *testing.T) {
		_, err := New[int, int](WithEpsilon(1.5))
		assert.ErrorIs(t, err, ErrInvalidEpsilon)
	})

	t.Run("exactly one is valid", func(t *testing.T) {
		tree, err := New[int, int](WithEpsilon(1.0))
		require.NoError(t, err)
		assert.NotNil(t, tree)
	})
}

func TestNew_InvalidBlockSize(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		_, err := New[int, int](WithBlockSize(0))
		assert.ErrorIs(t, err, ErrInvalidBlockSize)
	})

	t.Run("one", func(t *testing.T) {
		_, err := New[int, int](WithBlockSize(1))
		assert.ErrorIs(t, err, ErrInvalidBlockSize)
	})

	t.Run("two is valid", func(t *testing.T) {
		tree, err := New[int, int](WithBlockSize(2))
		require.NoError(t, err)
		assert.NotNil(t, tree)
	})
}

func TestNewWithCompare_NilComparator(t *testing.T) {
	_, err := NewWithCompare[int, int](nil)
	assert.ErrorIs(t, err, ErrNilCompare)
}

func TestNewWithCompare_CustomComparator(t *testing.T) {
	type CompositeKey struct {
		Namespace string
		ID        int64
	}

	cmpFn := func(a, b CompositeKey) int {
		if a.Namespace < b.Namespace {
			return -1
		}
		if a.Namespace > b.Namespace {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	}

	tree, err := NewWithCompare[CompositeKey, string](cmpFn)
	require.NoError(t, err)
	assert.NotNil(t, tree)
	assert.True(t, tree.root.leaf)
}

func TestDeriveParams_Defaults(t *testing.T) {
	p := deriveParams(defaultOptions())
	assert.Equal(t, 64, p.fanout)
	assert.Equal(t, 64, p.bufferCap)
	assert.Equal(t, DefaultBlockSize, p.leafCap)
}

func TestDeriveParams_SmallBlockSize(t *testing.T) {
	p := deriveParams(options{epsilon: 0.5, blockSize: 4})
	assert.Equal(t, 2, p.fanout)    // sqrt(4) = 2
	assert.Equal(t, 2, p.bufferCap) // sqrt(4) = 2
	assert.Equal(t, 4, p.leafCap)
}

func TestDeriveParams_MinimumClamp(t *testing.T) {
	p := deriveParams(options{epsilon: 0.9, blockSize: 2})
	assert.GreaterOrEqual(t, p.fanout, minFanout)
	assert.GreaterOrEqual(t, p.bufferCap, minBufferCap)
}

// --- Empty tree behavior ---

func TestEmptyTree(t *testing.T) {
	tree := newTestTree(t)

	t.Run("Len is zero", func(t *testing.T) {
		assert.Equal(t, 0, tree.Len())
	})

	t.Run("Get returns false", func(t *testing.T) {
		v, ok := tree.Get(42)
		assert.False(t, ok)
		assert.Equal(t, "", v)
	})

	t.Run("Contains returns false", func(t *testing.T) {
		assert.False(t, tree.Contains(42))
	})

	t.Run("Delete returns false", func(t *testing.T) {
		assert.False(t, tree.Delete(42))
	})
}

// --- Put and Get ---

func TestPut_SingleKey(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "one", v)
	assert.Equal(t, 1, tree.Len())
}

func TestPut_MultipleKeys(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(3, "three")
	tree.Put(1, "one")
	tree.Put(2, "two")

	for _, tc := range []struct {
		key int
		val string
	}{
		{1, "one"},
		{2, "two"},
		{3, "three"},
	} {
		v, ok := tree.Get(tc.key)
		assert.True(t, ok, "key %d should exist", tc.key)
		assert.Equal(t, tc.val, v, "key %d", tc.key)
	}
	assert.Equal(t, 3, tree.Len())
}

func TestPut_OverwriteExistingKey(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "first")
	tree.Put(1, "second")

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "second", v)
	assert.Equal(t, 1, tree.Len(), "overwrite should not change Len")
}

func TestPut_GetMissing(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")

	v, ok := tree.Get(999)
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

// --- Delete ---

func TestDelete_ExistingKey(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")
	tree.Put(2, "two")

	assert.True(t, tree.Delete(1))
	assert.Equal(t, 1, tree.Len())
	assert.False(t, tree.Contains(1))
	assert.True(t, tree.Contains(2))
}

func TestDelete_MissingKey(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")

	assert.False(t, tree.Delete(999))
	assert.Equal(t, 1, tree.Len())
}

func TestDelete_ThenReinsert(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "first")
	tree.Delete(1)
	tree.Put(1, "second")

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "second", v)
	assert.Equal(t, 1, tree.Len())
}

func TestDelete_AllKeys(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}
	for i := range 10 {
		assert.True(t, tree.Delete(i))
	}
	assert.Equal(t, 0, tree.Len())
}

// --- Contains ---

func TestContains(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(5, "five")

	assert.True(t, tree.Contains(5))
	assert.False(t, tree.Contains(6))
}

// --- Clear ---

func TestClear(t *testing.T) {
	tree := newTestTree(t)
	for i := range 20 {
		tree.Put(i, "v")
	}

	tree.Clear()
	assert.Equal(t, 0, tree.Len())
	assert.False(t, tree.Contains(0))
	assert.True(t, tree.root.leaf)
}

func TestClear_ThenReuse(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "before")
	tree.Clear()
	tree.Put(2, "after")

	assert.False(t, tree.Contains(1))
	assert.True(t, tree.Contains(2))
	assert.Equal(t, 1, tree.Len())
}

// --- Close ---

func TestClose_Idempotent(t *testing.T) {
	tree := newTestTree(t)
	require.NoError(t, tree.Close())
	require.NoError(t, tree.Close())
}

// --- Edge cases ---

func TestEdge_ZeroValueKey(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(0, "zero")

	v, ok := tree.Get(0)
	assert.True(t, ok)
	assert.Equal(t, "zero", v)
}

func TestEdge_ZeroValueValue(t *testing.T) {
	tree, err := New[int, int]()
	require.NoError(t, err)
	tree.Put(1, 0)

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, 0, v)
}

func TestEdge_IntBoundaries(t *testing.T) {
	tree, err := New[int, string]()
	require.NoError(t, err)

	tree.Put(math.MinInt, "min")
	tree.Put(math.MaxInt, "max")
	tree.Put(0, "zero")

	v, ok := tree.Get(math.MinInt)
	assert.True(t, ok)
	assert.Equal(t, "min", v)

	v, ok = tree.Get(math.MaxInt)
	assert.True(t, ok)
	assert.Equal(t, "max", v)

	v, ok = tree.Get(0)
	assert.True(t, ok)
	assert.Equal(t, "zero", v)

	assert.Equal(t, 3, tree.Len())
}

func TestEdge_StringKeys(t *testing.T) {
	tree, err := New[string, int]()
	require.NoError(t, err)

	tree.Put("", 0)
	tree.Put("hello", 1)
	tree.Put("world", 2)
	tree.Put("\x00", 3)

	v, ok := tree.Get("")
	assert.True(t, ok)
	assert.Equal(t, 0, v)

	assert.Equal(t, 4, tree.Len())
}

// --- DeleteRange ---

func TestDeleteRange_EmptyTree(t *testing.T) {
	tree := newTestTree(t)
	assert.Equal(t, 0, tree.DeleteRange(0, 100))
	assert.Equal(t, 0, tree.Len())
}

func TestDeleteRange_RemovesCorrectSubset(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i, "v")
	}

	// Delete keys [3, 7) => 3, 4, 5, 6
	count := tree.DeleteRange(3, 7)
	assert.Equal(t, 4, count)
	assert.Equal(t, 6, tree.Len())

	for i := range 10 {
		if i >= 3 && i < 7 {
			assert.False(t, tree.Contains(i), "key %d should be deleted", i)
		} else {
			assert.True(t, tree.Contains(i), "key %d should remain", i)
		}
	}
}

func TestDeleteRange_LoGreaterOrEqualHi(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "v")
	tree.Put(2, "v")

	assert.Equal(t, 0, tree.DeleteRange(5, 5), "lo == hi should be no-op")
	assert.Equal(t, 0, tree.DeleteRange(5, 3), "lo > hi should be no-op")
	assert.Equal(t, 2, tree.Len())
}

func TestDeleteRange_AllKeys(t *testing.T) {
	tree := newTestTree(t)
	for i := range 20 {
		tree.Put(i, "v")
	}

	count := tree.DeleteRange(0, 20)
	assert.Equal(t, 20, count)
	assert.Equal(t, 0, tree.Len())
}

func TestDeleteRange_NoKeysInRange(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	count := tree.DeleteRange(10, 20)
	assert.Equal(t, 0, count)
	assert.Equal(t, 5, tree.Len())
}

func TestDeleteRange_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)

	for i := range 50 {
		tree.Put(i, "v")
	}

	count := tree.DeleteRange(10, 30)
	assert.Equal(t, 20, count)
	assert.Equal(t, 30, tree.Len())

	for i := range 50 {
		if i >= 10 && i < 30 {
			assert.False(t, tree.Contains(i), "key %d should be deleted", i)
		} else {
			assert.True(t, tree.Contains(i), "key %d should remain", i)
		}
	}
}

func TestDeleteRange_ThenReinsert(t *testing.T) {
	tree := newSmallTree(t)
	for i := range 20 {
		tree.Put(i, "old")
	}

	tree.DeleteRange(5, 15)
	for i := 5; i < 15; i++ {
		tree.Put(i, "new")
	}

	for i := range 20 {
		v, ok := tree.Get(i)
		assert.True(t, ok, "key %d should exist", i)
		if i >= 5 && i < 15 {
			assert.Equal(t, "new", v)
		} else {
			assert.Equal(t, "old", v)
		}
	}
	assert.Equal(t, 20, tree.Len())
}

// --- Flush and split behavior ---
// Use a small block size to force flushes and splits at low key counts.

func TestFlush_TriggeredByBufferOverflow(t *testing.T) {
	// blockSize=4, epsilon=0.5 => fanout=2, bufferCap=2, leafCap=4
	tree := newSmallTree(t)

	// Insert enough keys to overflow the leaf, trigger a split (creating
	// an internal root), then overflow the buffer and trigger a flush.
	for i := range 20 {
		tree.Put(i, "v")
	}

	// All keys must be retrievable after flushes and splits.
	for i := range 20 {
		v, ok := tree.Get(i)
		assert.True(t, ok, "key %d should exist", i)
		assert.Equal(t, "v", v, "key %d", i)
	}
	assert.Equal(t, 20, tree.Len())
}

func TestFlush_SequentialKeys(t *testing.T) {
	tree := newSmallTree(t)
	n := 100
	for i := range n {
		tree.Put(i, "v")
	}
	for i := range n {
		assert.True(t, tree.Contains(i), "key %d missing", i)
	}
	assert.Equal(t, n, tree.Len())
}

func TestFlush_ReverseKeys(t *testing.T) {
	tree := newSmallTree(t)
	n := 100
	for i := n - 1; i >= 0; i-- {
		tree.Put(i, "v")
	}
	for i := range n {
		assert.True(t, tree.Contains(i), "key %d missing", i)
	}
	assert.Equal(t, n, tree.Len())
}

func TestFlush_RandomKeys(t *testing.T) {
	tree := newSmallTree(t)
	keys := rand.Perm(200)
	for _, k := range keys {
		tree.Put(k, "v")
	}
	for _, k := range keys {
		assert.True(t, tree.Contains(k), "key %d missing", k)
	}
	assert.Equal(t, 200, tree.Len())
}

func TestFlush_OverwriteAfterFlush(t *testing.T) {
	tree := newSmallTree(t)

	// Insert keys to trigger flushes.
	for i := range 20 {
		tree.Put(i, "old")
	}

	// Overwrite all of them.
	for i := range 20 {
		tree.Put(i, "new")
	}

	for i := range 20 {
		v, ok := tree.Get(i)
		assert.True(t, ok)
		assert.Equal(t, "new", v, "key %d should be overwritten", i)
	}
	assert.Equal(t, 20, tree.Len())
}

func TestFlush_DeleteAfterFlush(t *testing.T) {
	tree := newSmallTree(t)

	for i := range 30 {
		tree.Put(i, "v")
	}

	// Delete even keys.
	for i := 0; i < 30; i += 2 {
		assert.True(t, tree.Delete(i), "key %d should exist for deletion", i)
	}

	// Verify only odd keys remain.
	for i := range 30 {
		if i%2 == 0 {
			assert.False(t, tree.Contains(i), "key %d should be deleted", i)
		} else {
			assert.True(t, tree.Contains(i), "key %d should exist", i)
		}
	}
	assert.Equal(t, 15, tree.Len())
}

func TestFlush_InterleavedPutDelete(t *testing.T) {
	tree := newSmallTree(t)

	// Insert 50 keys, delete 25, insert 25 more.
	for i := range 50 {
		tree.Put(i, "v")
	}
	for i := range 25 {
		tree.Delete(i)
	}
	for i := 50; i < 75; i++ {
		tree.Put(i, "v")
	}

	assert.Equal(t, 50, tree.Len())
	for i := range 75 {
		if i < 25 {
			assert.False(t, tree.Contains(i))
		} else {
			assert.True(t, tree.Contains(i))
		}
	}
}

func TestFlush_PutDeletePutSameKey(t *testing.T) {
	tree := newSmallTree(t)

	// Force a multi-level tree.
	for i := range 20 {
		tree.Put(i, "initial")
	}

	tree.Put(5, "first")
	tree.Delete(5)
	tree.Put(5, "second")

	v, ok := tree.Get(5)
	assert.True(t, ok)
	assert.Equal(t, "second", v)
	assert.Equal(t, 20, tree.Len())
}

func TestFlush_LargeDataset(t *testing.T) {
	tree := newSmallTree(t)
	n := 1000
	for i := range n {
		tree.Put(i, "v")
	}
	for i := range n {
		assert.True(t, tree.Contains(i), "key %d missing after %d inserts", i, n)
	}
	assert.Equal(t, n, tree.Len())
}

func TestFlush_ClearAfterSplits(t *testing.T) {
	tree := newSmallTree(t)
	for i := range 50 {
		tree.Put(i, "v")
	}
	tree.Clear()
	assert.Equal(t, 0, tree.Len())
	assert.True(t, tree.root.leaf)

	// Reuse after clear.
	tree.Put(1, "new")
	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "new", v)
}

// --- Helpers ---

// newTestTree creates a BETree[int, string] with default options.
func newTestTree(t *testing.T) *BETree[int, string] {
	t.Helper()
	tree, err := New[int, string]()
	require.NoError(t, err)
	return tree
}

// newSmallTree creates a BETree with blockSize=4 and epsilon=0.5 to trigger
// flushes and splits at very low key counts (fanout=2, bufferCap=2, leafCap=4).
func newSmallTree(t *testing.T) *BETree[int, string] {
	t.Helper()
	tree, err := New[int, string](WithBlockSize(4), WithEpsilon(0.5))
	require.NoError(t, err)
	return tree
}

// --- P3 (unsorted append + lazy sort) adversarial tests ---

func TestP3_ReverseOrderInserts(t *testing.T) {
	tree := newSmallTree(t)
	// Insert in reverse order — buffer will be unsorted between flushes.
	for i := 100; i > 0; i-- {
		tree.Put(i, "v")
	}
	assert.Equal(t, 100, tree.Len())
	for i := 1; i <= 100; i++ {
		v, ok := tree.Get(i)
		assert.True(t, ok, "key %d missing", i)
		assert.Equal(t, "v", v)
	}
}

func TestP3_InterleavedReadsAndWrites(t *testing.T) {
	tree := newSmallTree(t)
	// Interleave writes and reads to test unsorted buffer reads.
	for i := range 50 {
		tree.Put(i*3, "a")   // sparse keys
		tree.Put(i*3+2, "c") // out of order within buffer
		tree.Put(i*3+1, "b")
		// Read before flush forces linear scan on unsorted buffer.
		v, ok := tree.Get(i * 3)
		assert.True(t, ok, "key %d missing after put", i*3)
		assert.Equal(t, "a", v)
	}
	assert.Equal(t, 150, tree.Len())
}

func TestP3_DuplicateKeyOverwrite(t *testing.T) {
	tree := newSmallTree(t)
	// Many overwrites of the same key — all land in buffer before flush.
	for i := range 20 {
		tree.Put(1, "v"+string(rune('a'+i)))
	}
	assert.Equal(t, 1, tree.Len())
	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "v"+string(rune('a'+19)), v) // last write wins
}

func TestP3_DeleteBeforeFlush(t *testing.T) {
	tree := newSmallTree(t)
	tree.Put(1, "a")
	tree.Put(2, "b")
	tree.Put(3, "c")
	// Delete before buffer flushes.
	assert.True(t, tree.Delete(2))
	assert.Equal(t, 2, tree.Len())
	_, ok := tree.Get(2)
	assert.False(t, ok)
	// Remaining keys intact.
	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "a", v)
	v, ok = tree.Get(3)
	assert.True(t, ok)
	assert.Equal(t, "c", v)
}

func TestP3_UpsertChainInUnsortedBuffer(t *testing.T) {
	tree, err := New[int, int](WithBlockSize(4), WithEpsilon(0.5))
	require.NoError(t, err)
	tree.Put(1, 10)
	// Multiple upserts accumulate in unsorted buffer.
	for range 5 {
		tree.Upsert(1, func(existing *int, exists bool) int {
			if exists {
				return *existing + 1
			}
			return 0
		})
	}
	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, 15, v) // 10 + 5 increments
}

func TestP3_RangeOnUnsortedBuffer(t *testing.T) {
	tree := newSmallTree(t)
	// Insert in reverse order so buffer is unsorted.
	for i := 10; i > 0; i-- {
		tree.Put(i, "v")
	}
	// Range query must still return correct sorted results.
	var keys []int
	for k := range tree.Range(3, 8) {
		keys = append(keys, k)
	}
	assert.Equal(t, []int{3, 4, 5, 6, 7}, keys)
}

func TestP3_LargeBufferCorrectness(t *testing.T) {
	// Use default tree (bufferCap=4096) — much larger buffer than before.
	tree := newTestTree(t)
	n := 50_000
	perm := rand.Perm(n)
	for _, i := range perm {
		tree.Put(i, "v")
	}
	assert.Equal(t, n, tree.Len())
	// Verify all keys present.
	for i := range n {
		v, ok := tree.Get(i)
		assert.True(t, ok, "key %d missing", i)
		assert.Equal(t, "v", v)
	}
	// Verify ascending order via iterator.
	prev := -1
	count := 0
	for k := range tree.All() {
		assert.Greater(t, k, prev)
		prev = k
		count++
	}
	assert.Equal(t, n, count)
}
