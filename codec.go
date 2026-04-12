package fractaltree

import (
	"bytes"
	"encoding/gob"
)

// Codec defines how values of type T are serialized and deserialized.
// Implementations must be safe for concurrent use.
type Codec[T any] interface {
	Encode(T) ([]byte, error)
	Decode([]byte) (T, error)
}

// GobCodec is a Codec backed by encoding/gob. It works for any type
// that gob can handle (primitives, structs with exported fields, etc.).
type GobCodec[T any] struct{}

// Encode serializes v using encoding/gob.
func (GobCodec[T]) Encode(v T) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode deserializes data produced by Encode back into a value of type T.
func (GobCodec[T]) Decode(data []byte) (T, error) {
	var v T
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&v); err != nil {
		var zero T
		return zero, err
	}
	return v, nil
}
