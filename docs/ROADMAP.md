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
- [x] `examples/` — 15 runnable programs
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

Replaced `getFromNode` (full root-to-leaf traversal with O(bufferCap) linear buffer scan per level) with `existsInLeaf` (leaf-only binary search, O(depth x log(fanout) + log(leafCap))) in the write path. A `pendingDeletes` counter triggers fallback to the full check only when buffered deletes are in flight. Added `counted` flag to `Message` for size correction in `applyToLeaf`.

| Benchmark | Before | After | Delta |
|:----------|-------:|------:|------:|
| Put/Sequential/1M | 198ms | 143ms | **-28%** |
| Put/Random/1M | 3.34s | 1.85s | **-45%** |

### P1: Reuse flush bucket allocations (v0.2.1)

Added reusable `flushBuckets` field to internal nodes, reset via `[:0]` each flush. Replaced `remaining` slice allocation with in-place buffer compaction.

| Benchmark | Before | After | Delta |
|:----------|-------:|------:|------:|
| Put/Random/1M | 1.70s | 1.08s | **-36%** |
| Put/Random/1M B/op | 5.5Gi | 55Mi | **-99%** |
| Put/Random/1M allocs | 11.2M | 4.8K | **-99.96%** |
| Mixed B/op | 89Mi | 21B | **-100%** |

### Cumulative improvement (v0.1.0 -> v0.2.1)

| Benchmark | v0.1.0 | v0.2.1 | Delta |
|:----------|-------:|-------:|------:|
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

| Benchmark | v0.2.1 | v0.3.0 | Delta |
|:----------|-------:|-------:|------:|
| Put/Sequential/10K | 595us | 464us | **-22%** |
| Put/Sequential/100K | 8.76ms | 6.52ms | **-26%** |
| Put/Sequential/1M | 105ms | 84ms | **-20%** |
| Put/Random/10K | 4.13ms | 2.76ms | **-33%** |
| Put/Random/100K | 70.8ms | 49.9ms | **-30%** |
| Put/Random/1M | 1127ms | 864ms | **-23%** |
| Delete | 39.2ms | 18.3ms | **-53%** |
| Mixed | 34.7ms | 37.3ms | +7.5% |
| geomean | — | — | **-13.9%** |

### Cumulative improvement (v0.1.0 -> v0.3.0)

| Benchmark | v0.1.0 | v0.3.0 | Delta |
|:----------|-------:|-------:|------:|
| Put/Sequential/1M | 204ms | 84ms | **-59%** |
| Put/Random/1M | 3.35s | 864ms | **-74%** |
| Put/Random/1M allocs | 11.5M | 4.9K | **-99.96%** |
| Delete | 58.5ms | 18.3ms | **-69%** |
| Mixed | 58.5ms | 37.3ms | **-36%** |

---

## Research (A1-A4, completed)

All four items where the implementation deviated from Brodal & Fagerberg (SODA 2003), Bender et al. (SPAA 2007), and Kuszmaul (Tokutek 2014) have been resolved. Full details in the v0.3.0 section and prior release notes. Summary: A1 (streaming range iterator), A2 (sorted buffer with binary search), A3 (buffered upsert messages), A4 (buffered delete-range messages).

Post-A1-A4, write benchmarks regressed +20-56% due to A2's sorted insertion (O(B) memmove per insert). Attempting to fix this by setting buffer=B=4096 without changing the insertion strategy caused a +249% regression. Full root cause analysis in [`docs/RCA.md`](RCA.md).

---

## Next: Buffer Architecture Overhaul (v0.4.0)

Informed by research into PerconaFT (production), oscarlab/Be-Tree (reference), and Google BTree (Go baseline). Full analysis in [`docs/RCA.md`](RCA.md).

### The problem

Our buffer uses sorted insertion via `slices.Insert` — O(B) memmove of full `Message` structs per insert. At `bufferCap=64`, `runtime.memmove` is already 16% of CPU. The papers call for buffer capacity O(B) = 4096, but increasing buffer size with sorted insertion causes memmove to dominate: 4096 x 48 bytes = 192 KB moved per insert. No production or reference implementation uses this approach.

### What production implementations do

