<p align="center">
  <img src="assets/logo.png" alt="FractalTree" width="400">
</p>

<h1 align="center">fractaltree</h1>

<p align="center">
  <strong>A write-optimized B&epsilon;-tree (fractal tree) for Go.</strong><br>
  Buffer writes, flush in batches, iterate in sorted order.
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/aalhour/fractaltree"><img src="https://img.shields.io/badge/pkg.go.dev-reference-007d9c?logo=go&logoColor=white" alt="Go Reference"></a>
  <a href="https://github.com/aalhour/fractaltree/releases"><img src="https://img.shields.io/github/v/release/aalhour/fractaltree?include_prereleases&sort=semver" alt="Version"></a>
  <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white" alt="Go Version">
  <a href="https://goreportcard.com/report/github.com/aalhour/fractaltree"><img src="https://goreportcard.com/badge/github.com/aalhour/fractaltree" alt="Go Report Card"></a>
  <img src="https://img.shields.io/badge/coverage-97.8%25-brightgreen" alt="Coverage">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/aalhour/fractaltree" alt="License"></a>
</p>

---

## What is this?

`fractaltree` is a pure-Go, generic, concurrent-safe implementation of the **B&epsilon;-tree** data structure. It is an ordered key-value store that is optimized for write-heavy workloads: instead of propagating every write to a leaf immediately (like a B-tree), it buffers mutations as messages in internal nodes and flushes them downward in batches. This amortizes the cost of random I/O across many writes.

The library works entirely in-memory by default, but ships with a pluggable **Flusher** interface and **Codec** system for disk persistence. Zero external runtime dependencies.

## Installation

```bash
go get github.com/aalhour/fractaltree@latest
```

## Quick Start

```go
package main

import (
    "fmt"
    "log"

    "github.com/aalhour/fractaltree"
)

func main() {
    t, err := fractaltree.New[string, int]()
    if err != nil {
        log.Fatal(err)
    }

    t.Put("alice", 100)
    t.Put("bob", 200)
    t.Put("charlie", 300)

    v, ok := t.Get("bob")
    fmt.Println(v, ok) // 200 true

    for k, v := range t.Range("alice", "charlie") {
        fmt.Println(k, v) // alice 100, bob 200
    }
}
```

---

## Background: B&epsilon;-Trees

### How It Works

A **fractal tree**, more formally known as an **B&epsilon;-tree** (pronounced "B-epsilon tree"), is a search tree from the family of *write-optimized data structures* introduced by Brodal and Fagerberg (2003) and later refined in the context of databases by Bender, Farach-Colton, Fineman, Fogel, Kuszmaul, and Nelson.

**Key papers:**

