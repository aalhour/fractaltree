// Comparator demonstrates NewWithCompare with a composite key struct.
package main

import (
	"cmp"
	"fmt"
	"log"

	"github.com/aalhour/fractaltree"
)

// TenantKey orders by Tenant first, then by ID.
type TenantKey struct {
	Tenant string
	ID     int64
}

func compareTenantKeys(a, b TenantKey) int {
	if c := cmp.Compare(a.Tenant, b.Tenant); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

func main() {
	t, err := fractaltree.NewWithCompare[TenantKey, string](compareTenantKeys)
	if err != nil {
		log.Fatal(err)
	}

	t.Put(TenantKey{"acme", 3}, "widget")
	t.Put(TenantKey{"acme", 1}, "gadget")
	t.Put(TenantKey{"beta", 2}, "thing")
	t.Put(TenantKey{"acme", 2}, "doohickey")

	fmt.Println("All entries in order:")
	for k, v := range t.All() {
		fmt.Printf("  {%s, %d} -> %s\n", k.Tenant, k.ID, v)
	}

	// Point query.
	v, ok := t.Get(TenantKey{"beta", 2})
	fmt.Printf("\nGet({beta, 2}) = %q, ok=%v\n", v, ok)
}
