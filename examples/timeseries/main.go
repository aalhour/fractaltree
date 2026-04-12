// Timeseries demonstrates using Range to query events within a time window.
// Events are keyed by Unix timestamp (int64) and values are event descriptions.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/aalhour/fractaltree"
)

func main() {
	t, err := fractaltree.New[int64, string]()
	if err != nil {
		log.Fatal(err)
	}

	// Simulate events over a day (using epoch seconds).
	base := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	events := []struct {
		offset time.Duration
		desc   string
	}{
		{0 * time.Hour, "server start"},
		{1 * time.Hour, "first user login"},
		{3 * time.Hour, "batch job started"},
		{4 * time.Hour, "batch job completed"},
		{6 * time.Hour, "traffic spike detected"},
		{8 * time.Hour, "deploy v2.1.0"},
		{10 * time.Hour, "cache miss rate elevated"},
		{12 * time.Hour, "midday health check"},
		{16 * time.Hour, "deploy v2.1.1 (hotfix)"},
		{20 * time.Hour, "daily backup started"},
		{22 * time.Hour, "daily backup completed"},
	}

	for _, e := range events {
		ts := base.Add(e.offset).Unix()
		t.Put(ts, e.desc)
	}

	fmt.Printf("Total events: %d\n\n", t.Len())

	// Query: what happened between 03:00 and 09:00?
	start := base.Add(3 * time.Hour).Unix()
	end := base.Add(9 * time.Hour).Unix()

	fmt.Println("=== Events from 03:00 to 09:00 ===")
	for ts, desc := range t.Range(start, end) {
		when := time.Unix(ts, 0).UTC().Format("15:04")
		fmt.Printf("  [%s] %s\n", when, desc)
	}

	// Query: last 3 events of the day (descend, take 3).
	fmt.Println("\n=== Last 3 events ===")
	count := 0
	for ts, desc := range t.Descend() {
		when := time.Unix(ts, 0).UTC().Format("15:04")
		fmt.Printf("  [%s] %s\n", when, desc)
		count++
		if count == 3 {
			break
		}
	}

	// Expire old events: delete everything before 10:00.
	cutoff := base.Add(10 * time.Hour).Unix()
	removed := t.DeleteRange(0, cutoff)
	fmt.Printf("\n=== Expired %d events before 10:00 ===\n", removed)
	fmt.Printf("Remaining: %d events\n", t.Len())
}