- G. S. Brodal and R. Fagerberg, [*Lower Bounds for External Memory Dictionaries*](https://cs.au.dk/~gerth/papers/soda03.pdf), SODA 2003 &mdash; introduced the B&epsilon;-tree model and proved optimal write-amortization bounds.
- M. A. Bender, M. Farach-Colton, J. T. Fineman, Y. R. Fogel, B. C. Kuszmaul, and J. Nelson, [*Cache-Oblivious Streaming B-trees*](https://people.csail.mit.edu/jfineman/sbtree.pdf), SICOMP 2007 &mdash; extended the model with streaming and cache-oblivious analysis.
- B. C. Kuszmaul, [*A Comparison of Fractal Trees to Log-Structured Merge (LSM) Trees*](https://www.appservgrid.com/refcards/refcards/dzonerefcards/bundles/Tokutek%20Whitepaper%20Bundle/lsm-vs-fractal.pdf), Tokutek White Paper 2014 &mdash; practical comparison of fractal trees vs LSM-trees as used in TokuDB.

The core mechanism is simple:

1. **Buffered writes.** Every internal node carries a *message buffer*. A write (put, delete, upsert) inserts a message into the root's buffer and returns immediately. No leaf is touched.
2. **Batch flush.** When a node's buffer fills up, all messages are sorted by key, partitioned by the node's pivot keys, and pushed to the appropriate child. If the child is a leaf, messages are applied directly. If it is internal, they land in *that* child's buffer.
3. **Reads check buffers.** A point query walks from root to leaf, collecting pending messages for the target key at every level. The most recent message wins (a `DELETE` cancels a prior `PUT`, an `UPSERT` transforms the value).

```
                    ┌──────────────────────────────────┐
         Root       │  pivots: [10, 20, 30]            │
       (internal)   │  buffer:  [PUT(25,"y"), DEL(7)]  │  ← writes land HERE first
                    └──┬──────┬──────┬──────┬──────────┘
                       │      │      │      │
                  <10  │ 10-20│ 20-30│  ≥30 │
                       ▼      ▼      ▼      ▼
                    [leaf]  [leaf] [leaf]  [leaf]
                    1,3,7   12,15  22,28   35,40
```

### The Epsilon Parameter

The parameter **&epsilon;** (epsilon, where 0 < &epsilon; &le; 1) controls how the block size B is divided between fanout and buffer capacity:

| &epsilon; | Fanout (B<sup>&epsilon;</sup>) | Buffer (B<sup>1-&epsilon;</sup>) | Character |
|:---------:|:---:|:---:|:---|
| 0.3 | 8 | 512 | Very large buffers, fastest bulk writes, slower reads |
| **0.5** | **64** | **64** | **Balanced (default)** |
| 0.7 | 512 | 8 | Small buffers, behaves closer to a B-tree |
| 1.0 | 4096 | 1 | Equivalent to a B-tree (no buffering) |

*Values shown for default B = 4096.*

### Complexity

| Operation | B-tree | B&epsilon;-tree |
|:----------|:-------|:----------------|
| Point read | O(log<sub>B</sub> N) | O(log<sub>B</sub> N) |
| Range (k results) | O(log<sub>B</sub> N + k/B) | O(log<sub>B</sub> N + k/B) |
| Insert / Delete | O(log<sub>B</sub> N) | **O(log<sub>B</sub> N / B<sup>1-&epsilon;</sup>)** amortized |

For &epsilon; = 0.5 and B = 4096, writes are **~64x fewer I/Os** than a B-tree in the amortized case.

### Use Cases

B&epsilon;-trees were designed for workloads where writes dominate reads:

- **Databases and storage engines** &mdash; TokuDB (MySQL), PerconaFT, BetrFS (file system). Any LSM-tree alternative that needs sorted order without compaction storms.
- **Time-series and event logging** &mdash; high-throughput append of timestamped records with occasional range scans.
- **Write-ahead logs with indexed access** &mdash; buffer mutations and flush periodically, while still supporting point lookups.
- **Counters and accumulators** &mdash; the `Upsert` / `Increment` pattern lets you bump values without reading first.
- **Rate limiters and leaderboards** &mdash; sorted structure supports both fast updates and ranked range queries.
- **ETL and bulk ingest pipelines** &mdash; batch-load millions of records, then scan or export in sorted order.

---

## Features

- **Generic** &mdash; works with any key type: `cmp.Ordered` types via `New`, composite keys via `NewWithCompare`.
- **Concurrent-safe** &mdash; `sync.RWMutex` with read-many / write-one semantics. Iterators take a snapshot and release the lock.
- **Rich query API** &mdash; `All`, `Ascend`, `Descend`, `Range`, `DescendRange` as Go 1.23+ `iter.Seq2` iterators, plus a stateful `Cursor` with `Seek` / `Next` / `Prev`.
- **Upsert primitives** &mdash; `Upsert(key, fn)`, `PutIfAbsent`, `Increment`, `CompareAndSwap` for atomic read-modify-write.
- **Range deletion** &mdash; `DeleteRange(lo, hi)` removes an entire key range in one call.
- **Disk persistence** &mdash; `DiskBETree` with pluggable `Flusher` interface and `Codec` system (ships with `GobCodec`).
- **Tunable** &mdash; `WithEpsilon` and `WithBlockSize` to control the write/read tradeoff.
- **Zero runtime dependencies** &mdash; only `stretchr/testify` and `uber/goleak` for tests.

---

## API Overview

### Constructors

```go
// For cmp.Ordered keys (int, string, float64, ...)
t, err := fractaltree.New[string, int](
    fractaltree.WithEpsilon(0.5),   // default
    fractaltree.WithBlockSize(4096), // default
)

// For composite or custom-ordered keys
t, err := fractaltree.NewWithCompare[MyKey, MyVal](compareFn)

// With disk persistence
t, err := fractaltree.NewDisk[string, int](flusher,
    fractaltree.WithKeyCodec[string, int](myCodec),
    fractaltree.WithValueCodec[string, int](myCodec),
)
```

### Core Operations

```go
t.Put(key, value)               // Insert or overwrite
v, ok := t.Get(key)             // Point lookup
ok := t.Contains(key)           // Existence check
deleted := t.Delete(key)        // Remove single key
n := t.DeleteRange(lo, hi)      // Remove keys in [lo, hi)
t.Clear()                       // Remove all keys
n := t.Len()                    // Key count
t.Close()                       // Release resources
```

### Upsert

```go
// Read-modify-write with a custom function
t.Upsert("counter", func(existing *int, exists bool) int {
    if exists {
        return *existing + 1
    }
    return 1
})

// Built-in helpers
t.Upsert("hits", fractaltree.Increment(1))
t.Upsert("flag", fractaltree.CompareAndSwap(oldVal, newVal))
inserted := t.PutIfAbsent("key", value)
```

### Iteration

All iterators use Go 1.23+ `range`-over-func (`iter.Seq2[K, V]`):

```go
for k, v := range t.All()                { ... }  // ascending
for k, v := range t.Ascend()             { ... }  // same as All
for k, v := range t.Descend()            { ... }  // descending
for k, v := range t.Range(lo, hi)        { ... }  // [lo, hi) ascending
for k, v := range t.DescendRange(hi, lo) { ... }  // (lo, hi] descending
```

### Cursor

For stateful traversal with seek support:

```go
c := t.Cursor()
defer c.Close()

c.Seek(42)               // position at first key >= 42
for c.Valid() {
    fmt.Println(c.Key(), c.Value())
    c.Next()
}
```

### Disk Persistence

Implement the `Flusher` interface to control where nodes are stored:

```go
type Flusher[K, V any] interface {
    WriteNode(id uint64, data []byte) error
    ReadNode(id uint64) ([]byte, error)
    Sync() error
    Close() error
}
```

Key and value serialization uses the `Codec` interface:

```go
type Codec[T any] interface {
    Encode(T) ([]byte, error)
    Decode([]byte) (T, error)
}
```

The default `GobCodec` works out of the box. See the `examples/disktree-*` directories for custom binary encodings.

---

## Concurrency Model

`BETree` uses a single `sync.RWMutex`:

| Operation | Lock | Blocks writers? | Blocks readers? |
|:----------|:-----|:----------------|:----------------|
| Get, Contains, Len | RLock | No | No |
| Put, Delete, Upsert, Clear, DeleteRange | Lock | Yes | Yes |
| All, Range, Cursor (materialization) | RLock | No | No |
| Iterator/Cursor use (after materialization) | None | No | No |

Iterators and cursors use **snapshot semantics**: they materialize a consistent view of the data under a read lock, then release the lock immediately. The snapshot is iterated without holding any lock, so writers are not blocked during traversal.

---

## Tuning

| Parameter | Option | Default | Effect |
|:----------|:-------|:--------|:-------|
| Block size | `WithBlockSize(B)` | 4096 | Controls leaf capacity and derived fanout/buffer size |
| Epsilon | `WithEpsilon(e)` | 0.5 | Tradeoff between write throughput and read latency |

**Rules of thumb:**

- For **write-heavy** workloads (logging, counters, bulk ingest): lower &epsilon; (0.3 &ndash; 0.5) gives larger buffers and fewer flushes.
- For **read-heavy** workloads (lookups, range scans): higher &epsilon; (0.5 &ndash; 0.8) gives more fanout and shallower trees.
- For **mixed** workloads: the default (&epsilon; = 0.5, B = 4096) is a good starting point.
- Larger block sizes improve throughput but increase memory per node.

---

## Performance

All benchmarks on Apple M2 Max, 32 GB RAM, Go 1.26.2. Full results, Google BTree comparison, and reproduction instructions in **[docs/PERFORMANCE.md](docs/PERFORMANCE.md)**.

| Workload | Time | Allocs | Notes |
|:---------|-----:|-------:|:------|
| Put/Sequential/1M | 114 ms | 7,980 | Append fast-path |
| Put/Random/1M | 1,344 ms | 4,849 | Batch merge |
| Get/Hit (100K) | 15.1 ms | 100,000 | Sorted buffer binary search |
| Delete (100K) | 17.0 ms | 100,006 | Buffered deletes |
| Range/10K (100K tree) | 383 &micro;s | 10,023 | Collect-and-merge iterator |
| Mixed read-heavy (100K) | 34.8 ms | 80,000 | 80% reads, 20% writes |
| Mixed write-heavy (100K) | 83.6 ms | 20,000 | 80% writes, 20% reads |

### vs Google BTree (highlights)

The B&epsilon;-tree and B-tree have different design points. FractalTree wins on sequential writes and point reads; Google BTree wins on random in-memory writes and range scans.

| Benchmark | FractalTree | Google BTree | Design point |
|:----------|------------:|-------------:|:-------------|
| Put/Sequential/100K | **8.9 ms** | 9.7 ms | B&epsilon;-tree (0.92x) |
| Put/Random/100K | 60.6 ms | 23.2 ms | B-tree (2.62x) |
| Get/Hit (100K) | **15.5 ms** | 21.7 ms | Neutral (0.71x) |
| Get/Miss (100K) | **5.1 ms** | 7.4 ms | Neutral (0.69x) |
| Range/10K (100K tree) | 412 &micro;s | 58 &micro;s | B-tree (7.1x) |

FractalTree achieves **99.6% fewer allocations** on write operations. See [docs/PERFORMANCE.md](docs/PERFORMANCE.md) for the full comparison with reclassified design-point analysis.

---

## Examples

The `examples/` directory contains 15 runnable programs:

| Example | Description |
|:--------|:------------|
| [`basic`](examples/basic) | Put, Get, Delete, Contains, Len, Clear |
| [`comparator`](examples/comparator) | Custom composite key with `NewWithCompare` |
| [`range`](examples/range) | Range queries, Descend, Cursor with Seek |
| [`concurrent`](examples/concurrent) | Multi-goroutine reads and writes |
| [`upsert`](examples/upsert) | Upsert, PutIfAbsent, Increment, CompareAndSwap |
| [`disktree-gob`](examples/disktree-gob) | DiskBETree with default GobCodec |
| [`disktree-binary`](examples/disktree-binary) | DiskBETree with `encoding/binary` codec |
| [`disktree-varint`](examples/disktree-varint) | DiskBETree with hand-rolled varint codec |
| [`leaderboard`](examples/leaderboard) | Ranked leaderboard with top-N and rank lookup |
| [`timeseries`](examples/timeseries) | Time-series ingestion with range queries |
| [`ratelimiter`](examples/ratelimiter) | Sliding-window rate limiter |
| [`eviction`](examples/eviction) | LRU-style eviction by timestamp |
| [`bulkimport`](examples/bulkimport) | Bulk ingest of 1M records |
| [`mergejoin`](examples/mergejoin) | Merge join and anti-join using two cursors |
| [`prefixscan`](examples/prefixscan) | Prefix scan using Range with ASCII ordering |

Run any example:

```bash
go run ./examples/basic
```

Run all examples:

```bash
make examples
```

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the development workflow, testing practices, and benchmark procedures.

Design decisions are documented in [`docs/ADR.md`](docs/ADR.md) — read the relevant records before modifying the flush or size-tracking paths.

---

## License

[Apache 2.0](LICENSE)
