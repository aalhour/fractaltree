// Upsert demonstrates Upsert, PutIfAbsent, Increment, and CompareAndSwap.
package main

import (
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

func main() {
	t, err := fractaltree.New[string, int]()
	if err != nil {
		log.Fatal(err)
	}

	// Word frequency counter using Increment.
	words := []string{"go", "tree", "go", "fractal", "tree", "go"}
	for _, w := range words {
		t.Upsert(w, fractaltree.Increment(1))
	}

	fmt.Println("Word counts:")
	for k, v := range t.All() {
		fmt.Printf("  %s: %d\n", k, v)
	}

	// PutIfAbsent — only inserts if key is missing.
	inserted := t.PutIfAbsent("go", 999)
	fmt.Printf("\nPutIfAbsent(go, 999) inserted: %v\n", inserted)
	v, _ := t.Get("go")
	fmt.Println("go count unchanged:", v)

	inserted = t.PutIfAbsent("new", 1)
	fmt.Printf("PutIfAbsent(new, 1) inserted: %v\n", inserted)

	// CompareAndSwap — conditional update.
	t2, _ := fractaltree.New[string, string]()
	t2.Put("status", "pending")

	t2.Upsert("status", fractaltree.CompareAndSwap("pending", "active"))
	s, _ := t2.Get("status")
	fmt.Printf("\nCAS pending->active: status=%s\n", s)

	t2.Upsert("status", fractaltree.CompareAndSwap("pending", "closed"))
	s, _ = t2.Get("status")
	fmt.Printf("CAS pending->closed (mismatch): status=%s\n", s)
}
