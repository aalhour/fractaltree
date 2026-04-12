// Disktree-binary demonstrates DiskBETree with a codec backed by
// encoding/binary for fixed-size types (int32 keys, float64 values).
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"sync"

	"github.com/aalhour/fractaltree"
)

// BinaryCodec serializes fixed-size types using encoding/binary
// with little-endian byte order.
type BinaryCodec[T any] struct{}

func (BinaryCodec[T]) Encode(v T) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (BinaryCodec[T]) Decode(data []byte) (T, error) {
	var v T
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &v); err != nil {
		var zero T
		return zero, err
	}
	return v, nil
}

type memFlusher[K, V any] struct {
	mu    sync.Mutex
	nodes map[uint64][]byte
}

func newMemFlusher[K, V any]() *memFlusher[K, V] {
	return &memFlusher[K, V]{nodes: make(map[uint64][]byte)}
}

func (m *memFlusher[K, V]) WriteNode(id uint64, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[id] = append([]byte(nil), data...)
	return nil
}

func (m *memFlusher[K, V]) ReadNode(id uint64) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.nodes[id]
	if !ok {
		return nil, fmt.Errorf("node %d not found", id)
	}
	return data, nil
}

func (m *memFlusher[K, V]) Sync() error  { return nil }
func (m *memFlusher[K, V]) Close() error { return nil }

func main() {
	f := newMemFlusher[int32, float64]()
	t, err := fractaltree.NewDisk[int32, float64](f,
		fractaltree.WithKeyCodec[int32, float64](BinaryCodec[int32]{}),
		fractaltree.WithValueCodec[int32, float64](BinaryCodec[float64]{}),
	)
	if err != nil {
		log.Fatal(err)
	}

	t.Put(1, 3.14)
	t.Put(2, 2.72)
	t.Put(3, 1.41)

	fmt.Println("All entries:")
	for k, v := range t.All() {
		fmt.Printf("  %d -> %.2f\n", k, v)
	}

	// Show encoded sizes.
	kc := BinaryCodec[int32]{}
	vc := BinaryCodec[float64]{}
	kb, _ := kc.Encode(1)
	vb, _ := vc.Encode(3.14)
	fmt.Printf("\nEncoded int32 key:    %d bytes\n", len(kb))
	fmt.Printf("Encoded float64 val:  %d bytes\n", len(vb))

	if err := t.Close(); err != nil {
		log.Fatal(err)
	}
}
