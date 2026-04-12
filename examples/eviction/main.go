// Eviction demonstrates LRU-style cache eviction using a composite key
// that orders by access time, allowing Ascend to find the oldest entries.
package main

import (
	"cmp"
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

// cacheEntry orders by last access time, then by key for uniqueness.
type cacheEntry struct {
	lastAccess int64
	key        string
}

func compareCacheEntries(a, b cacheEntry) int {
	if c := cmp.Compare(a.lastAccess, b.lastAccess); c != 0 {
		return c
	}
	return cmp.Compare(a.key, b.key)
}

func main() {
	// The eviction index: maps cacheEntry -> value.
	cache, err := fractaltree.NewWithCompare[cacheEntry, string](compareCacheEntries)
	if err != nil {
		log.Fatal(err)
	}

	// Simulate cache accesses with timestamps (max 5 entries).
	accesses := []cacheEntry{
		{lastAccess: 1, key: "user:1"},
		{lastAccess: 2, key: "user:2"},
		{lastAccess: 3, key: "user:3"},
		{lastAccess: 4, key: "user:4"},
		{lastAccess: 5, key: "user:5"},
	}

	for _, e := range accesses {
		cache.Put(e, "data-for-"+e.key)
	}

	fmt.Println("=== Cache (oldest first) ===")
	for k, v := range cache.Ascend() {
		fmt.Printf("  t=%d  %-8s -> %s\n", k.lastAccess, k.key, v)
	}

	// Cache is full. New entry arrives — evict the oldest.
	fmt.Println("\n--- New entry: user:6 at t=6, cache full ---")

	// Find and remove the oldest entry (first in Ascend order).
	c := cache.Cursor()
	c.Next() // position at oldest
	oldest := c.Key()
	c.Close()

	fmt.Printf("  Evicting: t=%d %s\n", oldest.lastAccess, oldest.key)
	cache.Delete(oldest)
	cache.Put(cacheEntry{lastAccess: 6, key: "user:6"}, "data-for-user:6")

	// Simulate re-access of user:2 at t=7 (touch = delete old + insert new).
	fmt.Println("  Re-accessing: user:2 at t=7")
	cache.Delete(cacheEntry{lastAccess: 2, key: "user:2"})
	cache.Put(cacheEntry{lastAccess: 7, key: "user:2"}, "data-for-user:2")

	fmt.Println("\n=== Cache after eviction + re-access ===")
	for k, v := range cache.Ascend() {
		fmt.Printf("  t=%d  %-8s -> %s\n", k.lastAccess, k.key, v)
	}
}
