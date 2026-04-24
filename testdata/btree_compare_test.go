// Package testdata contains cross-implementation benchmarks.
//
// This file compares fractaltree against google/btree v1.1.3 using identical
// workloads. Run from the repo root:
//
//	go test -tags compare -bench=. -benchmem -count=6 -timeout=30m ./testdata/
//
// The build tag keeps these out of normal CI runs (google/btree is not a
// runtime dependency).
package testdata

import (
	"math/rand/v2"
	"strconv"
	"testing"

	"github.com/aalhour/fractaltree"
	"github.com/google/btree"
)

// --- Google BTree helpers ---

type kv struct {
	key   int
	value int
}

func (a kv) Less(b btree.Item) bool { return a.key < b.(kv).key }

// --- Write (Put) ---

func BenchmarkCompare_Put(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run("Sequential/"+strconv.Itoa(n), func(b *testing.B) {
			b.Run("FractalTree", func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					t, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
					for i := range n {
						t.Put(i, i)
					}
				}
			})
			b.Run("GoogleBTree", func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					t := btree.New(32)
					for i := range n {
						t.ReplaceOrInsert(kv{key: i, value: i})
					}
				}
			})
		})
		b.Run("Random/"+strconv.Itoa(n), func(b *testing.B) {
			keys := rand.Perm(n)
			b.Run("FractalTree", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					t, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
					for _, k := range keys {
						t.Put(k, k)
					}
				}
			})
			b.Run("GoogleBTree", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					t := btree.New(32)
					for _, k := range keys {
						t.ReplaceOrInsert(kv{key: k, value: k})
					}
				}
			})
		})
	}
}

// --- Read (Get, 100K keys) ---

func BenchmarkCompare_Get(b *testing.B) {
	const n = 100_000

	ft, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
	gt := btree.New(32)
	for i := range n {
		ft.Put(i, i)
		gt.ReplaceOrInsert(kv{key: i, value: i})
	}

	b.Run("Hit", func(b *testing.B) {
		keys := rand.Perm(n)
		b.Run("FractalTree", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for _, k := range keys {
					ft.Get(k)
				}
			}
		})
		b.Run("GoogleBTree", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				for _, k := range keys {
					gt.Get(kv{key: k})
				}
			}
		})
	})

	b.Run("Miss", func(b *testing.B) {
		b.Run("FractalTree", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				for i := n; i < n+n; i++ {
					ft.Get(i)
				}
			}
		})
		b.Run("GoogleBTree", func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				for i := n; i < n+n; i++ {
					gt.Get(kv{key: i})
				}
			}
		})
	})
}

// --- Range Scan (100K key tree) ---

func BenchmarkCompare_Range(b *testing.B) {
	const n = 100_000

	ft, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
	gt := btree.New(32)
	for i := range n {
		ft.Put(i, i)
		gt.ReplaceOrInsert(kv{key: i, value: i})
	}

	for _, count := range []int{10, 100, 1_000, 10_000} {
		lo := n/2 - count/2
		hi := lo + count
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			b.Run("FractalTree", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					for range ft.Range(lo, hi) {
					}
				}
			})
			b.Run("GoogleBTree", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					gt.AscendRange(kv{key: lo}, kv{key: hi}, func(_ btree.Item) bool {
						return true
					})
				}
			})
		})
	}
}

// --- Mixed (80% Read, 20% Write, 100K keys) ---

func BenchmarkCompare_MixedReadHeavy(b *testing.B) {
	const n = 100_000

	ft, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
	gt := btree.New(32)
	for i := range n {
		ft.Put(i, i)
		gt.ReplaceOrInsert(kv{key: i, value: i})
	}
	keys := rand.Perm(n)

	b.Run("FractalTree", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			for i, k := range keys {
				if i%5 == 0 {
					ft.Put(k, k+1)
				} else {
					ft.Get(k)
				}
			}
		}
	})
	b.Run("GoogleBTree", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			for i, k := range keys {
				if i%5 == 0 {
					gt.ReplaceOrInsert(kv{key: k, value: k + 1})
				} else {
					gt.Get(kv{key: k})
				}
			}
		}
	})
}

// --- Mixed (80% Write, 20% Read, 100K keys) ---

func BenchmarkCompare_MixedWriteHeavy(b *testing.B) {
	const n = 100_000

	ft, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
	gt := btree.New(32)
	for i := range n {
		ft.Put(i, i)
		gt.ReplaceOrInsert(kv{key: i, value: i})
	}
	keys := rand.Perm(n)

	b.Run("FractalTree", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			for i, k := range keys {
				if i%5 < 4 {
					ft.Put(k, k+1)
				} else {
					ft.Get(k)
				}
			}
		}
	})
	b.Run("GoogleBTree", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			for i, k := range keys {
				if i%5 < 4 {
					gt.ReplaceOrInsert(kv{key: k, value: k + 1})
				} else {
					gt.Get(kv{key: k})
				}
			}
		}
	})
}

// --- Delete (100K keys) ---

func BenchmarkCompare_Delete(b *testing.B) {
	const n = 100_000

	b.Run("FractalTree", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			t, _ := fractaltree.New[int, int](fractaltree.WithBlockSize(63))
			for i := range n {
				t.Put(i, i)
			}
			b.StartTimer()
			for i := range n {
				t.Delete(i)
			}
		}
	})
	b.Run("GoogleBTree", func(b *testing.B) {
		for range b.N {
			b.StopTimer()
			t := btree.New(32)
			for i := range n {
				t.ReplaceOrInsert(kv{key: i, value: i})
			}
			b.StartTimer()
			for i := range n {
				t.Delete(kv{key: i})
			}
		}
	})
}