| Implementation | Buffer data | Buffer index | Insert cost | Memmove per insert |
|:---------------|:------------|:-------------|:------------|:-------------------|
| PerconaFT | Unsorted byte array | OMT (sorted int32 offsets) | O(1) append + O(log n) index | ~16 KB (int32, n=4096) |
| Be-Tree | `std::map` (RB tree) | Integrated | O(log n) | 0 |
| Google BTree | Sorted `[]T` slice | Integrated | O(n) memmove | ~3 KB (n bounded at ~63) |
| **Our code** | **Sorted `[]Message` slice** | **Integrated** | **O(B) memmove** | **192 KB (B=4096)** |

Google BTree gets away with sorted slices because node size is bounded at ~63. PerconaFT separates data from index — the byte buffer is append-only, the OMT index is sorted but stores 4-byte offsets, not full messages. Be-Tree uses `std::map`, which is O(log n) everything with zero memmove.

### P3: Unsorted append + lazy sort

**The change:** Two modifications, done together.

1. **Set `bufferCap = blockSize` (B = 4096).** This gives the greedy flush a minimum batch of B / B^e = B^{1-e} = 64 messages per flush, matching the amortized bound.

2. **Switch `appendToBuffer` to O(1) unsorted append.** Mark buffer as unsorted. Sort lazily: call `sortBuffer` at the start of `bufferMessagesForKey`, `bufferSlice`, and `flushNode` — only when someone reads or flushes. Writes never pay sort cost.

**Why lazy sort, not a sorted index:** Lazy sort is the minimum viable change. The O(B log B) sort at flush time is amortized across B inserts, giving O(log B) per insert — matching the theoretical bound. Go's `slices.Sort` is highly optimized (pattern-defeating quicksort). If profiling later shows flush-time sort is a bottleneck, upgrade to a sorted int index (P3b below).

**Why not a custom sorted collection:** A linked list has O(n) search (no binary search), poor cache locality, and GC pressure — worse in every dimension. A custom BST (OMT-style) adds significant complexity for a benefit that may not materialize. Start simple, profile, escalate if needed.

**Expected outcome:**
- Write path: O(1) append into a size-B buffer. No memmove.
- Read path: First read after writes pays O(B log B) sort, then binary search as before.
- Flush path: O(B log B) sort + O(B) partition. Amortized across B^{1-e} flushes.
- Write benchmarks should recover the +20-56% regression from A2 and potentially improve beyond v0.3.0 due to larger buffer enabling better batching.

### P3b: Sorted int index (contingency)

If P3 profiling shows that the lazy sort cost at flush time is a bottleneck (e.g., hot reads after interleaved writes paying repeated O(B log B) sorts), upgrade the buffer to a two-layer design:

1. **Data layer:** Unsorted `[]Message` — append-only, O(1) amortized.
2. **Index layer:** Sorted `[]int` of indices into the data slice — binary search for O(log n) lookup, `slices.Insert` for O(n) memmove on insert.

Memmove cost on `[]int`: 4096 x 8 = 32 KB vs 192 KB for `[]Message`. A 6x reduction. The PerconaFT OMT does the same with `int32_t` (4 bytes, 12x reduction), but Go's `int` is 8 bytes.

This is a medium-complexity change that preserves O(log B) reads at all times without the deferred sort penalty.

### P4: Drop epsilon as a runtime parameter

No production or reference implementation uses epsilon. Neither PerconaFT nor the paper authors' own Be-Tree code. Our code is the only one that derives fanout and buffer capacity from epsilon.

**The change:** Replace `WithEpsilon(eps)` with `WithFanout(f)` and `WithBufferCap(cap)` (or keep `WithBlockSize` and derive `bufferCap = blockSize`). Remove epsilon from `deriveParams`. Keep `WithBlockSize` for leaf capacity.

This aligns with every surveyed implementation and makes the parameters self-explanatory instead of requiring users to understand a theoretical knob that no one else uses.

### P5: Add min_flush_size

Be-Tree uses `min_flush_size = max_node_size / 16` to prevent degenerate flushes. If no child has enough messages to justify a flush, the node splits instead. Our code lacks this — with buffer=fanout=64, we can flush 1 message to the heaviest child.

**The change:** Add a `minFlushSize` parameter (default: `bufferCap / fanout`). In `flushNode`, if the heaviest child's bucket has fewer messages than `minFlushSize`, split the node instead of flushing. This matches Be-Tree's approach and prevents degenerate single-message flushes.

---

## Performance Optimizations (B-series)

### B2: Reduce `splitLeafChild` allocations

