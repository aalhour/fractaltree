package fractaltree

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// --- Put benchmarks ---

func BenchmarkPut(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000, 1_000_000} {
		b.Run("Sequential/"+strconv.Itoa(n), func(b *testing.B) {
			benchPutSequential(b, n)
		})
		b.Run("Random/"+strconv.Itoa(n), func(b *testing.B) {
			benchPutRandom(b, n)
		})
	}
}

func benchPutSequential(b *testing.B, n int) {
	b.Helper()
	b.ReportAllocs()
	for range b.N {
		t, _ := New[int, int]()
		for i := range n {
			t.Put(i, i)
		}
	}
}

func benchPutRandom(b *testing.B, n int) {
	b.Helper()
	keys := rand.Perm(n)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		t, _ := New[int, int]()
		for _, k := range keys {
			t.Put(k, k)
		}
	}
}

// --- Get benchmarks ---

func BenchmarkGet(b *testing.B) {
	const n = 100_000
	t := buildBenchTree(b)

	b.Run("Hit", func(b *testing.B) {
		keys := rand.Perm(n)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			for _, k := range keys {
				t.Get(k)
			}
		}
	})

	b.Run("Miss", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			for i := n; i < n+n; i++ {
				t.Get(i)
			}
		}
	})
}

// --- Delete benchmark ---

func BenchmarkDelete(b *testing.B) {
	const n = 100_000
	b.ReportAllocs()
	for range b.N {
		t := buildBenchTree(b)
		b.StartTimer()
		for i := range n {
			t.Delete(i)
		}
		b.StopTimer()
	}
}

// --- Range benchmark ---

func BenchmarkRange(b *testing.B) {
	const n = 100_000
	t := buildBenchTree(b)

	for _, count := range []int{10, 100, 1_000, 10_000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			lo := n/2 - count/2
			hi := lo + count
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for range t.Range(lo, hi) { //nolint:revive // intentionally draining iterator
				}
			}
		})
	}
}

// --- Mixed benchmark (80% read, 20% write) ---

func BenchmarkMixed(b *testing.B) {
	const n = 100_000
	t := buildBenchTree(b)
	keys := rand.Perm(n)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for i, k := range keys {
			if i%5 == 0 {
				t.Put(k, k+1)
			} else {
				t.Get(k)
			}
		}
	}
}

// --- Upsert benchmark ---

func BenchmarkUpsert(b *testing.B) {
	const n = 10_000
	b.ReportAllocs()
	for range b.N {
		t, _ := New[int, int]()
		for i := range n {
			t.Upsert(i%100, Increment(1))
		}
	}
}

// --- Helpers ---

const benchTreeSize = 100_000

func buildBenchTree(b *testing.B) *BETree[int, int] {
	b.Helper()
	t, err := New[int, int]()
	require.NoError(b, err)
	for i := range benchTreeSize {
		t.Put(i, i)
	}
	b.ResetTimer()
	return t
}
