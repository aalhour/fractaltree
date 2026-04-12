// Prefixscan demonstrates scanning all keys with a common prefix using
// Range. For string keys, Range("prefix:", "prefix;") captures all keys
// starting with "prefix:" because ";" is the next ASCII character after ":".
package main

import (
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

func main() {
	t, err := fractaltree.New[string, string]()
	if err != nil {
		log.Fatal(err)
	}

	// Insert keys with various prefixes.
	entries := []struct{ k, v string }{
		{"config:db.host", "localhost"},
		{"config:db.port", "5432"},
		{"config:db.name", "myapp"},
		{"config:cache.ttl", "300"},
		{"config:cache.size", "1024"},
		{"user:1:name", "alice"},
		{"user:1:email", "alice@example.com"},
		{"user:2:name", "bob"},
		{"user:2:email", "bob@example.com"},
		{"user:10:name", "jack"},
		{"session:abc123", "user:1"},
		{"session:def456", "user:2"},
	}

	for _, e := range entries {
		t.Put(e.k, e.v)
	}

	fmt.Printf("Total keys: %d\n\n", t.Len())

	// Prefix scan: all "config:" keys.
	// The trick: "config;" is the exclusive upper bound because
	// ';' (0x3B) is the next character after ':' (0x3A) in ASCII.
	fmt.Println("=== config:* ===")
	for k, v := range t.Range("config:", "config;") {
		fmt.Printf("  %-25s = %s\n", k, v)
	}

	// Prefix scan: all "user:1:" keys.
	fmt.Println("\n=== user:1:* ===")
	for k, v := range t.Range("user:1:", "user:1;") {
		fmt.Printf("  %-25s = %s\n", k, v)
	}

	// Prefix scan: all "user:" keys.
	fmt.Println("\n=== user:* ===")
	for k, v := range t.Range("user:", "user;") {
		fmt.Printf("  %-25s = %s\n", k, v)
	}

	// Prefix scan: all "session:" keys.
	fmt.Println("\n=== session:* ===")
	for k, v := range t.Range("session:", "session;") {
		fmt.Printf("  %-25s = %s\n", k, v)
	}

	// Count keys by prefix using Range.
	prefixes := []string{"config:", "user:", "session:"}
	fmt.Println("\n=== Key counts by prefix ===")
	for _, p := range prefixes {
		count := 0
		for range t.Range(p, p[:len(p)-1]+";") {
			count++
		}
		fmt.Printf("  %-12s %d keys\n", p+"*", count)
	}
}
