# Roadmap

---

## Completed (v0.1.0 — 2026-04-12)

### Phase 1: Foundation

- [x] `errors.go` — sentinel errors (`ErrClosed`, `ErrInvalidEpsilon`, `ErrNilCompare`, `ErrInvalidBlockSize`)
- [x] `message.go` — `MsgKind` enum + `Message[K,V]` struct
- [x] `options.go` — `Option` funcs (`WithEpsilon`, `WithBlockSize`) + `deriveParams`
- [x] `node.go` — `node[K,V]` struct, `newLeaf`, `findChildIndex`, `leafSearch`, `leafInsert`, `leafDelete`
- [x] `flush.go` — `flushNode`, `applyToLeaf`, `splitChild`, `splitLeafChild`, `splitInternalChild`, `splitRoot`

### Phase 2: Core Tree

- [x] `tree.go` — `BETree` struct, `New`, `NewWithCompare`, `Len`, `Clear`, `Close`
- [x] `tree.go` — `Get`, `Contains` (root-to-leaf message collection with buffer scan)
- [x] `tree.go` — `Put`, `Delete` (message insertion + flush orchestration)
- [x] `tree.go` — `DeleteRange` (candidate collection, dedup, batch delete)
- [x] `tree_test.go` — 30+ integration tests (happy path, flush, splits, edge cases, ordering)

### Phase 3: Advanced Operations

- [x] `upsert.go` — `UpsertFn[V]`, `Increment`, `CompareAndSwap`
- [x] `tree.go` — `Upsert` (eager read-modify-write), `PutIfAbsent`
- [x] `upsert_test.go` — 14 tests
- [x] `iterator.go` — snapshot-based `All`, `Ascend`, `Descend`, `Range`, `DescendRange` via `iter.Seq2`
- [x] `iterator_test.go` — 22 tests
- [x] `cursor.go` — `Cursor[K,V]` with `Next`, `Prev`, `Seek`, `Key`, `Value`, `Valid`, `Close`
- [x] `cursor_test.go` — 16 tests

### Phase 4: Disk Hooks

- [x] `codec.go` — `Codec[T]` interface + `GobCodec[T]`
- [x] `codec_test.go` — 6 tests
- [x] `disktree.go` — `Flusher[K,V]` interface, `DiskOption`, `DiskBETree` (delegation wrapper)
- [x] `disktree_test.go` — 16 tests (with mock `memFlusher`)

### Phase 5: Documentation and Examples

- [x] `doc.go` — comprehensive package docs for pkg.go.dev
- [x] `example_test.go` — 27 `Example*` functions covering all public APIs
- [x] `examples/` — 15 runnable programs:
  - `basic`, `comparator`, `range`, `concurrent`, `upsert`
  - `disktree-gob`, `disktree-binary`, `disktree-varint`
  - `leaderboard`, `timeseries`, `ratelimiter`, `eviction`, `bulkimport`, `mergejoin`, `prefixscan`
- [x] `README.md` — badges, background, API overview, benchmarks, comparison vs Google BTree
- [x] `AGENTS.md` — contributor guide with architecture, test organization, Makefile reference

### Phase 6: Quality

- [x] `fuzz_test.go` — `FuzzOperations` (random ops vs reference map), `FuzzRange` (range queries)
- [x] `bench_test.go` — 6 benchmarks (Put, Get, Delete, Range, Mixed, Upsert)
- [x] `stress_test.go` — 4 concurrent stress tests (read/write, range+write, increment, delete-range)
- [x] `Makefile` — 25 targets with tier system (`quick`, `long`, `marathon`)
- [x] `.golangci.yml` — 27 linters, no exclusions
- [x] `.github/workflows/` — CI pipeline (test, lint, race)

### Phase 7: Release

- [x] `.gitignore` — profiling artifacts, benchmark output, fuzz cache
- [x] `go.mod` pinned to `go 1.26`
- [x] Tagged `v0.1.0`, GitHub release created
- [x] Module indexed on pkg.go.dev

