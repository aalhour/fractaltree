.PHONY: all build test test-short coverage coverage-func lint fmt fmt-check clean check help
.PHONY: race fuzz bench bench-compare stress
.PHONY: profile-cpu profile-mem profile-alloc profile-mutex profile-block
.PHONY: escape flamegraph
.PHONY: examples deps deps-update todo doc-server info pre-commit
.PHONY: clean-test clean-fuzz

# Go binary — override with GO=/path/to/go if needed
GO ?= go

# Tier system: TIER=quick (default), long, marathon
TIER ?= quick

# Tier-specific settings
ifeq ($(TIER),marathon)
    FUZZ_TIME   := 5m
    STRESS_COUNT := 10
    BENCH_COUNT  := 5
    TEST_TIMEOUT := 15m
else ifeq ($(TIER),long)
    FUZZ_TIME   := 60s
    STRESS_COUNT := 3
    BENCH_COUNT  := 3
    TEST_TIMEOUT := 10m
else
    FUZZ_TIME   := 30s
    STRESS_COUNT := 1
    BENCH_COUNT  := 3
    TEST_TIMEOUT := 5m
endif

# Default target
all: check

# ─── Build ───────────────────────────────────────────────────────────

## build: Compile the library and verify it builds
build:
	$(GO) build ./...

# ─── Testing ─────────────────────────────────────────────────────────

## test: Run all tests
test:
	$(GO) test -timeout=$(TEST_TIMEOUT) ./...

## test-short: Run tests with -short flag (fast feedback)
test-short:
	$(GO) test -short -timeout=2m ./...

## race: Run tests with race detector
race:
	$(GO) test -race -count=1 -timeout=$(TEST_TIMEOUT) ./...

## fuzz: Run fuzz tests (duration set by TIER)
fuzz:
	$(GO) test -fuzz=FuzzOperations -fuzztime=$(FUZZ_TIME) .
	$(GO) test -fuzz=FuzzRange -fuzztime=$(FUZZ_TIME) .

## stress: Run stress tests with race detector
stress:
	$(GO) test -race -run=TestStress -count=$(STRESS_COUNT) -timeout=$(TEST_TIMEOUT) ./...

