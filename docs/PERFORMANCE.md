# Performance

All benchmarks run on **Apple M2 Max, 32 GB RAM, Go 1.26.2, darwin/arm64**.

Benchmark source: [`bench_test.go`](../bench_test.go). Cross-implementation source: [`testdata/btree_compare_test.go`](../testdata/btree_compare_test.go). Raw data and `benchstat` diffs are in [`benchmarks/`](../benchmarks/).

---

## Benchmarks

### Write Performance (Put)

| Workload | N | Time | Allocs |
|:---------|--:|-----:|-------:|
| Sequential | 1K | 33 &micro;s | 6 |
| Sequential | 10K | 600 &micro;s | 62 |
| Sequential | 100K | 9.0 ms | 768 |
| Sequential | 1M | 114 ms | 7,980 |
| Random | 1K | 114 &micro;s | 6 |
| Random | 10K | 2.8 ms | 62 |
| Random | 100K | 59.6 ms | 414 |
| Random | 1M | 1,344 ms | 4,849 |

### Read Performance (Get, 100K keys)

| Workload | Time / 100K ops | Allocs |
|:---------|----------------:|-------:|
| Hit (random) | 15.1 ms | 100,000 |
| Miss | 5.0 ms | 0 |

### Range Scans (100K key tree)

| Result count | Time | Allocs |
|-------------:|-----:|-------:|
| 10 | 678 ns | 19 |
| 100 | 3.9 &micro;s | 112 |
| 1K | 32.6 &micro;s | 1,015 |
| 10K | 383 &micro;s | 10,023 |

### Other Operations

| Operation | Time | Notes |
|:----------|-----:|:------|
| Delete (100K) | 17.0 ms | Sequential delete all keys |
| Upsert/Increment (10K) | 414 &micro;s | 100 counters, 10K total increments |
| Mixed 80/20 read-heavy (100K) | 34.8 ms | 80% reads, 20% writes |
| Mixed 80/20 write-heavy (100K) | 83.6 ms | 80% writes, 20% reads |

---

## Comparison with Google BTree

Head-to-head benchmark using identical workloads, same machine, same session (`count=6`). Google's [`btree`](https://github.com/google/btree) v1.1.3 with degree 32. Reusable benchmark at [`testdata/btree_compare_test.go`](../testdata/btree_compare_test.go).

The results are organized by **design point**: which data structure the workload is designed to favor.

### B&epsilon;-tree design point

These workloads play to the B&epsilon;-tree's strengths: sequential/batched writes and allocation-efficient operations.

#### Write (Put) &mdash; Sequential

| N | FractalTree | Google BTree | Ratio | Winner |
|--:|------------:|-------------:|------:|:-------|
| 1K | 32.2 &micro;s | 58.4 &micro;s | **0.55x** | FractalTree |
| 10K | 595 &micro;s | 744 &micro;s | **0.80x** | FractalTree |
| 100K | 8.9 ms | 9.7 ms | **0.92x** | FractalTree |

**FractalTree wins all sequential write sizes** &mdash; the append fast-path + batch flush beats per-key B-tree node operations.

#### Allocation Efficiency

| Operation | FractalTree | Google BTree |
|:----------|------------:|-------------:|
| Put/Sequential/100K | 768 allocs | 109,905 allocs |
| Put/Random/100K | 413 allocs | 106,918 allocs |
| Get/Hit (100K) | 100,000 allocs | 100,000 allocs |
| Get/Miss (100K) | 0 allocs | 100,000 allocs |
| Delete (100K) | 100,006 allocs | 100,000 allocs |

FractalTree achieves **99.6% fewer allocations** on write operations. Both implementations now allocate on read hits (8 B/op for FractalTree vs 16 B/op for Google BTree).

### B-tree design point

These workloads play to the B-tree's strengths: random in-memory writes and in-place range traversal.

#### Write (Put) &mdash; Random

| N | FractalTree | Google BTree | Ratio |
|--:|------------:|-------------:|------:|
| 1K | 116 &micro;s | 93.5 &micro;s | 1.24x |
| 10K | 2.8 ms | 1.5 ms | 1.91x |
| 100K | 60.6 ms | 23.2 ms | 2.62x |

Random writes favor the B-tree in-memory due to its cache-friendly layout. The amortized I/O advantage of B&epsilon;-trees is most visible in **disk-backed** scenarios where the cost of random I/O dwarfs in-memory pointer chasing.

#### Range Scan (100K key tree)

| Count | FractalTree | Google BTree | Ratio |
|------:|------------:|-------------:|------:|
| 10 | 716 ns | 141 ns | 5.1x |
| 100 | 4.4 &micro;s | 642 ns | 6.9x |
| 1K | 35.6 &micro;s | 5.4 &micro;s | 6.6x |
| 10K | 412 &micro;s | 58 &micro;s | 7.1x |

Google BTree walks the tree in-place with zero materialization. FractalTree must collect and merge pending buffer messages with leaf data, which costs O(result count) allocations.

### Neutral ground

These workloads don't strongly favor either design. Results depend on the specific operation mix and key distribution.

#### Read (Get, 100K keys)

| Workload | FractalTree | Google BTree | Ratio | Winner |
|:---------|------------:|-------------:|------:|:-------|
| Hit | 15.5 ms | 21.7 ms | **0.71x** | FractalTree |
| Miss | 5.1 ms | 7.4 ms | **0.69x** | FractalTree |

FractalTree wins both read workloads. Sorted buffers with binary search (O(log B) per level) keep the read path competitive despite the buffer-checking overhead.

#### Mixed (100K keys)

| Workload | FractalTree | Google BTree | Ratio |
|:---------|------------:|-------------:|------:|
| Read-heavy (80R/20W) | 38.2 ms | 28.3 ms | 1.35x |
| Write-heavy (80W/20R) | 90.8 ms | 29.0 ms | 3.13x |

The mixed benchmarks use random overwrites of existing keys, which is the B-tree's in-memory strength. For workloads with sequential or new-key writes, the B&epsilon;-tree's buffered write path would narrow this gap.

#### Delete (100K keys)

| | FractalTree | Google BTree | Ratio |
|:---------|------------:|-------------:|------:|
| Time | 17.6 ms | 10.3 ms | 1.71x |
| Allocs/op | 100,006 | 100,000 | &mdash; |

---

## Analysis

The comparison reveals the expected tradeoff profile of a B&epsilon;-tree vs a B-tree:

- **Sequential writes** are faster at every size tested (1K&ndash;100K). Buffer insertion + batch flush beats per-key B-tree node splitting.
- **Point reads** are faster in both hit and miss cases (0.71x and 0.69x respectively), thanks to sorted buffers with O(log B) binary search at each level.
- **Range scans** favor the B-tree (5&ndash;7x), but the gap has narrowed from 27&ndash;1,843x in v0.3.0 thanks to the collect-and-merge range iterator (A1).
- **Random writes** favor the B-tree in-memory (1.2&ndash;2.6x) due to cache-friendly layout. The B&epsilon;-tree's amortized I/O advantage is most visible in disk-backed scenarios.
- **Mixed workloads** favor the B-tree for random overwrite patterns. The B&epsilon;-tree's advantage appears with sequential or new-key writes.

**When to choose FractalTree over a B-tree:**

- Your workload is **write-heavy with sequential or batched keys** &mdash; FractalTree is faster at all sizes.
- Your workload is **disk-backed** &mdash; the batched flush model reduces random I/O by orders of magnitude.
- You care about **write-path allocation pressure / GC load** &mdash; FractalTree uses 99%+ fewer allocations on writes.
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