### v0.1.0 Metrics

| Metric | Value |
|:-------|:------|
| Library test coverage | 97.8% |
| Total test functions | 159 |
| Lines of Go code | ~5,200 |
| Runtime dependencies | 0 |
| Lint issues | 0 (27 linters) |
| Race detector | Clean |
| Fuzz testing | 1.6M+ executions, 0 failures |

---

## Completed (v0.2.0–v0.2.1 — 2026-04-12)

Informed by profiling v0.1.0 on Apple M2 Max / Go 1.26.

### P0: Eliminate buffer scans in `putLocked` (v0.2.0)

Replaced `getFromNode` (full root-to-leaf traversal with O(bufferCap) linear buffer scan per level) with `existsInLeaf` (leaf-only binary search, O(depth × log(fanout) + log(leafCap))) in the write path. A `pendingDeletes` counter triggers fallback to the full check only when buffered deletes are in flight. Added `counted` flag to `Message` for size correction in `applyToLeaf`.

| Benchmark | Before | After | Δ |
|:----------|-------:|------:|--:|
| Put/Sequential/1M | 198ms | 143ms | **-28%** |
| Put/Random/1M | 3.34s | 1.85s | **-45%** |

### P1: Reuse flush bucket allocations (v0.2.1)

Added reusable `flushBuckets` field to internal nodes, reset via `[:0]` each flush. Replaced `remaining` slice allocation with in-place buffer compaction.

| Benchmark | Before | After | Δ |
|:----------|-------:|------:|--:|
| Put/Random/1M | 1.70s | 1.08s | **-36%** |
| Put/Random/1M B/op | 5.5Gi | 55Mi | **-99%** |
| Put/Random/1M allocs | 11.2M | 4.8K | **-99.96%** |
| Mixed B/op | 89Mi | 21B | **-100%** |

### Cumulative improvement (v0.1.0 → v0.2.1)

| Benchmark | v0.1.0 | v0.2.1 | Δ |
|:----------|-------:|-------:|--:|
| Put/Sequential/1M | 204ms | 106ms | **-48%** |
| Put/Random/1M | 3.35s | 1.08s | **-68%** |
| Put/Random/1M allocs | 11.5M | 4.8K | **-99.96%** |
| Mixed | 58.5ms | 35ms | **-40%** |

---

## Completed (v0.3.0 — 2026-04-12)

Informed by profiling v0.2.1 on Apple M2 Max / Go 1.26. Workload: `Put/Random/100K` (count=6).

### Pre-P2 profile (top offenders)

**CPU:**

| Rank | Function | Flat | Cum | % of Total |
|:-----|:---------|-----:|----:|:-----------|
| 1 | `runtime.memmove` (via `slices.Insert`) | 690ms | 690ms | **16.0%** |
| 2 | `cmp.Compare` | 430ms | 440ms | **10.0%** |
| 3 | `findChildIndex` | 360ms | 560ms | **8.4%** |
| 4 | `leafSearch` | 160ms | 340ms | **3.7%** |
| 5 | `flushNode` (self) | 200ms | 1.63s | **4.7%** |
| 6 | `existsInLeaf` | 30ms | 490ms | 0.7% flat, 11.4% cum |

**Memory (alloc_space):**

| Rank | Function | Alloc | % of Total |
|:-----|:---------|------:|:-----------|
| 1 | `slices.Insert` (via `leafInsert`) | 200.8 MB | **53.8%** |
| 2 | `collectCandidateKeys` | 49.7 MB | 13.3% |
| 3 | `splitLeafChild` | 42.2 MB | 11.3% |
| 4 | `materializeRange` | 34.6 MB | 9.3% |
| 5 | `flushNode` (self) | 9.1 MB | 2.4% |

### P2: Batch merge in `applyToLeaf`

