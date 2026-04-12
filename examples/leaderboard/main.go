// Leaderboard demonstrates using Descend to maintain a live top-K leaderboard.
// Scores are stored with the score as the key (negated for descending order)
// and player name as the value.
package main

import (
	"cmp"
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

// Entry orders by score descending, then by name ascending for ties.
type Entry struct {
	Score int
	Name  string
}

func compareEntries(a, b Entry) int {
	// Higher score first (descending).
	if c := cmp.Compare(b.Score, a.Score); c != 0 {
		return c
	}
	// Alphabetical for ties.
	return cmp.Compare(a.Name, b.Name)
}

func main() {
	t, err := fractaltree.NewWithCompare[Entry, struct{}](compareEntries)
	if err != nil {
		log.Fatal(err)
	}

	// Simulate players submitting scores.
	scores := []Entry{
		{Score: 1500, Name: "alice"},
		{Score: 2300, Name: "bob"},
		{Score: 1800, Name: "charlie"},
		{Score: 2300, Name: "dave"},
		{Score: 3100, Name: "eve"},
		{Score: 900, Name: "frank"},
		{Score: 2100, Name: "grace"},
		{Score: 1800, Name: "heidi"},
	}
	for _, s := range scores {
		t.Put(s, struct{}{})
	}

	// Top 5 leaderboard using Ascend (entries are already sorted high-to-low
	// by our comparator).
	fmt.Println("=== Top 5 Leaderboard ===")
	rank := 0
	for k := range t.Ascend() {
		rank++
		fmt.Printf("  #%d  %-10s %d pts\n", rank, k.Name, k.Score)
		if rank == 5 {
			break
		}
	}

	// Update a score: remove old, insert new.
	t.Delete(Entry{Score: 900, Name: "frank"})
	t.Put(Entry{Score: 5000, Name: "frank"}, struct{}{})

	fmt.Println("\n=== After frank's big game ===")
	rank = 0
	for k := range t.Ascend() {
		rank++
		fmt.Printf("  #%d  %-10s %d pts\n", rank, k.Name, k.Score)
		if rank == 3 {
			break
		}
	}
}
