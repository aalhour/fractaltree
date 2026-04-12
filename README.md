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

All benchmarks run on **Apple M2 Max, 32 GB RAM, Go 1.26.2, darwin/arm64**.

### Benchmarks

#### Write Performance (Put)

| Workload | N | Time | Allocs |
|:---------|--:|-----:|-------:|
| Sequential | 1K | 33 &micro;s | 6 |
| Sequential | 10K | 1.1 ms | 1,388 |
| Sequential | 100K | 17.5 ms | 22,556 |
| Sequential | 1M | 203 ms | 353,973 |
| Random | 1K | 120 &micro;s | 6 |
| Random | 10K | 5.5 ms | 3,204 |
| Random | 100K | 129 ms | 680,476 |
| Random | 1M | 3.35 s | 11.4M |

#### Read Performance (Get, 100K keys)

| Workload | Time / 100K ops | Allocs |
|:---------|----------------:|-------:|
| Hit (random) | 17.9 ms | 0 |
| Miss | 8.9 ms | 0 |

**Zero allocations on reads.**

#### Range Scans (100K key tree)

| Result count | Time |
|-------------:|-----:|
| 10 | 264 &micro;s |
| 100 | 276 &micro;s |
| 1K | 396 &micro;s |
| 10K | 1.57 ms |

#### Other Operations

| Operation | Time | Notes |
|:----------|-----:|:------|
| Delete (100K) | 45.6 ms | Sequential delete all keys |
| Upsert/Increment (10K) | via bench | 100 counters, 10K total increments |
| Mixed 80/20 (100K) | 58.5 ms | 80% reads, 20% writes |

### Comparison with Google BTree

Head-to-head benchmark using identical workloads, same machine, same session. Google's [`btree`](https://github.com/google/btree) v1.1.3 with degree 32.

#### Write (Put) &mdash; Sequential

| N | FractalTree | Google BTree | Ratio |
|--:|------------:|-------------:|------:|
| 1K | 33.7 &micro;s | 38.4 &micro;s | **0.88x** |
| 10K | 1.12 ms | 501 &micro;s | 2.24x |
| 100K | 18.0 ms | 6.26 ms | 2.88x |

#### Write (Put) &mdash; Random

| N | FractalTree | Google BTree | Ratio |
|--:|------------:|-------------:|------:|
| 1K | 122 &micro;s | 67.9 &micro;s | 1.80x |
| 10K | 5.57 ms | 1.06 ms | 5.27x |
| 100K | 131 ms | 14.5 ms | 9.06x |

#### Read (Get, 100K keys)

| Workload | FractalTree | Google BTree | Ratio |
|:---------|------------:|-------------:|------:|
| Hit | 18.2 ms | 13.4 ms | 1.36x |
| Miss | 9.18 ms | 4.62 ms | 1.99x |

#### Range Scan (100K key tree)

| Count | FractalTree | Google BTree | Ratio |
|------:|------------:|-------------:|------:|
| 10 | 264 &micro;s | 84 ns | 3,143x |
| 100 | 276 &micro;s | 480 ns | 576x |
| 1K | 396 &micro;s | 4.0 &micro;s | 99x |
| 10K | 1.57 ms | 41 &micro;s | 38x |

#### Mixed (80% Read, 20% Write, 100K keys)

| | FractalTree | Google BTree | Ratio |
|:---------|------------:|-------------:|------:|
| Time | 58.5 ms | 14.0 ms | 4.18x |

#### Analysis

The comparison reveals the expected tradeoff profile of a B&epsilon;-tree vs a B-tree:

- **Sequential writes at small N** are competitive &mdash; the FractalTree wins at 1K because buffer insertion is cheaper than B-tree node splitting.
- **Reads and range scans** favor the B-tree, as expected. The FractalTree must collect and resolve pending messages from buffers at every level during reads. Range scans pay a materialization cost (snapshot + sort + dedup) rather than doing an in-place tree walk.
- **Random writes at scale** show the B-tree pulling ahead because Google's btree is a highly optimized, mature implementation with cache-friendly memory layout. The amortized I/O advantage of B&epsilon;-trees is most visible in **disk-backed** scenarios where the cost of a random I/O dwarfs in-memory pointer chasing.

**When to choose FractalTree over a B-tree:**

- Your workload is **disk-backed** &mdash; the batched flush model reduces random I/O by orders of magnitude, which is where the theoretical advantage materializes.
- You need **Upsert / Increment / CompareAndSwap** semantics &mdash; B&epsilon;-trees support these natively via the message buffer.
- You need **range deletion** as a first-class operation.
- You want a **pluggable persistence layer** with codec support.
- You are building a **storage engine** and want the internal architecture of a fractal tree rather than a B-tree.

### Quality

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

## License

[MIT](LICENSE)
