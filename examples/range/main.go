// Range demonstrates Range queries, Descend, and Cursor usage.
package main

import (
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

func main() {
	t, err := fractaltree.New[int, string]()
	if err != nil {
		log.Fatal(err)
	}

	for i := 0; i < 100; i += 10 {
		t.Put(i, fmt.Sprintf("val%d", i))
	}

	// Range [20, 60) — ascending.
	fmt.Println("Range [20, 60):")
	for k, v := range t.Range(20, 60) {
		fmt.Printf("  %d -> %s\n", k, v)
	}

	// Descend — all keys, descending.
	fmt.Println("\nDescend (all):")
	for k, v := range t.Descend() {
		fmt.Printf("  %d -> %s\n", k, v)
	}

	// Cursor — seek to 45, then walk forward.
	fmt.Println("\nCursor: seek(45), then Next:")
	c := t.Cursor()
	defer c.Close()

	if c.Seek(45) {
		fmt.Printf("  %d -> %s\n", c.Key(), c.Value())
		for c.Next() {
			fmt.Printf("  %d -> %s\n", c.Key(), c.Value())
		}
	}
}
