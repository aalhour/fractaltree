// Basic demonstrates Put, Get, Delete, Contains, Len, and Clear.
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

	// Insert some key-value pairs.
	t.Put("alice", 100)
	t.Put("bob", 200)
	t.Put("charlie", 300)
	fmt.Println("Len after 3 puts:", t.Len())

	// Retrieve a value.
	v, ok := t.Get("bob")
	fmt.Printf("Get(bob) = %d, ok=%v\n", v, ok)

	// Check existence.
	fmt.Println("Contains(alice):", t.Contains("alice"))
	fmt.Println("Contains(dave):", t.Contains("dave"))

	// Overwrite a value.
	t.Put("alice", 999)
	v, _ = t.Get("alice")
	fmt.Println("alice after overwrite:", v)

	// Delete a key.
	deleted := t.Delete("bob")
	fmt.Println("Deleted bob:", deleted)
	fmt.Println("Len after delete:", t.Len())

	// Clear all entries.
	t.Clear()
	fmt.Println("Len after clear:", t.Len())
}
