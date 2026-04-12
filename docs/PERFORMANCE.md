# Performance

All benchmarks run on **Apple M2 Max, 32 GB RAM, Go 1.26.2, darwin/arm64**.

Benchmark source: [`bench_test.go`](../bench_test.go). Cross-implementation source: [`testdata/btree_compare_test.go`](../testdata/btree_compare_test.go). Raw data and `benchstat` diffs are in [`benchmarks/`](../benchmarks/).

---

## Benchmarks

### Write Performance (Put)

| Workload | N | Time | Allocs |
|:---------|--:|-----:|-------:|
| Sequential | 1K | 33 &micro;s | 6 |
| Sequential | 10K | 464 &micro;s | 62 |
| Sequential | 100K | 6.5 ms | 768 |
| Sequential | 1M | 84 ms | 7,979 |
| Random | 1K | 115 &micro;s | 6 |
| Random | 10K | 2.8 ms | 62 |
| Random | 100K | 50 ms | 415 |
| Random | 1M | 864 ms | 4,925 |

### Read Performance (Get, 100K keys)

| Workload | Time / 100K ops | Allocs |
|:---------|----------------:|-------:|
| Hit (random) | 17.9 ms | 0 |
| Miss | 8.9 ms | 0 |

**Zero allocations on reads.**

### Range Scans (100K key tree)

| Result count | Time |
|-------------:|-----:|
| 10 | 259 &micro;s |
| 100 | 270 &micro;s |
| 1K | 385 &micro;s |
| 10K | 1.55 ms |

### Other Operations

| Operation | Time | Notes |
|:----------|-----:|:------|
| Delete (100K) | 18.3 ms | Sequential delete all keys |
| Upsert/Increment (10K) | 480 &micro;s | 100 counters, 10K total increments |
| Mixed 80/20 (100K) | 37.3 ms | 80% reads, 20% writes |

---

## Comparison with Google BTree

Head-to-head benchmark using identical workloads, same machine, same session (`count=6`). Google's [`btree`](https://github.com/google/btree) v1.1.3 with degree 32. Reusable benchmark at [`testdata/btree_compare_test.go`](../testdata/btree_compare_test.go).

### Write (Put) &mdash; Sequential

| N | FractalTree | Google BTree | Ratio | Winner |
|--:|------------:|-------------:|------:|:-------|
| 1K | 34.4 &micro;s | 58.9 &micro;s | **0.58x** | FractalTree |
| 10K | 471 &micro;s | 756 &micro;s | **0.62x** | FractalTree |
| 100K | 6.6 ms | 9.7 ms | **0.68x** | FractalTree |

**FractalTree wins all sequential write sizes** &mdash; the append fast-path + batch flush beats per-key B-tree node operations.

### Write (Put) &mdash; Random

| N | FractalTree | Google BTree | Ratio |
|--:|------------:|-------------:|------:|
| 1K | 114 &micro;s | 91 &micro;s | 1.25x |
| 10K | 2.7 ms | 1.5 ms | 1.87x |
| 100K | 50.4 ms | 22.6 ms | 2.23x |

### Read (Get, 100K keys)

| Workload | FractalTree | Google BTree | Ratio | Winner |
|:---------|------------:|-------------:|------:|:-------|
| Hit | 17.9 ms | 21.3 ms | **0.84x** | FractalTree |
| Miss | 8.9 ms | 7.4 ms | 1.21x | Google BTree |

**FractalTree wins on read hits** &mdash; zero allocations vs 100K allocs/op from interface boxing in Google BTree.

### Range Scan (100K key tree)

| Count | FractalTree | Google BTree | Ratio |
|------:|------------:|-------------:|------:|
| 10 | 260 &micro;s | 141 ns | 1,843x |
| 100 | 272 &micro;s | 655 ns | 416x |
| 1K | 391 &micro;s | 5.4 &micro;s | 72x |
| 10K | 1.56 ms | 57.6 &micro;s | 27x |

### Mixed (80% Read, 20% Write, 100K keys)

