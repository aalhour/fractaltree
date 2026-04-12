// Bulkimport benchmarks inserting 1M keys to demonstrate the write
// amortization benefit of the B-epsilon-tree's message buffering.
package main

import (
	"fmt"
	"log"
	"math/rand/v2"
	"runtime"
	"time"

	"github.com/aalhour/fractaltree"
)

func main() {
	const n = 1_000_000

	// Generate random keys upfront so key generation doesn't affect timing.
	keys := make([]int, n)
	for i := range keys {
		keys[i] = i
	}
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	// --- Sequential insert ---
	t1, err := fractaltree.New[int, int]()
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	for i := range n {
		t1.Put(i, i)
	}
	seqDur := time.Since(start)

	fmt.Printf("Sequential insert:  %d keys in %v (%.0f ops/sec)\n",
		n, seqDur, float64(n)/seqDur.Seconds())

	// --- Random insert ---
	t2, err := fractaltree.New[int, int]()
	if err != nil {
		log.Fatal(err)
	}

	start = time.Now()
	for _, k := range keys {
		t2.Put(k, k)
	}
	randDur := time.Since(start)

	fmt.Printf("Random insert:      %d keys in %v (%.0f ops/sec)\n",
		n, randDur, float64(n)/randDur.Seconds())

	// --- Point reads (random) ---
	start = time.Now()
	hits := 0
	for _, k := range keys {
		if _, ok := t2.Get(k); ok {
			hits++
		}
	}
	readDur := time.Since(start)

	fmt.Printf("Random reads:       %d keys in %v (%.0f ops/sec), hits=%d\n",
		n, readDur, float64(n)/readDur.Seconds(), hits)

	// --- Memory usage ---
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("\nHeap in use:        %.1f MB\n", float64(m.HeapInuse)/(1024*1024))
	fmt.Printf("Total alloc:        %.1f MB\n", float64(m.TotalAlloc)/(1024*1024))

	// --- Verify correctness ---
	fmt.Printf("\nt1.Len() = %d (sequential)\n", t1.Len())
	fmt.Printf("t2.Len() = %d (random)\n", t2.Len())
}
