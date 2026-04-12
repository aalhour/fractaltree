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

## Performance Optimizations (Remaining)

### P3: Index the message buffer for faster lookups

**Problem.** `getFromNode` scans the buffer linearly backwards (`tree.go:188`). For buffer capacity 64 (default), this is up to 64 comparisons per internal node per read. With a 3-level tree, that's ~192 comparisons per `Get`. `cmp.Compare` is the **#2 CPU cost at 10.0%**, called from both `findChildIndex` and `leafSearch` — buffer indexing would reduce the `findChildIndex` calls during read-path buffer scans.

**Location.** `tree.go:188` — `for i := len(n.buffer) - 1; i >= 0; i--`

**Options.**
1. **Sorted buffer + binary search.** Maintain buffer in sorted order by key. Insertion becomes O(log B) instead of O(1), but lookups drop from O(B) to O(log B). Net win if reads outnumber flushes.
2. **Per-node key index.** A `map[K]int` mapping keys to their latest buffer position. O(1) lookup, but map overhead and GC load from pointers.
3. **Keep unsorted, batch sort on flush.** Current approach. Acceptable if reads are infrequent relative to writes.

**Impact.** Faster `Get`, `Contains`, and the existence check in `putLocked` when `pendingDeletes > 0`. Most impactful for read-heavy or mixed workloads.

### P4: Avoid `defer` in hot-path methods

**Problem.** `Put`, `Get`, `Len` use `defer t.mu.Unlock()` which prevents the compiler from inlining the outer function. While `defer` is cheap (~30ns on modern Go), inlining the caller could enable further optimizations.

**Location.** `tree.go:103-104`, `tree.go:168-169`, `tree.go:243-244`

**Option.** Replace `defer` with explicit unlock before each return. Adds maintenance risk (missed unlocks on new code paths), so only worth it if profiling shows the defer overhead is material relative to the operation cost.

**Impact.** Minor. Only measurable for very small trees where the lock/unlock dominates.

---

## Feature Work

### F1: Buffered Upsert messages

Currently, `Upsert` does an eager read-modify-write (read current value, apply function, put result). A true B-epsilon-tree buffers `MsgUpsert` messages and resolves them during flush. This would make `Upsert` as fast as `Put` — no read required.

**Complexity.** Moderate. Requires storing `UpsertFn` in the message, chaining multiple upsert messages during resolution, and handling the absent-key case at apply time.

### F2: Buffered DeleteRange messages

Currently, `DeleteRange` eagerly collects candidate keys and deletes them one by one. A buffered `MsgDeleteRange` would be inserted into the root buffer and resolved during flush, making range deletion O(1) at insertion time.

**Complexity.** Moderate. Requires range-aware message resolution during flush and read path (checking whether a key falls within any pending delete range).

### F3: Real disk persistence via `Flusher`

`DiskBETree` currently wraps `BETree` with lifecycle hooks but does not actually persist nodes to disk. Implement a file-backed `Flusher` that:
- Assigns stable node IDs
- Serializes nodes via `Codec` on flush
- Lazily loads nodes from disk on read (LRU cache of hot nodes)
- Supports `Sync` via `fsync`

### F4: WAL (Write-Ahead Log)

For crash recovery, write messages to a WAL before buffering them in memory. On recovery, replay the WAL to reconstruct the in-memory state. Pairs with F3.

### F5: Snapshots and MVCC

Support point-in-time snapshots for consistent reads without blocking writers. This enables MVCC (multi-version concurrency control) for database use cases.

### F6: Bulk loading

Specialized constructor that accepts pre-sorted input and builds the tree bottom-up without going through the buffer/flush path. Would make `BulkLoad(iter.Seq2[K,V])` significantly faster than sequential `Put` for initial data loading.

### F7: Merge iterator

An `iter.Seq2` that merges multiple trees in sorted order, useful for distributed or sharded setups. The `mergejoin` example demonstrates the pattern with cursors; a dedicated merge iterator would be more ergonomic.

### F8: Configurable flush strategy

Currently, flush always picks the heaviest child (greedy). Alternative strategies:
- **Round-robin** — fairer distribution, avoids hot-child starvation
- **Threshold-based** — only flush children whose bucket exceeds a minimum size
- **Full flush** — push to all children at once (simpler, higher write amplification)

Expose as a `WithFlushStrategy` option.

---

## Benchmarking Infrastructure

A benchmark baseline is tracked in `benchmarks/baseline.txt`. Use `benchstat` to compare against new runs:

```bash
make bench                                        # generates bench.out
benchstat benchmarks/baseline.txt bench.out        # compare
cp bench.out benchmarks/baseline.txt               # update baseline after optimization
```

Update the baseline after each release or significant optimization.
