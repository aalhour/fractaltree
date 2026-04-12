package fractaltree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Upsert ---

func TestUpsert_AbsentKey(t *testing.T) {
	tree := newTestTree(t)

	tree.Upsert(1, func(existing *string, exists bool) string {
		assert.Nil(t, existing)
		assert.False(t, exists)
		return "created"
	})

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "created", v)
	assert.Equal(t, 1, tree.Len())
}

func TestUpsert_PresentKey(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "old")

	tree.Upsert(1, func(existing *string, exists bool) string {
		require.NotNil(t, existing)
		assert.True(t, exists)
		assert.Equal(t, "old", *existing)
		return "updated"
	})

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "updated", v)
	assert.Equal(t, 1, tree.Len(), "upsert on existing key should not change Len")
}

func TestUpsert_MultipleOnSameKey(t *testing.T) {
	tree, err := New[int, int]()
	require.NoError(t, err)

	for i := range 5 {
		tree.Upsert(1, func(existing *int, exists bool) int {
			if !exists {
				return i
			}
			return *existing + i
		})
	}

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, 0+1+2+3+4, v) // 0 + 1 + 2 + 3 + 4 = 10
	assert.Equal(t, 1, tree.Len())
}

func TestUpsert_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)

	// Force flushes with many keys.
	for i := range 20 {
		tree.Put(i, "v")
	}

	tree.Upsert(10, func(_ *string, exists bool) string {
		require.True(t, exists)
		return "upserted"
	})

	v, ok := tree.Get(10)
	assert.True(t, ok)
	assert.Equal(t, "upserted", v)
	assert.Equal(t, 20, tree.Len())
}

// --- PutIfAbsent ---

func TestPutIfAbsent_Absent(t *testing.T) {
	tree := newTestTree(t)

	inserted := tree.PutIfAbsent(1, "one")
	assert.True(t, inserted)

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "one", v)
	assert.Equal(t, 1, tree.Len())
}

func TestPutIfAbsent_Present(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "original")

	inserted := tree.PutIfAbsent(1, "rejected")
	assert.False(t, inserted)

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "original", v, "existing value should not change")
	assert.Equal(t, 1, tree.Len())
}

func TestPutIfAbsent_AfterDelete(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "first")
	tree.Delete(1)

	inserted := tree.PutIfAbsent(1, "second")
	assert.True(t, inserted)

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "second", v)
}

func TestPutIfAbsent_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)

	for i := range 20 {
		tree.Put(i, "v")
	}

	assert.False(t, tree.PutIfAbsent(10, "rejected"))
	assert.True(t, tree.PutIfAbsent(100, "accepted"))

	v, ok := tree.Get(100)
	assert.True(t, ok)
	assert.Equal(t, "accepted", v)
}

// --- Increment helper ---

func TestIncrement_AbsentKey(t *testing.T) {
	tree, err := New[string, int]()
	require.NoError(t, err)

	tree.Upsert("counter", Increment(5))

	v, ok := tree.Get("counter")
	assert.True(t, ok)
	assert.Equal(t, 5, v)
}

func TestIncrement_PresentKey(t *testing.T) {
	tree, err := New[string, int]()
	require.NoError(t, err)

	tree.Put("counter", 10)
	tree.Upsert("counter", Increment(3))

	v, ok := tree.Get("counter")
	assert.True(t, ok)
	assert.Equal(t, 13, v)
}

func TestIncrement_MultipleIncrements(t *testing.T) {
	tree, err := New[string, int]()
	require.NoError(t, err)

	for range 100 {
		tree.Upsert("counter", Increment(1))
	}

	v, ok := tree.Get("counter")
	assert.True(t, ok)
	assert.Equal(t, 100, v)
	assert.Equal(t, 1, tree.Len())
}

func TestIncrement_Float64(t *testing.T) {
	tree, err := New[string, float64]()
	require.NoError(t, err)

	tree.Put("balance", 100.50)
	tree.Upsert("balance", Increment(25.25))

	v, ok := tree.Get("balance")
	assert.True(t, ok)
	assert.InDelta(t, 125.75, v, 0.001)
}

// --- CompareAndSwap helper ---

func TestCompareAndSwap_Match(t *testing.T) {
	tree, err := New[string, string]()
	require.NoError(t, err)

	tree.Put("status", "pending")
	tree.Upsert("status", CompareAndSwap("pending", "active"))

	v, ok := tree.Get("status")
	assert.True(t, ok)
	assert.Equal(t, "active", v)
}

func TestCompareAndSwap_Mismatch(t *testing.T) {
	tree, err := New[string, string]()
	require.NoError(t, err)

	tree.Put("status", "active")
	tree.Upsert("status", CompareAndSwap("pending", "closed"))

	v, ok := tree.Get("status")
	assert.True(t, ok)
	assert.Equal(t, "active", v, "value should not change on CAS mismatch")
}

func TestCompareAndSwap_AbsentKey(t *testing.T) {
	tree, err := New[string, string]()
	require.NoError(t, err)

	tree.Upsert("status", CompareAndSwap("pending", "active"))

	v, ok := tree.Get("status")
	assert.True(t, ok)
	assert.Equal(t, "", v, "CAS on absent key should store zero value")
	assert.Equal(t, 1, tree.Len())
}
