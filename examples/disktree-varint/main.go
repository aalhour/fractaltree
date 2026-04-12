// Disktree-varint demonstrates DiskBETree with a hand-rolled codec that
// encodes strings as a varint length prefix followed by raw UTF-8 bytes.
// This is more compact than GobCodec for short strings.
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/aalhour/fractaltree"
)

// VarintStringCodec encodes strings as a varint length prefix followed
// by raw UTF-8 bytes. Decoding reads the length, then extracts that
// many bytes.
type VarintStringCodec struct{}

func (VarintStringCodec) Encode(s string) ([]byte, error) {
	b := []byte(s)
	buf := make([]byte, binary.MaxVarintLen64+len(b))
	n := binary.PutUvarint(buf, uint64(len(b)))
	copy(buf[n:], b)
	return buf[:n+len(b)], nil
}

func (VarintStringCodec) Decode(data []byte) (string, error) {
	length, n := binary.Uvarint(data)
	if n <= 0 {
		return "", errors.New("invalid varint")
	}
	remaining := len(data) - n
	if remaining < 0 || length > uint64(remaining) {
		return "", fmt.Errorf("data too short: want %d bytes, have %d", length, remaining)
	}
	end := n + int(length) //nolint:gosec // length <= remaining, which fits in int
	return string(data[n:end]), nil
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
	f := newMemFlusher[string, string]()
	t, err := fractaltree.NewDisk[string, string](f,
		fractaltree.WithKeyCodec[string, string](VarintStringCodec{}),
		fractaltree.WithValueCodec[string, string](VarintStringCodec{}),
	)
	if err != nil {
		log.Fatal(err)
	}

	t.Put("hello", "world")
	t.Put("foo", "bar")
	t.Put("fractaltree", "fast")

	fmt.Println("All entries:")
	for k, v := range t.All() {
		fmt.Printf("  %s -> %s\n", k, v)
	}

	// Compare encoded sizes: VarintStringCodec vs GobCodec.
	vc := VarintStringCodec{}
	gc := fractaltree.GobCodec[string]{}

	words := []string{"go", "fractaltree", "hello world"}
	fmt.Println("\nEncoded sizes:")
	for _, w := range words {
		vb, _ := vc.Encode(w)
		gb, _ := gc.Encode(w)
		fmt.Printf("  %q: varint=%d bytes, gob=%d bytes\n", w, len(vb), len(gb))
	}

	if err := t.Close(); err != nil {
		log.Fatal(err)
	}
}