`splitLeafChild` allocates new key/value slices for the right sibling. This is 13.6% of alloc_space (75 MB in profile). Options: pre-allocate leaf backing arrays, arena/slab allocator, or copy-on-write leaves. Moderate impact — splits are unavoidable, this only reduces per-split cost.

### B3: Avoid `defer` in hot-path methods

`Put`, `Get`, `Len` use `defer t.mu.Unlock()`. While cheap (~30ns), it prevents inlining. Minor impact, low priority.

---

## Benchmark Improvements (C-series)

### C2: Batch insert benchmark

Measure amortized cost of inserting N pre-generated keys. The B^e-tree's amortized write bound should dominate for large batches.

### C3: Sequential-insert-then-random-read benchmark

Common append-log-then-query pattern. Should be the B^e-tree's sweet spot.

### C4: Upsert/increment-heavy benchmark

Counter and accumulator workloads. Should show the B^e-tree's advantage over B-trees (which must read-modify-write).

### C6: Write amplification measurement

The research (Kuszmaul 2014) frames the comparison in terms of write, read, and space amplification. Adding a write-amplification counter would directly validate the theoretical advantage.

---

## Feature Work: Toward Production

The goal is a production-grade fractal tree in Go with two operation modes: in-memory (`tree.go`, current) and disk-persistent (`disktree.go`, scaffolded).

### F1: Real disk persistence via `Flusher`

`DiskBETree` currently wraps `BETree` with lifecycle hooks but does not persist nodes to disk. Implement a file-backed `Flusher` that:
- Assigns stable node IDs
- Serializes nodes via `Codec` on flush
- Lazily loads nodes from disk on read (LRU cache of hot nodes)
- Supports `Sync` via `fsync`

PerconaFT uses a 4 MB node size and 128 KB basement nodes for disk. Our in-memory tree uses message-count-based capacity. The disk tree should switch to byte-budget capacity (like PerconaFT's `nodesize`) since serialized size matters more than message count for I/O.

### F2: WAL (Write-Ahead Log)

Write messages to a WAL before buffering in memory. On recovery, replay the WAL. Pairs with F1.

### F3: Snapshots and MVCC

Point-in-time snapshots for consistent reads without blocking writers. Kuszmaul (2014) lists MVCC as a key practical feature of TokuDB. PerconaFT implements this via fresh/stale message tracking in OMTs.

### F4: Bulk loading

Bottom-up construction from pre-sorted input without the buffer/flush path. `BulkLoad(iter.Seq2[K,V])`.

### F5: Merge iterator

`iter.Seq2` that merges multiple trees in sorted order for distributed/sharded setups.

### F6: Configurable flush strategy

Currently greedy (heaviest child). Alternatives: round-robin, threshold-based, full flush. Expose as `WithFlushStrategy`. Only greedy preserves the theoretical amortized bound.

### F7: Compression

Leaf and buffer compression (snappy/zstd) for disk-backed mode and large in-memory trees. Kuszmaul (2014): "compression works very well for FT indexes."

---

## Priority Order

| Priority | Item | Category | Status |
|:---------|:-----|:---------|:-------|
| ~~1~~ | ~~A1-A4: Research gaps~~ | Research | **Done** |
| ~~2~~ | ~~C1+C5: Write-heavy benchmarks + reclassify comparison~~ | Benchmarks | **Done** |
| ~~3~~ | ~~P0-P2: Buffer scans, allocation reuse, batch merge~~ | Performance | **Done** |
| ~~4~~ | ~~P3: Unsorted append + lazy sort~~ | Architecture | **Done** |
| **5** | **P4: Drop epsilon** | Architecture | Open |
| **6** | **P5: Add min_flush_size** | Architecture | Open |
| 7 | C6: Write amplification measurement | Benchmarks | Open |
| 8 | B2: Reduce splitLeafChild allocations | Performance | Open |
| 9 | F1: Disk persistence | Feature | Open |
| 10 | F2: WAL | Feature | Open |
| 11 | F3: MVCC | Feature | Open |

---

## Benchmarking Infrastructure

A benchmark baseline is tracked in `benchmarks/baseline.txt`. Use `benchstat` to compare against new runs:

```bash
make bench                                        # generates bench.out
benchstat benchmarks/baseline.txt bench.out        # compare
cp bench.out benchmarks/baseline.txt               # update baseline after optimization
```

Update the baseline after each release or significant optimization.
