# Contributing to fractaltree

Thank you for your interest in contributing. This guide covers the development workflow, testing practices, and benchmark procedures.

---

## Getting Started

```bash
git clone https://github.com/aalhour/fractaltree.git
cd fractaltree
make deps        # download dependencies
make check       # fmt + lint + test — must pass before any PR
```

**Requirements:** Go 1.26+, [golangci-lint](https://golangci-lint.run/) v2.

---

## Development Workflow

Every change follows the same loop: write a failing test, make it pass, lint.

1. Write a test that describes the behavior you want.
2. `make test` — confirm it fails for the right reason.
3. Write the minimum code to make it pass.
4. `make test` — confirm it passes.
5. `make lint` — fix every complaint. No `//nolint` unless documenting a false positive.
6. `make check` — final gate before committing.

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html).
- Use modern stdlib: `slices.Insert`, `cmp.Compare`, `maps.Keys`, etc.
- Use `strconv` over `fmt.Sprintf` for simple conversions.
- Use early returns to reduce nesting.
- Keep lines under 120 characters.
- Add comments only where the *why* is not obvious.
- Do not add features beyond what the task requires.

### Zero Runtime Dependencies

The library has no external runtime dependencies. Only test dependencies (`testify`, `go-cmp`, `goleak`) are allowed. Do not add runtime dependencies.

---

## Writing Tests

### Principles

Test **behaviors and scenarios**, not lines of code. Every test should answer: *"what happens when...?"*

- **Happy path:** Normal usage works correctly.
- **Edge cases:** Empty tree, zero-value keys, `math.MinInt`, `math.MaxInt`, empty string, unicode keys.
- **Ordering:** Sequential, reverse, and random insertion all produce correct sorted output.
- **Flush behavior:** Inserting more keys than the buffer capacity triggers flushes and splits correctly.
- **Concurrency:** Concurrent readers and writers do not corrupt state or panic.
- **Error conditions:** Invalid epsilon, nil comparator, operations on a closed tree.

### Test Organization

| File | What it covers |
|:-----|:--------------|
| `tree_test.go` | Integration tests for BETree (constructors, Put/Get/Delete, flush, splits, edge cases) |
| `iterator_test.go` | Range iterators, snapshot semantics |
| `cursor_test.go` | Cursor positioning, Seek, Next, Prev |
| `upsert_test.go` | Upsert, PutIfAbsent, Increment, CompareAndSwap |
| `fuzz_test.go` | Random operation sequences verified against a reference `map` |
| `bench_test.go` | Benchmarks with `b.ReportAllocs()` |
| `stress_test.go` | Concurrent stress tests (run with `-race`) |
| `example_test.go` | `Example*` functions for pkg.go.dev |

### Naming

Use descriptive subtests named after the scenario:

```go
func TestDelete(t *testing.T) {
    t.Run("existing key returns true", func(t *testing.T) { ... })
    t.Run("missing key returns false", func(t *testing.T) { ... })
    t.Run("after delete Get returns false", func(t *testing.T) { ... })
    t.Run("delete from empty tree", func(t *testing.T) { ... })
}
```

### Running Tests

```bash
make test          # go test ./...
make test-short    # go test -short ./... (fast feedback)
make race          # go test -race -count=1 ./...
make fuzz          # fuzz testing (duration set by TIER)
make stress        # concurrent stress tests with -race
make coverage      # generates coverage.html
```

### Test Dependencies

- **`testify`** — use `require` for preconditions (stops on failure), `assert` for actual checks.
- **`go-cmp`** — deep struct comparison when testify's `Equal` is not enough.
- **`goleak`** — goroutine leak detection via `TestMain`.

---

## Benchmarks

### Running Benchmarks

```bash
make bench                     # run all benchmarks, output to bench.out
```

For statistically rigorous results, use `count=6`:

```bash
go test -bench=. -benchmem -count=6 -timeout=30m . > bench.out
```

### Comparing Against Baseline

A benchmark baseline is tracked in `benchmarks/baseline.txt`. Use `benchstat` to compare:

