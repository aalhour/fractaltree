// Mergejoin demonstrates using two cursors to perform a merge join
// between two trees — finding keys that exist in both.
package main

import (
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

func main() {
	signups := buildSignups()
	purchases := buildPurchases()

	mergeJoin(signups, purchases)
	antiJoin(signups, purchases)
}

func buildSignups() *fractaltree.BETree[int, string] {
	t, err := fractaltree.New[int, string]()
	if err != nil {
		log.Fatal(err)
	}
	t.Put(101, "alice")
	t.Put(103, "charlie")
	t.Put(105, "eve")
	t.Put(107, "grace")
	t.Put(109, "iris")
	t.Put(111, "kate")
	return t
}

func buildPurchases() *fractaltree.BETree[int, string] {
	t, err := fractaltree.New[int, string]()
	if err != nil {
		log.Fatal(err)
	}
	t.Put(100, "external-buyer")
	t.Put(103, "charlie")
	t.Put(105, "eve")
	t.Put(106, "frank")
	t.Put(109, "iris")
	t.Put(112, "liam")
	return t
}

// mergeJoin finds users present in both trees using two cursors
// advancing in lockstep.
func mergeJoin(signups, purchases *fractaltree.BETree[int, string]) {
	fmt.Println("=== Users who signed up AND purchased ===")
	ca := signups.Cursor()
	cb := purchases.Cursor()
	defer ca.Close()
	defer cb.Close()

	aOk := ca.Next()
	bOk := cb.Next()

	matches := 0
	for aOk && bOk {
		switch {
		case ca.Key() == cb.Key():
			fmt.Printf("  user %d: %s\n", ca.Key(), ca.Value())
			matches++
			aOk = ca.Next()
			bOk = cb.Next()
		case ca.Key() < cb.Key():
			aOk = ca.Next()
		default:
			bOk = cb.Next()
		}
	}
	fmt.Printf("\nTotal matches: %d (out of %d signups, %d purchasers)\n",
		matches, signups.Len(), purchases.Len())
}

// antiJoin finds signups with no corresponding purchase (left anti-join).
func antiJoin(signups, purchases *fractaltree.BETree[int, string]) {
	fmt.Println("\n=== Signed up but did NOT purchase ===")
	ca := signups.Cursor()
	cb := purchases.Cursor()
	defer ca.Close()
	defer cb.Close()

	aOk := ca.Next()
	bOk := cb.Next()

	for aOk {
		switch {
		case !bOk || ca.Key() < cb.Key():
			fmt.Printf("  user %d: %s\n", ca.Key(), ca.Value())
			aOk = ca.Next()
		case ca.Key() == cb.Key():
			aOk = ca.Next()
			bOk = cb.Next()
		default:
			bOk = cb.Next()
		}
	}
}
