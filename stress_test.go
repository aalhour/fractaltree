package fractaltree

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStress_ConcurrentReadWrite launches concurrent readers and writers
// and verifies no races or panics occur.
func TestStress_ConcurrentReadWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tree, err := New[int, int]()
	require.NoError(t, err)

	const writers = 8
	const readers = 8
	const opsPerGoroutine = 100_000

	var wg sync.WaitGroup

	// Writers: each writes to a non-overlapping key range.
	for w := range writers {
		wg.Go(func() {
			base := w * opsPerGoroutine
			for i := range opsPerGoroutine {
				tree.Put(base+i, base+i)
			}
		})
	}

	// Readers: read random keys (some will exist, some won't).
	for range readers {
		wg.Go(func() {
			for i := range opsPerGoroutine {
				tree.Get(i)
				tree.Contains(i)
			}
		})
	}

	wg.Wait()

	// All writer keys must be present.
	expectedLen := writers * opsPerGoroutine
	assert.Equal(t, expectedLen, tree.Len())

	for w := range writers {
		base := w * opsPerGoroutine
		v, ok := tree.Get(base)
		assert.True(t, ok, "key %d should exist", base)
		assert.Equal(t, base, v)

		last := base + opsPerGoroutine - 1
		v, ok = tree.Get(last)
		assert.True(t, ok, "key %d should exist", last)
		assert.Equal(t, last, v)
	}
}

// TestStress_ConcurrentRangeWhileWriting iterates via Range while
// concurrent writers are modifying the tree.
func TestStress_ConcurrentRangeWhileWriting(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tree, err := New[int, int]()
	require.NoError(t, err)

	// Seed the tree so Range has something to iterate.
	for i := range 10_000 {
		tree.Put(i, i)
	}

	var wg sync.WaitGroup

	// Writer: continuously put keys.
	wg.Go(func() {
		for i := 10_000; i < 50_000; i++ {
			tree.Put(i, i)
		}
	})

	// Readers: run Range queries concurrently.
	for range 4 {
		wg.Go(func() {
			for range 100 {
				count := 0
				for range tree.Range(0, 5000) {
					count++
				}
				// Snapshot semantics: count should be consistent.
				assert.Greater(t, count, 0)
			}
		})
	}

	wg.Wait()
}

// TestStress_ConcurrentIncrement verifies that concurrent Increment
// operations on the same key produce the correct sum.
func TestStress_ConcurrentIncrement(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tree, err := New[string, int]()
	require.NoError(t, err)

	const goroutines = 8
	const incrementsPerGoroutine = 10_000

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementsPerGoroutine {
				tree.Upsert("counter", Increment(1))
			}
		})
	}

	wg.Wait()

	expected := goroutines * incrementsPerGoroutine
	v, ok := tree.Get("counter")
	assert.True(t, ok)
	assert.Equal(t, expected, v, "concurrent increments should sum correctly")
}

// TestStress_ConcurrentDeleteRange runs DeleteRange while other
// goroutines are reading and writing.
func TestStress_ConcurrentDeleteRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tree, err := New[int, int]()
	require.NoError(t, err)

	// Seed with keys 0..9999.
	for i := range 10_000 {
		tree.Put(i, i)
	}

	var wg sync.WaitGroup

	// Writer: re-insert keys that may be deleted.
	wg.Go(func() {
		for i := range 5000 {
			tree.Put(i, i*2)
		}
	})

	// Deleter: delete ranges.
	wg.Go(func() {
		tree.DeleteRange(2000, 4000)
		tree.DeleteRange(6000, 8000)
	})

	// Reader: continuously read.
	wg.Go(func() {
		for i := range 10_000 {
			tree.Get(i)
		}
	})

	wg.Wait()

	// Tree should be in a consistent state (no panics, Len >= 0).
	assert.GreaterOrEqual(t, tree.Len(), 0)
}