Replaced per-message `leafInsert` (N individual `slices.Insert` memmoves) with three specialized merge paths in `applyToLeaf`:

1. **Append fast-path** (`appendToLeaf`): when all messages sort after the last leaf key (sequential inserts), skip binary search entirely and append in O(N). Zero allocation when capacity suffices.
2. **Binary-search + chunk-copy** (`mergeLeafPuts`): for puts-only batches, process messages largest-first. Each message does O(log L) binary search, then `copy()` shifts the chunk between insertion points. Total: O(N log L) comparisons + O(L) memmove — same comparisons as N individual inserts but 1 memmove pass instead of N. In-place reverse merge (grow slice, merge from tail) avoids allocation.
3. **In-place compaction + merge** (`mergeLeafWithDeletes`): for batches containing deletes, phase 1 walks leaf and messages left-to-right applying deletes and overwrites in-place (zero allocation); phase 2 inserts any new keys via `mergeLeafPuts`.

Supporting optimizations:
- `resolveMessages` skips sort when input is already sorted with no duplicates (common case: each buffered key appears once).
- Small batches (≤3 messages) still use per-message `leafInsert` to avoid sort+merge overhead.

| Benchmark | v0.2.1 | v0.3.0 | Δ |
|:----------|-------:|-------:|--:|
| Put/Sequential/10K | 595µs | 464µs | **-22%** |
| Put/Sequential/100K | 8.76ms | 6.52ms | **-26%** |
| Put/Sequential/1M | 105ms | 84ms | **-20%** |
| Put/Random/10K | 4.13ms | 2.76ms | **-33%** |
| Put/Random/100K | 70.8ms | 49.9ms | **-30%** |
| Put/Random/1M | 1127ms | 864ms | **-23%** |
| Delete | 39.2ms | 18.3ms | **-53%** |
| Mixed | 34.7ms | 37.3ms | +7.5% |
| geomean | — | — | **-13.9%** |

The Mixed regression (+7.5%) is a trade-off: the batch merge adds sort+dedup overhead to the write path that is not offset by memmove savings when all writes are overwrites (existing keys). The 80% read portion of Mixed is unaffected. All other benchmarks improved or held steady. Zero allocation regressions: Delete B/op and allocs/op are unchanged from v0.2.1.

### Cumulative improvement (v0.1.0 → v0.3.0)

| Benchmark | v0.1.0 | v0.3.0 | Δ |
|:----------|-------:|-------:|--:|
| Put/Sequential/1M | 204ms | 84ms | **-59%** |
| Put/Random/1M | 3.35s | 864ms | **-74%** |
| Put/Random/1M allocs | 11.5M | 4.9K | **-99.96%** |
| Delete | 58.5ms | 18.3ms | **-69%** |
| Mixed | 58.5ms | 37.3ms | **-36%** |

---

## Category A: Gaps from the Research (all completed)

Items where the current implementation deviated from the algorithms described in Brodal & Fagerberg (SODA 2003), Bender et al. (SPAA 2007), and Kuszmaul (Tokutek 2014). All four items have been implemented.

### A1: Streaming range iterator (algorithmic correctness) &mdash; Done

Replaced `materializeRange` with a collect-and-merge approach. `collectRange` descends the tree collecting buffer messages and leaf entries as `mergeEntry` structs with depth information. `resolveEntries` sorts by (key, depth) and resolves same-key groups across depth levels, handling Put/Delete/Upsert interactions. Range scan gap vs Google BTree narrowed from 27&ndash;1,843x to 5&ndash;7x.

### A2: Sorted buffer with binary search on reads &mdash; Done

Buffers are now maintained in sorted order. `appendToBuffer` inserts at the correct sorted position (O(log B) binary search + O(B) memmove). `bufferMessagesForKey` uses binary search for O(log B) lookups. `bufferSlice` extracts range sub-slices via binary search for efficient range iteration.

