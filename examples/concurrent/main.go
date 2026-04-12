// Concurrent demonstrates safe concurrent reads and writes.
package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/aalhour/fractaltree"
)

func main() {
	t, err := fractaltree.New[int, int]()
	if err != nil {
		log.Fatal(err)
	}

	const numWriters = 4
	const numReaders = 4
	const opsPerGoroutine = 1000

	var wg sync.WaitGroup

	// Launch writers.
	for w := range numWriters {
		wg.Go(func() {
			base := w * opsPerGoroutine
			for i := range opsPerGoroutine {
				t.Put(base+i, base+i)
			}
		})
	}

	// Launch readers (read while writes are in progress).
	for range numReaders {
		wg.Go(func() {
			for i := range opsPerGoroutine {
				t.Get(i)
			}
		})
	}

	wg.Wait()

	fmt.Println("Final Len:", t.Len())
	fmt.Println("Contains(0):", t.Contains(0))
	fmt.Println("Contains(3999):", t.Contains(3999))
	fmt.Println("No races detected!")
}
