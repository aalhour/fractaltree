# Fractal Tree (Bε-tree)

## What Is a Fractal Tree?

A **fractal tree** (formally a **Bε-tree**, pronounced "B-epsilon tree") is a write-optimized search tree used in real databases like TokuDB (now PerconaFT), and studied extensively in the academic storage systems literature.

The core insight: **every internal node carries a message buffer**. Instead of immediately propagating a write all the way down to a leaf, you *park* the write operation as a message in the nearest ancestor's buffer. When a buffer fills up, you **flush** it one level down. This batching amortizes the cost of random I/O across many writes.

```
Traditional B-tree write:  Walk root → leaf, modify leaf, done.
                           Cost: O(log_B N) I/Os per write.

Fractal tree write:        Insert message into root buffer, done (usually).
                           Cost: O(log_B N / B^ε) amortized I/Os per write.
                           For ε = 0.5 and B = 1024, that's ~32x fewer I/Os.
```

The parameter **ε** (epsilon, where 0 < ε ≤ 1) controls the trade-off:
- **ε → 0**: Larger buffers, faster writes, slower point reads (more messages to check).
- **ε → 1**: Smaller buffers, behaves more like a classic B-tree. Faster reads, slower writes.
- **ε = 0.5**: The sweet spot used in most implementations.

### A Visual Example

Imagine a tree with branching factor 4 (small, for illustration). Each internal node
has 3 pivot keys and 4 children, plus a **message buffer** that holds up to 2 pending
writes:

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

When you call `tree.Put(25, "y")`:
1. The message `PUT(25, "y")` goes into root's buffer. **Done.** No leaf touched.

When you call `tree.Put(5, "z")`:
1. The message `PUT(5, "z")` goes into root's buffer. But now the buffer has 3 messages
   and its capacity is 2 — **it's full.**
2. **Flush:** drain the buffer and push each message down one level:
   - `PUT(25, "y")` → child[2] (the 20-30 child, because 20 ≤ 25 < 30)
   - `DEL(7)` → child[0] (the <10 child, because 7 < 10)
   - `PUT(5, "z")` → child[0] (same child)
3. Those children are **leaves**, so the messages are applied directly:
   - Leaf [22, 28] → insert 25 → becomes [22, 25, 28]
   - Leaf [1, 3, 7] → delete 7, insert 5 → becomes [1, 3, 5]

When you call `tree.Get(25)`:
1. Check root's buffer for key 25. Not found (buffer was just flushed).
2. Use pivot keys: 20 ≤ 25 < 30 → descend to child[2].
3. Child[2] is a leaf → binary search for 25 → found, return "y".

**The key insight**: writes are cheap because they just append to a buffer. The
expensive part (flushing down to leaves) happens in bulk, amortizing the cost. Reads
have to check buffers at every level on the way down, which makes them slightly more
expensive than a plain B-tree — but the write speedup is worth it for write-heavy
workloads.
