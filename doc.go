// Package fractaltree provides an in-memory B-epsilon-tree (fractal tree),
// a write-optimized ordered key-value data structure.
//
// A B-epsilon-tree amortizes write cost by buffering mutations as messages in
// internal nodes and flushing them toward leaves in batches. The parameter
// epsilon controls the trade-off between write throughput and read latency.
//
// # Quick Start
//
// For keys that satisfy [cmp.Ordered] (int, string, float64, ...):
//
//	t := fractaltree.New[string, int]()
//	t.Put("alice", 42)
//	v, ok := t.Get("alice") // v=42, ok=true
//
// For composite or custom-ordered keys, supply a comparator:
//
//	type TenantKey struct {
//	    Tenant string
//	    ID     int64
//	}
//
//	t := fractaltree.NewWithCompare[TenantKey, string](func(a, b TenantKey) int {
//	    if c := cmp.Compare(a.Tenant, b.Tenant); c != 0 {
//	        return c
//	    }
//	    return cmp.Compare(a.ID, b.ID)
//	})
//
// # Concurrency
//
// A [BETree] is safe for concurrent use by multiple goroutines. Reads acquire
// a shared lock; writes acquire an exclusive lock. Iterators hold a shared lock
// for their lifetime, providing snapshot-consistent traversal.
//
// # Epsilon Parameter
//
// Epsilon (set via [WithEpsilon]) must be in (0, 1] and defaults to 0.5.
// Given a block size B:
//
//   - Fanout (max children per internal node): B^epsilon
//   - Buffer capacity (max messages per node): B^(1-epsilon)
//
// Lower epsilon favors write throughput; higher epsilon favors read latency.
//
// # Complexity
//
//   - Point query:  O(log_B N)
//   - Range query:  O(log_B N + k/B) for k results
//   - Insert/Delete: O(log_B N / B^(1-epsilon)) amortized
//
// # Disk Extension
//
// [BETree] operates entirely in memory. [DiskBETree] extends it with pluggable
// persistence via the [Flusher] interface. Implement [Flusher] to control how
// and where nodes are serialized. Key and value serialization is handled by the
// [Codec] interface, with [GobCodec] provided as a default.
package fractaltree
