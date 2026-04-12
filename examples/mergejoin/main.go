// Mergejoin demonstrates using two cursors to perform a merge join
// between two trees — finding keys that exist in both.
package main

import (
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

func main() {
	// Tree A: users who signed up this month.
	signups, err := fractaltree.New[int, string]()
	if err != nil {
		log.Fatal(err)
	}

	signups.Put(101, "alice")
	signups.Put(103, "charlie")
	signups.Put(105, "eve")
	signups.Put(107, "grace")
	signups.Put(109, "iris")
	signups.Put(111, "kate")

	// Tree B: users who made a purchase this month.
	purchases, err := fractaltree.New[int, string]()
	if err != nil {
		log.Fatal(err)
	}

	purchases.Put(100, "external-buyer")
	purchases.Put(103, "charlie")
	purchases.Put(105, "eve")
	purchases.Put(106, "frank")
	purchases.Put(109, "iris")
	purchases.Put(112, "liam")

	// Merge join: find users who both signed up AND purchased.
	fmt.Println("=== Users who signed up AND purchased ===")
	ca := signups.Cursor()
	cb := purchases.Cursor()
	defer ca.Close()
	defer cb.Close()

	aOk := ca.Next()
	bOk := cb.Next()

	matches := 0
	for aOk && bOk {
		if ca.Key() == cb.Key() {
			fmt.Printf("  user %d: %s\n", ca.Key(), ca.Value())
			matches++
			aOk = ca.Next()
			bOk = cb.Next()
		} else if ca.Key() < cb.Key() {
			aOk = ca.Next()
		} else {
			bOk = cb.Next()
		}
	}
	fmt.Printf("\nTotal matches: %d (out of %d signups, %d purchasers)\n",
		matches, signups.Len(), purchases.Len())

	// Anti-join: signups with NO purchase (left anti-join).
	fmt.Println("\n=== Signed up but did NOT purchase ===")
	ca2 := signups.Cursor()
	cb2 := purchases.Cursor()
	defer ca2.Close()
	defer cb2.Close()

	aOk = ca2.Next()
	bOk = cb2.Next()

	for aOk {
		if !bOk || ca2.Key() < cb2.Key() {
			fmt.Printf("  user %d: %s\n", ca2.Key(), ca2.Value())
			aOk = ca2.Next()
		} else if ca2.Key() == cb2.Key() {
			aOk = ca2.Next()
			bOk = cb2.Next()
		} else {
			bOk = cb2.Next()
		}
	}
}
