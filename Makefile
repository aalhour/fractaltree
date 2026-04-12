.PHONY: all build test coverage lint fmt clean examples check help
.PHONY: race fuzz bench bench-compare stress
.PHONY: profile-cpu profile-mem profile-alloc profile-mutex profile-block
.PHONY: escape flamegraph

# Default target
all: check

## build: Compile the library and verify it builds
build:
	go build ./...

## test: Run all tests
test:
	go test ./...

## examples: Run all example programs
examples:
	@if [ ! -d examples ]; then echo "No examples/ directory yet"; exit 0; fi
	@echo "Running examples..."
	@for dir in examples/*/; do \
		echo "\n=== Running $$dir ==="; \
		go run ./$$dir || exit 1; \
	done
	@echo "\n  All examples completed successfully"

## coverage: Run tests with coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## fmt: Format code using golangci-lint formatters
fmt:
	golangci-lint fmt ./...

## check: Runs fmt, lint and test
check: fmt lint test

## race: Run tests with race detector
race:
	go test -race -count=1 ./...

## fuzz: Run fuzz tests for 30 seconds
fuzz:
	go test -fuzz=Fuzz -fuzztime=30s ./...

## bench: Run benchmarks
bench:
	go test -bench=. -benchmem -count=3 -run=^$$ ./... | tee bench.out

## bench-compare: Compare benchmarks (requires benchstat)
bench-compare:
	@if [ ! -f bench.old ]; then echo "No bench.old found. Run 'make bench' and 'cp bench.out bench.old' first."; exit 1; fi
	benchstat bench.old bench.out

## stress: Run stress tests with race detector
stress:
	go test -race -run=TestStress -count=1 -timeout=5m ./...

## profile-cpu: CPU profile of benchmarks
profile-cpu:
	go test -bench=BenchmarkPut -benchmem -cpuprofile=cpu.prof -run=^$$ ./...

## profile-mem: Memory profile of benchmarks
profile-mem:
	go test -bench=BenchmarkPut -benchmem -memprofile=mem.prof -run=^$$ ./...

## profile-alloc: Allocation profile
profile-alloc:
	go test -bench=BenchmarkPut -benchmem -memprofile=alloc.prof -memprofilerate=1 -run=^$$ ./...

## profile-mutex: Mutex contention profile
profile-mutex:
	go test -bench=BenchmarkMixed -mutexprofile=mutex.prof -run=^$$ ./...

## profile-block: Block (scheduling) profile
profile-block:
	go test -bench=BenchmarkMixed -blockprofile=block.prof -run=^$$ ./...

## escape: Run escape analysis
escape:
	go build -gcflags='-m -m' ./... 2>&1 | head -100

## flamegraph: Generate CPU flame graph (opens browser)
flamegraph: profile-cpu
	go tool pprof -http=:8080 cpu.prof

## clean: Remove build artifacts and test output
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -f *.test *.out *.prof
	rm -f cpu.prof mem.prof alloc.prof mutex.prof block.prof
	rm -f bench.out bench.old

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /' | column -t -s ':'