### A3: Buffered Upsert messages &mdash; Done

Added `MsgUpsert` message kind carrying `UpsertFn`. Upserts are buffered as messages and resolved lazily during flush via `applyToLeaf`. `getWithUpserts` collects upsert functions from buffers on the root-to-leaf path and applies them as a chain. Handles: multiple upserts on same key, upsert after delete, upsert after put.

### A4: Buffered DeleteRange messages &mdash; Done

After exploring true `MsgDeleteRange` buffering (fan-out to children, sequence-number ordering), discovered a fundamental correctness issue: leaf values lack sequence numbers, making it impossible to resolve range deletes against leaf values after partial flushes. Settled on expanding range deletes to individual `MsgDelete` entries using the iterator infrastructure at `DeleteRange` call time. This is O(K) instead of O(1) but correct and simpler than the previous approach which called `deleteLocked` per key (each doing a full `getFromNode` traversal).

---

## Category B: Performance Optimizations

Items that improve performance without changing algorithmic behavior. Informed by profiling v0.3.0 on Apple M2 Max / Go 1.26.

### B1: Eliminate materialization in range queries (depends on A1)

**Problem.** Even ignoring the algorithmic issue, `materializeRange` allocates a `[]K` candidate slice (49 MB) and a `[]kvPair` result slice (36 MB). These are the #3 and #4 memory costs.

**Fix.** Streaming iteration (A1) eliminates both allocations entirely — pairs are yielded one at a time.

### B2: Reduce `splitLeafChild` allocations

**Problem.** `splitLeafChild` allocates new key/value slices for the right sibling. This is 13.6% of alloc_space (75 MB in profile).

**Options.**
1. **Pre-allocate leaf backing arrays at node creation.** Wastes memory for mostly-empty leaves.
2. **Arena/slab allocator for leaf slices.** Amortizes allocation across many splits.
3. **Copy-on-write leaves.** Share backing array, split only on mutation. Adds complexity.

**Impact.** Moderate. Splits are unavoidable — this only reduces the per-split cost.

### B3: Avoid `defer` in hot-path methods

**Problem.** `Put`, `Get`, `Len` use `defer t.mu.Unlock()`. While cheap (~30ns), it prevents inlining.

**Location.** `tree.go:103-104`, `tree.go:168-169`, `tree.go:243-244`

**Impact.** Minor. Only measurable for very small trees. Low priority.

---

## Category C: Benchmark Improvements

### C1: Add write-heavy mixed benchmark (80% write, 20% read) &mdash; Done

Added `BenchmarkMixed/WriteHeavy` (80% writes, 20% reads) alongside the existing `BenchmarkMixed/ReadHeavy`. Added corresponding `BenchmarkCompare_MixedWriteHeavy` in the cross-implementation suite.

### C2: Add batch insert benchmark

Measure the amortized cost of inserting N pre-generated keys. The B^ε-tree's amortized write bound should dominate for large batches. Current benchmarks measure per-key insertion time, which includes flush overhead but doesn't capture the batch amortization clearly.

### C3: Add sequential-insert-then-random-read benchmark

Common pattern: append-log-then-query. This should be the B^ε-tree's sweet spot — fast sequential writes (append fast-path), followed by point reads in a fully-flushed tree.

### C4: Add upsert/increment-heavy benchmark

Counter and accumulator workloads. Once A3 (buffered upsert) is implemented, this should show the B^ε-tree's advantage over B-trees (which must read-modify-write).

### C5: Reclassify Google BTree comparison &mdash; Done

Reorganized the comparison in `docs/PERFORMANCE.md` and `README.md` into three sections by design point: **B^ε-tree design point** (sequential writes, allocation efficiency), **B-tree design point** (random in-memory writes, range scans), and **Neutral ground** (point reads, deletes, mixed workloads). Updated all benchmark numbers to reflect A1&ndash;A4 changes.

### C6: Add write amplification measurement

