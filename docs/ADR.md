# Architecture Decision Records

This document records significant design decisions in the `fractaltree` library. Each ADR captures the context, the decision, alternatives considered, and consequences — so future contributors can understand *why* the code is the way it is, not just *what* it does.

**Format.** Each record follows the structure: Context (what problem?), Decision (what we chose and how it works), Alternatives Considered (what we rejected and why), Consequences (what improved, what regressed, what invariants must hold).

---

## Index

| ID | Title | Status | Date | Applies to |
|:---|:------|:-------|:-----|:-----------|
| [ADR-001](#adr-001-batch-leaf-merge) | Batch Leaf Merge in `applyToLeaf` | Accepted | 2026-04-12 | `flush.go` (v0.3.0) |
| [ADR-002](#adr-002-optimistic-size-tracking) | Optimistic Size Tracking with Deferred Correction | Accepted | 2026-04-12 | `tree.go`, `flush.go` (v0.2.0+) |
| [ADR-003](#adr-003-greedy-flush-with-reusable-buckets) | Greedy Flush with Reusable Buckets | Accepted | 2026-04-12 | `flush.go`, `node.go` (v0.2.1) |
| [ADR-004](#adr-004-unsorted-buffer-append-with-lazy-sort) | Unsorted Buffer Append with Lazy Sort | Accepted | 2026-04-24 | `node.go`, `flush.go` (v0.4.0) |

---

## ADR-001: Batch Leaf Merge

**Status:** Accepted | **Date:** 2026-04-12 | **Applies to:** `flush.go` (v0.3.0)

### Context

In a B-epsilon-tree, writes are buffered as messages in internal nodes and flushed toward leaves in batches. When a batch reaches a leaf, the messages must be applied: puts insert or overwrite key-value pairs, deletes remove them. The function responsible for this is `applyToLeaf`.

Prior to v0.3.0, `applyToLeaf` applied each message individually via `leafInsert` / `leafDelete`. Each `leafInsert` called `slices.Insert`, which shifts all elements right of the insertion point via `memmove`. For a leaf of capacity L = 4096 (default), a mid-leaf insertion moves ~2048 elements. With N messages per flush (typically 32--64), the cost was N individual memmoves of O(L) each.

Profiling on Apple M2 Max (Go 1.26) showed this was the dominant bottleneck after P0 + P1 eliminated buffer management overhead:

- **CPU:** `runtime.memmove` via `slices.Insert` was #1 at 16.0% of total.
- **Memory:** `slices.Insert` via `leafInsert` was #1 at 53.8% of alloc_space.

The per-message path is simple and correct, but does O(N * L) total data movement per flush. A batch merge that consolidates the N memmoves into a single O(L) pass reduces data movement by a factor of N.

### Decision

Replace the per-message `leafInsert` loop with a **multi-path batch merge**. The batch is first sorted and deduplicated (`resolveMessages`), then dispatched to one of three specialized merge functions:

#### Path 1: Append fast-path (`appendToLeaf`)

**When:** All messages are puts, and `msgs[0].Key > leaf.keys[last]`. Common for sequential inserts.

**Algorithm:** `slices.Grow` + `append`. O(N) copies, zero binary searches, zero allocation when capacity suffices.

**Why a separate path:** The general merge does N binary searches even when all insertion points are at the end. A single comparison detects the append case and yields 20--26% speedup on sequential writes.

#### Path 2: Binary-search + chunk-copy (`mergeLeafPuts`)

**When:** All messages are puts, but they interleave with existing leaf keys.

**Algorithm:**

1. Grow the leaf's key/value slices by N slots.
2. Process messages largest-first (reverse order).
3. For each message, binary-search the old keys for insertion point: O(log L).
4. `copy()` the chunk of old keys between insertion points to final position.
5. Write the message's key/value at the write cursor.

Total: **O(N log L) comparisons + O(L) memmove** — same comparisons as N individual inserts but with 1 memmove pass instead of N.

**In-place reverse merge:** The write cursor `w` starts at `oldLen + N - 1`, the read cursor `r` at `oldLen - 1`. Since `w` starts N ahead of `r`, and each placed message consumes one slot, the invariant `w >= r` holds throughout — the write position never overtakes unread data.

**Overwrite handling:** When a key already exists (`found == true`), the old slot is skipped and an `overwrites` counter increments. After the loop, a single `copy()` closes the gap, followed by `clear()` and truncation.

#### Path 3: In-place compaction + merge (`mergeLeafWithDeletes`)

**When:** The batch contains at least one `MsgDelete`.

**Phase 1 — In-place compaction:** Walk leaf and messages left-to-right with cursors `li`, `mi`, `w`:

- `leaf[li] < msg[mi]`: Keep leaf key, copy to `w`.
- `leaf[li] > msg[mi]`: New-insert put — collect into `msgs[:numNew]` (reusing already-consumed prefix; `numNew <= mi` guarantees no overtake).
- `leaf[li] == msg[mi]`: If put, overwrite at `w`. If delete, skip (don't advance `w`).

**Phase 2 — Insert new keys:** Call `mergeLeafPuts(leaf, msgs[:numNew])`. After compaction, freed slots usually provide sufficient capacity — zero allocation.

#### Supporting optimizations

- **Small-batch threshold (<=3):** Batches of 1--3 messages use per-message `leafInsert`/`leafDelete` directly. Sort+merge setup overhead exceeds memmove savings at this scale.
- **Sort elision:** `resolveMessages` scans for pre-sorted input (O(N)) before calling `SortStableFunc` (O(N log N)). Common case: each buffered key appears once and the batch is already sorted.

### Alternatives Considered

| ID | Alternative | Verdict | Reason |
|:---|:-----------|:--------|:-------|
| A1 | Unsorted leaf, sort at split/read | Rejected | Penalizes every `Get`, `Contains`, and range query with O(L log L). Breaks sorted-leaf invariant. |
| A2 | Gap buffer in leaf slices | Rejected | Every leaf operation must account for gap position. Complexity spreads across entire codebase. |
| A3 | Smaller default leaf capacity (B=1024) | Rejected | Reduces constant but doesn't eliminate O(N*L) scaling. More splits, deeper trees. |
| A4 | Full linear merge O(L+N) | Rejected after bench | **+57% regression** on random writes. O(L) comparisons at ~5ns/call via function pointer exceeded the O(L) memmove savings at ~0.3ns/element. |
| A5 | Allocating merge for deletes | Rejected after bench | **+1,081,680% B/op** on Delete benchmark. 36 MB allocated per flush for new backing arrays. |

### Consequences

**Positive:**
- Random writes 23--33% faster (10K--1M keys).
- Sequential writes 20--26% faster.
- Deletes 53% faster.
- Zero allocation regressions.
- Geomean -13.9% across all benchmarks.

**Negative:**
- Mixed (80/20 read/write) regressed +7.5%. Sort+dedup overhead on overwrites is not offset by memmove savings. Confined to the 20% write portion.
- Code complexity: `applyToLeaf` went from a simple loop to a four-phase pipeline with three dispatch paths.

**Invariants to maintain:**

1. **Leaf keys always sorted** after `applyToLeaf` returns.
2. **`t.size` always correct** — size correction depends on `preCounted` and `numDeletes` being tallied *before* `resolveMessages` deduplicates.
3. **`t.pendingDeletes` decremented** for every `MsgDelete` reaching a leaf, regardless of whether the key existed.
4. **`resolveMessages` must be stable** — for duplicate keys, last message wins.
5. **`mergeLeafWithDeletes` collects new-insert puts before truncating** — the `msgs[:numNew]` reuse depends on `numNew <= mi`.

---

## ADR-002: Optimistic Size Tracking with Deferred Correction

**Status:** Accepted | **Date:** 2026-04-12 | **Applies to:** `tree.go`, `flush.go` (v0.2.0+)

### Context

`Len()` must return an accurate count of key-value pairs. In a B-epsilon-tree, the true count can only be determined by flushing all buffers to leaves — but flushing on every `Len()` call would defeat the purpose of buffering.

### Decision

Track `t.size` optimistically at message insertion time, then correct when messages reach leaves.

**Write path (`putLocked`):**

1. Check whether the key is new: `existsInLeaf` (fast, leaf-only) when no deletes are pending; `getFromNode` (full buffer scan) when `pendingDeletes > 0`.
2. If new, increment `t.size` and mark `msg.counted = true`.

**Delete path (`deleteLocked`):**

1. Check key exists via `getFromNode`.
2. If exists, decrement `t.size` and increment `t.pendingDeletes`.

**Correction (`applyToLeaf`):**

The optimistic adjustment can be wrong (duplicate puts in buffer, redundant deletes). When messages reach a leaf, the batch merge measures the actual key-count delta and corrects:

```
t.size += (newLen - oldLen) - preCounted + numDeletes
```

Where `preCounted` = puts marked as counted, `numDeletes` = delete messages in the batch. This formula undoes the optimistic delta and applies the actual delta.

### Alternatives Considered

| Alternative | Verdict | Reason |
|:-----------|:--------|:-------|
| Flush on every `Len()` | Rejected | O(N) worst case, defeats buffering. |
| Count only at leaves, walk tree for `Len()` | Rejected | O(leaves) per call. |
| Exact buffer tracking with dedup map | Rejected | Per-node maps add GC pressure and complicate flush. |

### Consequences

- `Len()` is O(1) — just read `t.size` under RLock.
- Write path pays one extra existence check per put (O(depth * log(fanout) + log(leafCap)) via `existsInLeaf`, or O(depth * bufferCap) via `getFromNode` when deletes are pending).
- Correctness depends on `applyToLeaf` always running the size correction formula, even if the batch is empty after deduplication.

---

## ADR-003: Greedy Flush with Reusable Buckets

**Status:** Accepted | **Date:** 2026-04-12 | **Applies to:** `flush.go`, `node.go` (v0.2.1)

### Context

When an internal node's buffer is full, `flushNode` partitions messages by child and pushes them down. The original implementation allocated a new `[][]Message` slice for the partition buckets on every flush. Profiling showed this as the #1 allocation source after P0.

### Decision

**Greedy child selection:** Always flush to the single child that received the most messages. This is required by the B-epsilon-tree amortized complexity proof — it guarantees each message is flushed O(log_B N) times total.

**Reusable buckets:** Each internal node carries a `flushBuckets [][]Message` field, grown lazily and reset via `[:0]` on each flush. This eliminates per-flush allocation entirely.

**In-place buffer compaction:** After flushing the heaviest bucket, unflushed messages are compacted in-place using `copy()` rather than building a new `remaining` slice.

### Alternatives Considered

| Alternative | Verdict | Reason |
|:-----------|:--------|:-------|
| Flush all children at once | Rejected | Higher write amplification. Violates the greedy invariant needed for amortized bounds. |
| Per-node `sync.Pool` for buckets | Rejected | Pool overhead and type assertions outweigh savings for this use case. |
| Global shared bucket pool | Rejected | Complicates concurrency (buckets used under tree lock, but pool would be shared). |

### Consequences

- Put/Random/1M: **-36%** wall time, **-99%** B/op, **-99.96%** allocs/op.
- The `flushBuckets` field adds one pointer per internal node. For a tree with thousands of internal nodes, this is negligible.
- Bucket slices grow monotonically (never shrunk). If a node has a transient spike in child count due to splits, the bucket slice retains the high-water mark. This is acceptable because splits increase child count by 1 and the slice holds only pointers.

---

## ADR-004: Unsorted Buffer Append with Lazy Sort

**Status:** Accepted | **Date:** 2026-04-24 | **Applies to:** `node.go`, `flush.go` (v0.4.0)

### Context

After P2 (batch leaf merge), `runtime.memmove` via `slices.Insert` in `appendToBuffer` was the #1 CPU bottleneck at 16.0% of total. Every `Put` inserted the message into the node's buffer in sorted position — O(log B) binary search + O(B) memmove per insert. At the default `bufferCap=64` (B^{1-ε}, ε=0.5), each insert memmoved ~32 × 48-byte Message structs (~1.5 KB). The write path paid this cost on every single insert, even though sorted order is only needed at flush time (for `resolveMessages`) and at read time (for `bufferMessagesForKey` / `bufferSlice`).

No production B^ε-tree uses sorted insertion into the message buffer. PerconaFT uses unsorted append into a byte array with a separate sorted index (OMT). The reference Be-Tree (oscarlab) uses `std::map`. Google BTree uses sorted slices but bounds node size at ~63 elements.

### Decision

Replace sorted insertion with **unsorted O(1) append** and **no eager sorting**. Three coordinated changes:

#### 1. `appendToBuffer`: O(1) unsorted append

The old `appendToBuffer(msg, cmp)` did binary search + `slices.Insert`. The new `appendToBuffer(msg)` does `append(n.buffer, msg)` and sets `n.bufferSorted = false`. No comparator needed — the write path never examines key order.

#### 2. Linear-scan fallback for read paths

`bufferMessagesForKey` and `bufferSlice` check `n.bufferSorted`. When sorted (rare — only after a split partitions by pivot), they use the existing binary search path. When unsorted (common), they fall back to O(B) linear scan. This is safe under `RLock` because it doesn't mutate the buffer.

The linear scan trades O(log B) reads for O(B) reads. At B=64, this is 64 comparisons vs 6 — a ~10x per-node read cost increase. Across a tree of depth ~3, the total read regression is bounded. This is acceptable for a write-optimized data structure.

#### 3. No sorting in `flushNode`

The key insight: `flushNode` does not need a sorted buffer. Message partitioning (routing each message to a child via `findChildIndex`) is per-message and order-independent. The sort only matters when messages reach a leaf — and `resolveMessages` (inside `applyToLeaf`) already sorts the batch before merging.

The original P3 implementation sorted the buffer at two points in `flushNode`: after compacting unflushed messages (parent), and after appending flushed messages to a child. Profiling showed these sorts consumed **19.4% of CPU** — worse than the 16% memmove they replaced. Removing both sorts eliminated the regression entirely.

`splitInternalChild` partitions the buffer by pivot. The partition preserves the relative order within each half, so the `bufferSorted` flag is propagated unchanged to both halves.

### Alternatives Considered

| ID | Alternative | Verdict | Reason |
|:---|:-----------|:--------|:-------|
| A1 | Unsorted append + sort at flush time (P3v1) | Rejected after bench | Sorting 64-element buffer of 48-byte structs at flush time cost 19.4% CPU — worse than the 16% memmove it replaced. `SortStableFunc` uses function-pointer comparisons (no inlining) and merge sort internals (symMerge, rotate) that thrash cache on 48-byte swaps. |
| A2 | bufferCap = B = 4096 (paper-prescribed) | Rejected after bench | Sorting 4096 elements at flush time caused +46% write regression and +1973% read regression (linear scan on 4096 elements). Larger buffers also increased memory 85-239%. |
| A3 | Sort after every write | Rejected after bench | O(B log B) per insert is worse than O(B) memmove. Put/Random/100K regressed from 56ms to 3200ms. |
| A4 | Sorted int index (P3b from roadmap) | Deferred | Medium complexity. Only needed if the linear-scan read regression proves unacceptable in practice. Current read regression is 5-11%, acceptable for a write-optimized tree. |
| A5 | `sort.Slice` (unstable sort) | Rejected | Same-key messages must preserve insertion order for correct resolution (newest wins). Unstable sort would reorder same-key messages, breaking the semantic. |

### Consequences

**Positive:**

- Write path neutral (within noise of baseline). The 16% memmove cost is eliminated with no replacement overhead.
- Range queries 75-99% faster (from other changes between baseline capture and this version, not P3 specifically, but validated in this benchmark run).
- Upsert 16% faster.
- Zero allocation regression on write path.
- `sortBuffer` method removed — dead code eliminated.
- Simpler write path: `appendToBuffer` is now 2 lines with no comparator parameter.

**Negative:**

- Get/Hit: +9.5% regression (linear scan on unsorted root buffer).
- Get/Miss: +5.6% regression.
- Delete: +11.1% regression (Delete calls `getFromNode` which hits the linear scan).
- `bufferMessagesForKey` and `bufferSlice` allocate per-call when unsorted (collecting matches into a new slice). Baseline had zero-alloc binary search returning a sub-slice.

**Invariants to maintain:**

1. **`bufferSorted` flag accuracy.** `appendToBuffer` sets it false. `splitInternalChild` propagates the parent's flag. No other code sets it true (the only former setter, `sortBuffer`, is removed).
2. **`resolveMessages` sorts before dedup.** This is the only sort in the write path — it runs inside `applyToLeaf` when messages reach a leaf. It must remain a stable sort to preserve same-key message order.
3. **Linear scan fallback correctness.** `bufferMessagesForKey` and `bufferSliceScan` must return all matching messages regardless of buffer order. They must not mutate the buffer (safe under `RLock`).
4. **`flushNode` partition is order-independent.** Each message is routed to a child via `findChildIndex` — this does not depend on buffer sort order.
