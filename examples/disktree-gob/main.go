// Disktree-gob demonstrates DiskBETree with the default GobCodec.
package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/aalhour/fractaltree"
)

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
	f := newMemFlusher[string, int]()
	t, err := fractaltree.NewDisk[string, int](f)
	if err != nil {
		log.Fatal(err)
	}

	t.Put("alice", 100)
	t.Put("bob", 200)
	t.Put("charlie", 300)

	fmt.Println("All entries:")
	for k, v := range t.All() {
		fmt.Printf("  %s -> %d\n", k, v)
	}

	v, ok := t.Get("bob")
	fmt.Printf("\nGet(bob) = %d, ok=%v\n", v, ok)
	fmt.Println("Len:", t.Len())

	// Show gob encoded sizes.
	gc := fractaltree.GobCodec[string]{}
	kb, _ := gc.Encode("alice")
	fmt.Printf("\nGobCodec(\"alice\"): %d bytes\n", len(kb))

	if err := t.Close(); err != nil {
		log.Fatal(err)
	}
}
