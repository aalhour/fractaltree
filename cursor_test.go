package fractaltree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Empty tree ---

func TestCursor_EmptyTree(t *testing.T) {
	tree := newTestTree(t)
	c := tree.Cursor()
	defer c.Close()

	assert.False(t, c.Valid())
	assert.False(t, c.Next())
	assert.False(t, c.Prev())
	assert.False(t, c.Seek(42))
}

// --- Next ---

func TestCursor_Next_TraversesAscending(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(3, "three")
	tree.Put(1, "one")
	tree.Put(2, "two")

	c := tree.Cursor()
	defer c.Close()

	var keys []int
	for c.Next() {
		keys = append(keys, c.Key())
	}
	assert.Equal(t, []int{1, 2, 3}, keys)
	assert.False(t, c.Valid())
}

func TestCursor_Next_PastEnd(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")

	c := tree.Cursor()
	defer c.Close()

	assert.True(t, c.Next())
	assert.Equal(t, 1, c.Key())
	assert.False(t, c.Next())
	assert.False(t, c.Valid())
}

// --- Prev ---

func TestCursor_Prev_TraversesDescending(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")
	tree.Put(2, "two")
	tree.Put(3, "three")

	c := tree.Cursor()
	defer c.Close()

	var keys []int
	for c.Prev() {
		keys = append(keys, c.Key())
	}
	assert.Equal(t, []int{3, 2, 1}, keys)
	assert.False(t, c.Valid())
}

func TestCursor_Prev_PastStart(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")

	c := tree.Cursor()
	defer c.Close()

	assert.True(t, c.Prev())
	assert.Equal(t, 1, c.Key())
	assert.False(t, c.Prev())
	assert.False(t, c.Valid())
}

// --- Seek ---

func TestCursor_Seek_ExistingKey(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i*2, "v") // 0, 2, 4, 6, 8, 10, 12, 14, 16, 18
	}

	c := tree.Cursor()
	defer c.Close()

	assert.True(t, c.Seek(6))
	assert.True(t, c.Valid())
	assert.Equal(t, 6, c.Key())
	assert.Equal(t, "v", c.Value())
}

func TestCursor_Seek_NonExistentPositionsAtNextGreater(t *testing.T) {
	tree := newTestTree(t)
	for i := range 10 {
		tree.Put(i*2, "v") // 0, 2, 4, 6, 8, ...
	}

	c := tree.Cursor()
	defer c.Close()

	assert.True(t, c.Seek(5))
	assert.True(t, c.Valid())
	assert.Equal(t, 6, c.Key(), "should position at first key >= 5")
}

func TestCursor_Seek_BeyondMax(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")
	tree.Put(2, "two")

	c := tree.Cursor()
	defer c.Close()

	assert.False(t, c.Seek(100))
	assert.False(t, c.Valid())
}

func TestCursor_Seek_AtMin(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(5, "five")
	tree.Put(10, "ten")

	c := tree.Cursor()
	defer c.Close()

	assert.True(t, c.Seek(0))
	assert.Equal(t, 5, c.Key(), "should position at first key >= 0")
}

// --- Seek + Next/Prev ---

func TestCursor_Seek_ThenNext(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	c := tree.Cursor()
	defer c.Close()

	require.True(t, c.Seek(2))
	assert.Equal(t, 2, c.Key())

	assert.True(t, c.Next())
	assert.Equal(t, 3, c.Key())

	assert.True(t, c.Next())
	assert.Equal(t, 4, c.Key())

	assert.False(t, c.Next())
}

func TestCursor_Seek_ThenPrev(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	c := tree.Cursor()
	defer c.Close()

	require.True(t, c.Seek(3))
	assert.Equal(t, 3, c.Key())

	assert.True(t, c.Prev())
	assert.Equal(t, 2, c.Key())

	assert.True(t, c.Prev())
	assert.Equal(t, 1, c.Key())
}

// --- Close ---

func TestCursor_Close_Idempotent(t *testing.T) {
	tree := newTestTree(t)
	tree.Put(1, "one")

	c := tree.Cursor()
	c.Close()
	c.Close() // should not panic

	assert.False(t, c.Valid())
}

// --- After flush ---

func TestCursor_AfterFlush(t *testing.T) {
	tree := newSmallTree(t)
	for i := range 50 {
		tree.Put(i, "v")
	}

	c := tree.Cursor()
	defer c.Close()

	var keys []int
	for c.Next() {
		keys = append(keys, c.Key())
	}

	assert.Equal(t, 50, len(keys))
	for i, k := range keys {
		assert.Equal(t, i, k)
	}
}

// --- Snapshot semantics ---

func TestCursor_SnapshotSemantics(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	c := tree.Cursor()
	defer c.Close()

	// Modify tree after cursor creation.
	tree.Put(100, "new")
	tree.Delete(0)

	var keys []int
	for c.Next() {
		keys = append(keys, c.Key())
	}

	assert.Equal(t, []int{0, 1, 2, 3, 4}, keys, "cursor should see original snapshot")
}

// --- Bidirectional ---

func TestCursor_NextThenPrev(t *testing.T) {
	tree := newTestTree(t)
	for i := range 5 {
		tree.Put(i, "v")
	}

	c := tree.Cursor()
	defer c.Close()

	require.True(t, c.Next()) // -> 0
	require.True(t, c.Next()) // -> 1
	require.True(t, c.Next()) // -> 2
	assert.Equal(t, 2, c.Key())

	require.True(t, c.Prev()) // -> 1
	assert.Equal(t, 1, c.Key())

	require.True(t, c.Next()) // -> 2
	assert.Equal(t, 2, c.Key())
}