## examples: Run all example programs
examples:
	@if [ ! -d examples ]; then echo "No examples/ directory yet"; exit 0; fi
	@echo "Running examples..."
	@for dir in examples/*/; do \
		echo "\n=== Running $$dir ==="; \
		$(GO) run ./$$dir || exit 1; \
	done
	@echo "\n  All examples completed successfully"

# ─── Coverage ────────────────────────────────────────────────────────

## coverage: Run tests with coverage report (HTML)
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## coverage-func: Show per-function coverage in terminal
coverage-func:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

# ─── Code Quality ────────────────────────────────────────────────────

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format code using golangci-lint formatters
fmt:
	golangci-lint fmt ./...

## fmt-check: Check formatting without modifying files (CI-friendly)
fmt-check:
	@golangci-lint fmt --diff ./... | head -1 > /dev/null 2>&1; \
	if golangci-lint fmt --diff ./... 2>/dev/null | grep -q '^'; then \
		echo "Formatting issues found. Run 'make fmt' to fix."; \
		golangci-lint fmt --diff ./...; \
		exit 1; \
	else \
		echo "All files formatted correctly."; \
	fi

## check: Run fmt, lint and test
check: fmt lint test

## pre-commit: Lightweight gate before committing (fmt-check + lint + test-short)
pre-commit: fmt-check lint test-short

# ─── Benchmarks ──────────────────────────────────────────────────────

## bench: Run benchmarks
bench:
	$(GO) test -bench=. -benchmem -count=$(BENCH_COUNT) -run=^$$ ./... | tee bench.out

## bench-compare: Compare benchmarks (requires benchstat)
bench-compare:
	@if [ ! -f bench.old ]; then echo "No bench.old found. Run 'make bench' and 'cp bench.out bench.old' first."; exit 1; fi
	benchstat bench.old bench.out

# ─── Profiling ───────────────────────────────────────────────────────

## profile-cpu: CPU profile of benchmarks
profile-cpu:
	$(GO) test -bench=BenchmarkPut -benchmem -cpuprofile=cpu.prof -run=^$$ ./...

## profile-mem: Memory profile of benchmarks
profile-mem:
	$(GO) test -bench=BenchmarkPut -benchmem -memprofile=mem.prof -run=^$$ ./...

## profile-alloc: Allocation profile
profile-alloc:
	$(GO) test -bench=BenchmarkPut -benchmem -memprofile=alloc.prof -memprofilerate=1 -run=^$$ ./...

## profile-mutex: Mutex contention profile
profile-mutex:
	$(GO) test -bench=BenchmarkMixed -mutexprofile=mutex.prof -run=^$$ ./...

## profile-block: Block (scheduling) profile
profile-block:
	$(GO) test -bench=BenchmarkMixed -blockprofile=block.prof -run=^$$ ./...

## escape: Run escape analysis
escape:
	$(GO) build -gcflags='-m -m' ./... 2>&1 | head -100

## flamegraph: Generate CPU flame graph (opens browser)
flamegraph: profile-cpu
	$(GO) tool pprof -http=:8080 cpu.prof

# ─── Dependencies ────────────────────────────────────────────────────

## deps: Download and verify dependencies
deps:
	$(GO) mod download
	$(GO) mod verify

## deps-update: Update all dependencies to latest
deps-update:
	$(GO) get -u ./...
	$(GO) mod tidy

# ─── Utilities ───────────────────────────────────────────────────────

## todo: List all TODO/FIXME/HACK/BUG comments in the codebase
todo:
	@grep -rn 'TODO\|FIXME\|HACK\|BUG' --include='*.go' . || echo "No TODOs found."

## doc-server: Start local pkgsite documentation server
doc-server:
	@echo "Starting pkgsite at http://localhost:6060/github.com/aalhour/fractaltree"
	@command -v pkgsite >/dev/null 2>&1 || { echo "Install pkgsite: go install golang.org/x/pkgsite/cmd/pkgsite@latest"; exit 1; }
	pkgsite -http=:6060

## info: Show project metadata
info:
	@echo "Module:    $$($(GO) list -m)"
	@echo "Go:        $$($(GO) version)"
	@echo "Platform:  $$($(GO) env GOOS)/$$($(GO) env GOARCH)"
	@echo "Packages:  $$($(GO) list ./... | wc -l | tr -d ' ')"
	@echo "Go files:  $$(find . -name '*.go' -not -path './vendor/*' | wc -l | tr -d ' ')"
	@echo "Test files: $$(find . -name '*_test.go' -not -path './vendor/*' | wc -l | tr -d ' ')"
	@echo "LOC:       $$(find . -name '*.go' -not -path './vendor/*' -exec cat {} + | wc -l | tr -d ' ')"

# ─── Clean ───────────────────────────────────────────────────────────

## clean: Remove all build artifacts and test output
clean: clean-test
	rm -rf bin/

## clean-test: Remove test and profiling artifacts
clean-test:
	rm -f coverage.out coverage.html
	rm -f *.test *.out *.prof
	rm -f cpu.prof mem.prof alloc.prof mutex.prof block.prof
	rm -f bench.out bench.old

## clean-fuzz: Remove fuzz cache
clean-fuzz:
	$(GO) clean -fuzzcache

# ─── Help ────────────────────────────────────────────────────────────

## help: Show this help message
help:
	@echo ""
	@echo "fractaltree — B-epsilon-tree library for Go"
	@echo ""
	@echo "Usage: make <target> [TIER=quick|long|marathon]"
	@echo ""
	@awk 'BEGIN {section=""} \
		/^# ─── / { \
			gsub(/^# ─── /, ""); gsub(/ ───.*/, ""); \
			section=$$0; printf "\n\033[1m%s:\033[0m\n", section; next \
		} \
		/^## / { \
			sub(/^## /, ""); \
			split($$0, a, ": "); \
			printf "  \033[36m%-18s\033[0m %s\n", a[1], a[2] \
		}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Tiers control duration of long-running targets (fuzz, stress, bench):"
	@echo "  quick (default)  — fast feedback loop"
	@echo "  long             — thorough CI run"
	@echo "  marathon         — exhaustive pre-release run"
	@echo ""