| | FractalTree | Google BTree | Ratio |
|:---------|------------:|-------------:|------:|
| Time | 37.6 ms | 23.7 ms | 1.59x |
| Allocs/op | 0 | 100,000 | &mdash; |

### Delete (100K keys)

| | FractalTree | Google BTree | Ratio |
|:---------|------------:|-------------:|------:|
| Time | 18.5 ms | 10.2 ms | 1.81x |
| Allocs/op | 6 | 100,000 | &mdash; |

### Allocation Efficiency

| Operation | FractalTree | Google BTree |
|:----------|------------:|-------------:|
| Put/Sequential/100K | 768 allocs | 109,905 allocs |
| Put/Random/100K | 413 allocs | 106,911 allocs |
| Get/Hit (100K) | 0 allocs | 100,000 allocs |
| Mixed 80/20 (100K) | 0 allocs | 100,000 allocs |
| Delete (100K) | 6 allocs | 100,000 allocs |

FractalTree achieves **99.6&ndash;100% fewer allocations** than Google BTree on most operations.

---

## Analysis

The comparison reveals the expected tradeoff profile of a B&epsilon;-tree vs a B-tree:

- **Sequential writes** are faster at every size tested (1K&ndash;100K). Buffer insertion + batch flush beats per-key B-tree node splitting.
- **Read hits** are faster (0.84x) thanks to zero allocations &mdash; Google BTree pays 100K interface-boxing allocations per 100K lookups.
- **Read misses** slightly favor the B-tree due to its shallower traversal and direct comparison path.
- **Range scans** heavily favor the B-tree. FractalTree materializes a snapshot (sort + dedup) while Google BTree walks the tree in-place.
- **Random writes** favor the B-tree in-memory due to its cache-friendly layout. The amortized I/O advantage of B&epsilon;-trees is most visible in **disk-backed** scenarios where the cost of random I/O dwarfs in-memory pointer chasing.

**When to choose FractalTree over a B-tree:**

- Your workload is **write-heavy with sequential or batched keys** &mdash; FractalTree is faster at all sizes.
- Your workload is **disk-backed** &mdash; the batched flush model reduces random I/O by orders of magnitude.
- You care about **allocation pressure / GC load** &mdash; FractalTree uses 99%+ fewer allocations on most operations.
- You need **Upsert / Increment / CompareAndSwap** semantics &mdash; B&epsilon;-trees support these natively via the message buffer.
- You need **range deletion** as a first-class operation.
- You want a **pluggable persistence layer** with codec support.
- You are building a **storage engine** and want the internal architecture of a fractal tree rather than a B-tree.

---

## Quality

| Metric | Result |
|:-------|:-------|
| Test coverage | **97.8%** (library code) |
| Total tests | **159** (unit + integration + fuzz + bench + stress + examples) |
| Race detector | All tests pass with `-race` |
| Fuzz testing | **FuzzOperations**: 1.2M+ executions, 0 failures |
| | **FuzzRange**: 430K+ executions, 0 failures |
| Stress tests | 4 concurrent scenarios (8 writers + 8 readers, 100K ops each) |
| Lint | 0 issues (golangci-lint v2, 27 linters enabled) |

---

## Reproducing Benchmarks

```bash
# Run all benchmarks (single iteration)
make bench

# Statistically rigorous run (count=6, required for benchstat)
go test -bench=. -benchmem -count=6 -timeout=30m . > bench.out

# Compare against baseline
benchstat benchmarks/baseline.txt bench.out

# Cross-implementation comparison (requires google/btree in testdata go.mod)
go test -tags compare -bench=. -benchmem -count=6 -timeout=30m ./testdata/

# Update baseline after a release
cp bench.out benchmarks/baseline.txt
```

### Investigating Regressions

| What | Command |
|:-----|:--------|
| CPU profile | `make profile-cpu` then `go tool pprof cpu.prof` |
| Memory profile | `make profile-mem` |
| Allocation profile (rate=1) | `make profile-alloc` |
| Flame graph | `make flamegraph` |
| Escape analysis | `make escape` |

See [`CONTRIBUTING.md`](../CONTRIBUTING.md) for the full benchmark workflow.
