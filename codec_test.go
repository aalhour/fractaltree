package fractaltree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGobCodec_Int(t *testing.T) {
	c := GobCodec[int]{}

	data, err := c.Encode(42)
	require.NoError(t, err)

	v, err := c.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, 42, v)
}

func TestGobCodec_String(t *testing.T) {
	c := GobCodec[string]{}

	data, err := c.Encode("hello")
	require.NoError(t, err)

	v, err := c.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, "hello", v)
}

func TestGobCodec_Struct(t *testing.T) {
	type Record struct {
		Name  string
		Score int
	}

	c := GobCodec[Record]{}

	original := Record{Name: "alice", Score: 100}
	data, err := c.Encode(original)
	require.NoError(t, err)

	decoded, err := c.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestGobCodec_ZeroValue(t *testing.T) {
	c := GobCodec[int]{}

	data, err := c.Encode(0)
	require.NoError(t, err)

	v, err := c.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, 0, v)
}

func TestGobCodec_CorruptedInput(t *testing.T) {
	c := GobCodec[int]{}

	_, err := c.Decode([]byte{0xFF, 0xFE, 0xFD})
	assert.Error(t, err)
}

func TestGobCodec_EmptyInput(t *testing.T) {
	c := GobCodec[string]{}

	_, err := c.Decode([]byte{})
	assert.Error(t, err)
}
