// Ratelimiter demonstrates a sliding-window rate limiter using Increment
// to count requests per time bucket and DeleteRange to expire old buckets.
package main

import (
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

// bucketKey combines a client ID and a time bucket (e.g., second).
// This lets us track per-client, per-second request counts.
type bucketKey struct {
	client string
	second int64
}

func compareBucketKeys(a, b bucketKey) int {
	if a.client < b.client {
		return -1
	}
	if a.client > b.client {
		return 1
	}
	if a.second < b.second {
		return -1
	}
	if a.second > b.second {
		return 1
	}
	return 0
}

func main() {
	t, err := fractaltree.NewWithCompare[bucketKey, int](compareBucketKeys)
	if err != nil {
		log.Fatal(err)
	}

	const limit = 5 // max requests per second

	// Simulate requests from "alice" across several seconds.
	requests := []struct {
		client string
		second int64
	}{
		{"alice", 100}, {"alice", 100}, {"alice", 100},
		{"alice", 101}, {"alice", 101}, {"alice", 101},
		{"alice", 101}, {"alice", 101}, {"alice", 101}, // 6 in second 101
		{"alice", 102}, {"alice", 102},
		{"bob", 100}, {"bob", 100}, {"bob", 100}, {"bob", 100},
	}

	for _, r := range requests {
		key := bucketKey{client: r.client, second: r.second}
		t.Upsert(key, fractaltree.Increment(1))
	}

	// Check rate limits.
	fmt.Println("=== Rate check ===")
	for k, v := range t.All() {
		status := "OK"
		if v > limit {
			status = "RATE LIMITED"
		}
		fmt.Printf("  client=%-6s second=%d  count=%d  %s\n", k.client, k.second, v, status)
	}

	// Expire buckets older than second 101 (sliding window).
	removed := t.DeleteRange(
		bucketKey{client: "alice", second: 0},
		bucketKey{client: "alice", second: 101},
	)
	fmt.Printf("\n=== Expired %d old alice buckets ===\n", removed)

	fmt.Println("\n=== After expiry ===")
	for k, v := range t.All() {
		fmt.Printf("  client=%-6s second=%d  count=%d\n", k.client, k.second, v)
	}
}
