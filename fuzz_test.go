package fractaltree

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzOperations runs random Put/Get/Delete/Contains sequences against
// a BETree and verifies every result against a reference map.
func FuzzOperations(f *testing.F) {
	// Seed corpus with a few interesting byte patterns.
	f.Add([]byte{0, 1, 2, 3, 4, 5})
	f.Add([]byte{255, 0, 128, 64, 32, 16})
	f.Add([]byte{1, 1, 1, 1, 1, 1, 1, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		tree, err := New[int, int](WithBlockSize(4), WithEpsilon(0.5))
		require.NoError(t, err)

		ref := make(map[int]int)

		for i := 0; i+1 < len(data); i += 2 {
			op := data[i] % 4
			key := int(data[i+1])

			switch op {
			case 0: // Put
				val := key * 10
				tree.Put(key, val)
				ref[key] = val

			case 1: // Get
				got, ok := tree.Get(key)
				expected, exists := ref[key]
				if ok != exists {
					t.Fatalf("Get(%d): tree ok=%v, ref exists=%v", key, ok, exists)
				}
				if ok && got != expected {
					t.Fatalf("Get(%d): tree=%d, ref=%d", key, got, expected)
				}

			case 2: // Delete
				treeDeleted := tree.Delete(key)
				_, refExisted := ref[key]
				delete(ref, key)
				if treeDeleted != refExisted {
					t.Fatalf("Delete(%d): tree=%v, ref=%v", key, treeDeleted, refExisted)
				}

			case 3: // Contains
				got := tree.Contains(key)
				_, exists := ref[key]
				if got != exists {
					t.Fatalf("Contains(%d): tree=%v, ref=%v", key, got, exists)
				}
			}
		}

		// Final consistency check: tree length must match reference.
		if tree.Len() != len(ref) {
			t.Fatalf("Len mismatch: tree=%d, ref=%d", tree.Len(), len(ref))
		}
	})
}

// FuzzRange verifies Range queries against a sorted reference.
func FuzzRange(f *testing.F) {
	f.Add([]byte{10, 20, 30, 40, 50, 5, 35})
	f.Add([]byte{1, 2, 3, 4, 5, 0, 6})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 3 {
			return
		}

		tree, err := New[int, int](WithBlockSize(4), WithEpsilon(0.5))
		require.NoError(t, err)

		ref := make(map[int]int)

		// Use all but last 2 bytes as keys to insert.
		for _, b := range data[:len(data)-2] {
			key := int(b)
			tree.Put(key, key*10)
			ref[key] = key * 10
		}

		// Last 2 bytes define the range bounds.
		lo := int(data[len(data)-2])
		hi := int(data[len(data)-1])
		if lo > hi {
			lo, hi = hi, lo
		}

		// Collect tree range results.
		var treeKeys []int
		for k := range tree.Range(lo, hi) {
			treeKeys = append(treeKeys, k)
		}

		// Collect reference range results.
		var refKeys []int
		for k := range ref {
			if k >= lo && k < hi {
				refKeys = append(refKeys, k)
			}
		}

		if len(treeKeys) != len(refKeys) {
			t.Fatalf("Range(%d,%d): tree has %d results, ref has %d",
				lo, hi, len(treeKeys), len(refKeys))
		}
	})
}