```bash
# Run new benchmarks
go test -bench=. -benchmem -count=6 -timeout=30m . > bench.out

# Compare against baseline
benchstat benchmarks/baseline.txt bench.out

# If results are good, update the baseline
cp bench.out benchmarks/baseline.txt
```

`benchstat` requires `count >= 5` to compute statistical significance. Always use `count=6`.

### Cross-Implementation Comparison

The `testdata/` directory contains benchmarks comparing fractaltree against Google's `btree` v1.1.3. These are gated behind a build tag to keep the dependency out of normal builds:

```bash
go test -tags compare -bench=. -benchmem -count=6 -timeout=30m ./testdata/
```

Results are tracked in `benchmarks/btree_compare.txt`.

### Investigating Regressions

If a change regresses performance:

1. **Identify the hot path:** `make profile-cpu` generates `cpu.prof`. View with `go tool pprof cpu.prof`.
2. **Find allocations:** `make profile-mem` or `make profile-alloc` (allocation rate = 1).
3. **Visualize:** `make flamegraph` opens a CPU flame graph in the browser.
4. **Check escape analysis:** `make escape` shows which variables escape to the heap.
5. **Narrow it down:** Run a single benchmark in isolation:
   ```bash
   go test -bench=BenchmarkPut/Random/100000 -benchmem -count=6 -timeout=10m .
   ```

### Benchmark Files

| File | Purpose |
|:-----|:--------|
| `benchmarks/baseline.txt` | Current performance baseline (raw `go test -bench` output) |
| `benchmarks/benchstat_*.txt` | Saved `benchstat` comparisons between versions |
| `benchmarks/btree_compare.txt` | Cross-implementation comparison with Google BTree |
| `bench_test.go` | Benchmark definitions (Put, Get, Delete, Range, Mixed, Upsert) |
| `testdata/btree_compare_test.go` | Cross-implementation benchmark definitions |

---

## Architecture Decisions

Design decisions with non-obvious trade-offs are documented in [`docs/ADR.md`](docs/ADR.md). Read the relevant ADRs before modifying:

- **Flush and leaf merge** (`flush.go`) — [ADR-001](docs/ADR.md#adr-001-batch-leaf-merge): batch merge paths, why three paths exist, sort elision, the reverse merge invariant.
- **Size tracking** (`tree.go`) — [ADR-002](docs/ADR.md#adr-002-optimistic-size-tracking-with-deferred-correction): optimistic `t.size` with deferred correction formula.
- **Flush bucket reuse** (`flush.go`, `node.go`) — [ADR-003](docs/ADR.md#adr-003-greedy-flush-with-reusable-buckets): reusable per-node bucket slices, in-place buffer compaction.

If your change introduces a new non-obvious design decision, add an ADR following the existing format: Context, Decision, Alternatives Considered, Consequences.

---

## Project Documentation

| Document | Purpose |
|:---------|:--------|
| [`README.md`](README.md) | User-facing: installation, API, performance summary |
| [`docs/PERFORMANCE.md`](docs/PERFORMANCE.md) | Full benchmarks, Google BTree comparison, reproduction instructions |
| [`docs/ADR.md`](docs/ADR.md) | Architecture decision records — the *why* behind design choices |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | Completed work, profiling data, remaining optimizations, feature backlog |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Development workflow, testing practices, benchmark procedures |
| [`doc.go`](doc.go) | Package documentation for pkg.go.dev |

---

## Makefile Quick Reference

| Target | When to use |
|:-------|:-----------|
| `make check` | End of every task (fmt + lint + test) |
| `make test` | After every code change |
| `make lint` | After every code change |
| `make race` | After touching concurrent code |
| `make bench` | After performance-related changes |
| `make fuzz` | After changing core tree logic |
| `make stress` | After touching locking or shared state |
| `make coverage` | To inspect untested paths |
| `make flamegraph` | To visualize CPU hot paths |
| `make escape` | To find heap escapes in hot paths |