The research (Kuszmaul 2014) frames the comparison in terms of write amplification, read amplification, and space amplification. Current benchmarks only measure wall time and allocations. Adding a write-amplification counter (total bytes written to leaves / bytes inserted by user) would directly validate the B^ε-tree's theoretical advantage.

---

## Feature Work

### F1: Real disk persistence via `Flusher`

`DiskBETree` currently wraps `BETree` with lifecycle hooks but does not actually persist nodes to disk. Implement a file-backed `Flusher` that:
- Assigns stable node IDs
- Serializes nodes via `Codec` on flush
- Lazily loads nodes from disk on read (LRU cache of hot nodes)
- Supports `Sync` via `fsync`

### F2: WAL (Write-Ahead Log)

For crash recovery, write messages to a WAL before buffering them in memory. On recovery, replay the WAL to reconstruct the in-memory state. Pairs with F1.

### F3: Snapshots and MVCC

Support point-in-time snapshots for consistent reads without blocking writers. This enables MVCC (multi-version concurrency control) for database use cases. Kuszmaul (2014) lists MVCC as a key practical feature of TokuDB.

### F4: Bulk loading

Specialized constructor that accepts pre-sorted input and builds the tree bottom-up without going through the buffer/flush path. Would make `BulkLoad(iter.Seq2[K,V])` significantly faster than sequential `Put` for initial data loading.

### F5: Merge iterator

An `iter.Seq2` that merges multiple trees in sorted order, useful for distributed or sharded setups. The `mergejoin` example demonstrates the pattern with cursors; a dedicated merge iterator would be more ergonomic.

### F6: Configurable flush strategy

Currently, flush always picks the heaviest child (greedy), as required by the amortized complexity proof. Alternative strategies for specific use cases:
- **Round-robin** — fairer distribution, avoids hot-child starvation
- **Threshold-based** — only flush children whose bucket exceeds a minimum size
- **Full flush** — push to all children at once (simpler, higher write amplification)

Expose as a `WithFlushStrategy` option. Note: only greedy flush preserves the theoretical O(log_B N / B^(1-ε)) amortized bound.

### F7: Compression

Kuszmaul (2014): "compression works very well for FT indexes, and can dramatically reduce write amplification." Leaf and buffer compression (snappy/zstd) would reduce I/O for disk-backed mode and memory footprint for large in-memory trees.

---

## Priority Order

| Priority | Item | Category | Status |
|:---------|:-----|:---------|:-------|
| ~~1~~ | ~~A2: Sorted buffer + binary search~~ | Research gap | **Done** |
| ~~2~~ | ~~A1: Streaming range iterator~~ | Research gap | **Done** |
| ~~3~~ | ~~A3: Buffered Upsert messages~~ | Research gap | **Done** |
| ~~4~~ | ~~C1+C5: Write-heavy benchmarks + reclassify comparison~~ | Benchmarks | **Done** |
| ~~5~~ | ~~A4: Buffered DeleteRange~~ | Research gap | **Done** |
| **6** | C6: Write amplification measurement | Benchmarks | Open |
| **7** | B2: Reduce splitLeafChild allocations | Performance | Open |
| **8** | F1: Disk persistence | Feature | Open |
| **9** | F2: WAL | Feature | Open |
| **10** | F3: MVCC | Feature | Open |

All research gaps (A1&ndash;A4) and benchmark improvements (C1+C5) are complete. Remaining work is performance optimization (B2), benchmarking (C6), and feature work (F1&ndash;F7).

---

## Benchmarking Infrastructure

A benchmark baseline is tracked in `benchmarks/baseline.txt`. Use `benchstat` to compare against new runs:

```bash
make bench                                        # generates bench.out
benchstat benchmarks/baseline.txt bench.out        # compare
cp bench.out benchmarks/baseline.txt               # update baseline after optimization
```

Update the baseline after each release or significant optimization.
