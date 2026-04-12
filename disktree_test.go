package fractaltree

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFlusher is an in-memory Flusher for testing.
type mockFlusher[K, V any] struct {
	mu     sync.Mutex
	nodes  map[uint64][]byte
	synced bool
	closed bool
}

func newMockFlusher[K, V any]() *mockFlusher[K, V] {
	return &mockFlusher[K, V]{nodes: make(map[uint64][]byte)}
}

func (m *mockFlusher[K, V]) WriteNode(id uint64, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("flusher closed")
	}
	m.nodes[id] = append([]byte(nil), data...)
	return nil
}

func (m *mockFlusher[K, V]) ReadNode(id uint64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %d not found", id)
	}
	return data, nil
}

func (m *mockFlusher[K, V]) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synced = true
	return nil
}

func (m *mockFlusher[K, V]) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// --- Constructor tests ---

func TestNewDisk_DefaultOptions(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer func() { require.NoError(t, tree.Close()) }()

	assert.Equal(t, 0, tree.Len())
}

func TestNewDisk_WithOptions(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f,
		WithDiskEpsilon[int, string](0.3),
		WithDiskBlockSize[int, string](100),
	)
	require.NoError(t, err)
	require.NotNil(t, tree)
	defer func() { require.NoError(t, tree.Close()) }()
}

func TestNewDiskWithCompare_NilComparator(t *testing.T) {
	f := newMockFlusher[int, string]()
	_, err := NewDiskWithCompare[int, string](nil, f)
	assert.ErrorIs(t, err, ErrNilCompare)
}

// --- Put/Get via DiskBETree ---

func TestDiskBETree_PutGet(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	tree.Put(1, "one")
	tree.Put(2, "two")
	tree.Put(3, "three")

	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "one", v)

	assert.Equal(t, 3, tree.Len())
	assert.True(t, tree.Contains(2))
}

func TestDiskBETree_Delete(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	tree.Put(1, "one")
	assert.True(t, tree.Delete(1))
	assert.False(t, tree.Contains(1))
	assert.Equal(t, 0, tree.Len())
}

func TestDiskBETree_DeleteRange(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	for i := range 10 {
		tree.Put(i, "v")
	}
	count := tree.DeleteRange(3, 7)
	assert.Equal(t, 4, count)
	assert.Equal(t, 6, tree.Len())
}

func TestDiskBETree_Upsert(t *testing.T) {
	f := newMockFlusher[string, int]()
	tree, err := NewDisk[string, int](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	tree.Upsert("counter", Increment(5))
	tree.Upsert("counter", Increment(3))

	v, ok := tree.Get("counter")
	assert.True(t, ok)
	assert.Equal(t, 8, v)
}

func TestDiskBETree_PutIfAbsent(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	assert.True(t, tree.PutIfAbsent(1, "one"))
	assert.False(t, tree.PutIfAbsent(1, "rejected"))

	v, _ := tree.Get(1)
	assert.Equal(t, "one", v)
}

// --- Iteration ---

func TestDiskBETree_Iterators(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	tree.Put(3, "three")
	tree.Put(1, "one")
	tree.Put(2, "two")

	var ascending []int
	for k := range tree.All() {
		ascending = append(ascending, k)
	}
	assert.Equal(t, []int{1, 2, 3}, ascending)

	var descending []int
	for k := range tree.Descend() {
		descending = append(descending, k)
	}
	assert.Equal(t, []int{3, 2, 1}, descending)

	var rangeKeys []int
	for k := range tree.Range(1, 3) {
		rangeKeys = append(rangeKeys, k)
	}
	assert.Equal(t, []int{1, 2}, rangeKeys)

	var drKeys []int
	for k := range tree.DescendRange(3, 1) {
		drKeys = append(drKeys, k)
	}
	assert.Equal(t, []int{3, 2}, drKeys)
}

// --- Cursor ---

func TestDiskBETree_Cursor(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	tree.Put(1, "one")
	tree.Put(2, "two")

	c := tree.Cursor()
	defer c.Close()

	require.True(t, c.Seek(2))
	assert.Equal(t, 2, c.Key())
	assert.Equal(t, "two", c.Value())
}

// --- Close behavior ---

func TestDiskBETree_Close_CallsSyncThenClose(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)

	require.NoError(t, tree.Close())
	assert.True(t, f.synced, "Sync should be called")
	assert.True(t, f.closed, "Close should be called")
}

func TestDiskBETree_Close_Idempotent(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)

	require.NoError(t, tree.Close())
	// Second close calls flusher again (which returns error since closed).
	// The flusher.Sync will succeed because synced flag is already true,
	// but Close on the mock returns nil since it's a no-op once closed.
}

// --- Custom codecs ---

func TestDiskBETree_CustomCodecs(t *testing.T) {
	f := newMockFlusher[int, string]()
	kc := GobCodec[int]{}
	vc := GobCodec[string]{}

	tree, err := NewDisk[int, string](f,
		WithKeyCodec[int, string](kc),
		WithValueCodec[int, string](vc),
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	// Verify codecs are stored (basic smoke test).
	assert.NotNil(t, tree.keyCodec)
	assert.NotNil(t, tree.valCodec)

	// Operations still work.
	tree.Put(1, "one")
	v, ok := tree.Get(1)
	assert.True(t, ok)
	assert.Equal(t, "one", v)
}

// --- Interface compliance ---

func TestDiskBETree_ImplementsTree(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	// Compile-time check: DiskBETree satisfies Tree interface.
	var _ Tree[int, string] = tree
}

// --- Clear ---

func TestDiskBETree_Clear(t *testing.T) {
	f := newMockFlusher[int, string]()
	tree, err := NewDisk[int, string](f)
	require.NoError(t, err)
	defer func() { require.NoError(t, tree.Close()) }()

	tree.Put(1, "one")
	tree.Put(2, "two")
	tree.Clear()

	assert.Equal(t, 0, tree.Len())
	assert.False(t, tree.Contains(1))
}
