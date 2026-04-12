package fractaltree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

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
	// Very small block size should clamp fanout and buffer to minimums.
	p := deriveParams(options{epsilon: 0.9, blockSize: 2})
	assert.GreaterOrEqual(t, p.fanout, minFanout)
	assert.GreaterOrEqual(t, p.bufferCap, minBufferCap)
}
