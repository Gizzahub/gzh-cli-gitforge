# Makefile.test - Testing targets for gzh-cli
# Unit tests, integration tests, benchmarks, and coverage

# ==============================================================================
# Testing Configuration
# ==============================================================================

# Critical packages guarded by coverage-critical-check. Their floors live in
# .ci/critical-coverage-floors.json and are enforced by cmd/coveragegate.
CRITICAL_COVERAGE_PACKAGES := \
	github.com/gizzahub/gzh-cli-gitforge/internal/gitcmd \
	github.com/gizzahub/gzh-cli-gitforge/pkg/repository \
	github.com/gizzahub/gzh-cli-gitforge/pkg/reposync \
	github.com/gizzahub/gzh-cli-gitforge/pkg/workspacecli

# ==============================================================================
# Testing Targets
# ==============================================================================

.PHONY: test test-unit test-unit-quality coverage-critical-check test-integration-quality test-integration test-integration-only test-e2e test-e2e-only test-all
.PHONY: cover cover-html cover-report bench test-coverage test-docker

test: clean build ## run all tests with coverage (requires binary for integration tests)
	@echo -e "$(CYAN)Running all tests with coverage...$(RESET)"
	go test --cover -parallel=1 -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | sort -rnk3
	@echo -e "$(GREEN)✅ Tests completed$(RESET)"

test-unit: ## run only unit tests (exclude integration and e2e)
	@echo -e "$(CYAN)Running unit tests...$(RESET)"
	go test -short --cover -parallel=1 -v -coverprofile=coverage-unit.out \
		$$(go list ./... | grep -v -E '(tests/integration|tests/e2e)')
	go tool cover -func=coverage-unit.out | sort -rnk3
	@echo -e "$(GREEN)✅ Unit tests completed$(RESET)"

# quality-check is deliberately source-non-mutating with respect to the
# checkout. Keep the coverage profile in a private temporary directory; CI can
# opt into a runner-temporary output by setting QUALITY_COVERAGE_OUT for Codecov
# upload.
test-unit-quality: ## run unit tests without leaving a coverage artifact in the checkout
	@set -eu; \
	export GOWORK=off; \
	quality_tmp=$$(mktemp -d "$${TMPDIR:-/tmp}/gzh-cli-quality-tests.XXXXXX"); \
	trap 'rm -rf "$$quality_tmp"' EXIT HUP INT TERM; \
	coverage_out="$${QUALITY_COVERAGE_OUT:-$$quality_tmp/coverage-unit.out}"; \
	echo -e "$(CYAN)Running unit tests with coverage...$(RESET)"; \
	GOWORK=off go test -short --cover -parallel=1 -v -coverprofile="$$coverage_out" \
		$$(GOWORK=off go list ./... | grep -v -E '(tests/integration|tests/e2e)'); \
	go tool cover -func="$$coverage_out" | sort -rnk3; \
	echo -e "$(GREEN)✅ Unit tests completed$(RESET)"

# coverage-critical-check measures exactly the critical packages into a
# throwaway profile under tmp/ (gitignored) and fails when any package drops
# below its recorded floor. The trap deletes only the temporary directory this
# run created, on success, failure, and interrupt alike, and preserves the
# checker's exit status. tmp/ itself is left in place: concurrent make runs
# may hold their own temporary directories there.
coverage-critical-check: ## fail when a critical package's coverage drops below its recorded floor
	@set -eu; \
	export GOWORK=off; \
	mkdir -p tmp; \
	critical_tmp=$$(mktemp -d tmp/coverage-critical.XXXXXX); \
	trap 'rm -rf "$$critical_tmp"' EXIT HUP INT TERM; \
	profile="$$critical_tmp/critical-coverage.out"; \
	echo -e "$(CYAN)Measuring critical package coverage...$(RESET)"; \
	GOWORK=off go test -short -count=1 -covermode=set -coverprofile="$$profile" \
		$(CRITICAL_COVERAGE_PACKAGES); \
	echo -e "$(CYAN)Checking critical coverage floors...$(RESET)"; \
	go run ./cmd/coveragegate -manifest .ci/critical-coverage-floors.json -profile "$$profile"; \
	echo -e "$(GREEN)✅ Critical coverage floors satisfied$(RESET)"

# Keep the integration package self-contained for the canonical quality gate.
# Its TestMain builds gz-git in a private temporary directory and each test
# uses t.TempDir, so this target must not require a checked-out binary or leave
# build/coverage artifacts in the repository.
test-integration-quality: ## run package integration tests without repository artifacts
	@set -eu; \
	export GOWORK=off; \
	echo -e "$(CYAN)Running package integration tests...$(RESET)"; \
	GOWORK=off go test -short -count=1 -v ./tests/integration/...; \
	echo -e "$(GREEN)✅ Package integration tests completed$(RESET)"

test-integration-only: build ## run only integration tests with build tag
	@echo -e "$(CYAN)Running integration tests...$(RESET)"
	@if [ -d "./tests/integration" ]; then \
		cd tests/integration && go test -v .; \
	else \
		echo -e "$(YELLOW)No integration tests found$(RESET)"; \
	fi
	@echo -e "$(GREEN)✅ Integration tests completed$(RESET)"

test-e2e-only: ## run only e2e tests with build tag
	@echo -e "$(CYAN)Running E2E tests...$(RESET)"
	@if [ -d "./tests/e2e" ]; then \
		go test -tags=e2e -v ./tests/e2e/...; \
	else \
		echo -e "$(YELLOW)No e2e tests found$(RESET)"; \
	fi
	@echo -e "$(GREEN)✅ E2E tests completed$(RESET)"

test-integration: ## run Docker-based integration tests (alias for test-docker)
	@echo -e "$(CYAN)Running Docker integration tests...$(RESET)"
	@if [ -f "./tests/integration/run_docker_tests.sh" ]; then \
		./tests/integration/run_docker_tests.sh all; \
	else \
		echo -e "$(YELLOW)No Docker integration test script found$(RESET)"; \
		make test-integration-only; \
	fi
	@echo -e "$(GREEN)✅ Integration tests completed$(RESET)"

test-e2e: build ## run End-to-End test scenarios
	@echo -e "$(CYAN)Running E2E tests...$(RESET)"
	@if [ -f "./tests/e2e/run_e2e_tests.sh" ]; then \
		./tests/e2e/run_e2e_tests.sh all; \
	else \
		echo -e "$(YELLOW)No E2E test script found$(RESET)"; \
		make test-e2e-only; \
	fi
	@echo -e "$(GREEN)✅ E2E tests completed$(RESET)"
test-all: test test-integration test-e2e ## run all tests (unit, integration, e2e)
	@echo -e "$(GREEN)✅ All tests completed successfully!$(RESET)"

test-docker: test-integration ## alias for test-integration

# ==============================================================================
# Coverage Targets
# ==============================================================================

cover: ## display test coverage
	@echo -e "$(CYAN)Generating test coverage report...$(RESET)"
	go test -v -race $$(go list ./... | grep -v /vendor/) -v -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo -e "$(GREEN)✅ Coverage report generated$(RESET)"

cover-html: ## generate HTML coverage report
	@echo -e "$(CYAN)Generating HTML coverage report...$(RESET)"
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo -e "$(GREEN)✅ Coverage report generated: coverage.html$(RESET)"

cover-report: ## generate detailed coverage report
	@echo -e "$(CYAN)Generating detailed coverage report...$(RESET)"
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo -e "$(YELLOW)=== Coverage Summary ===$(RESET)"
	@go tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'
	@echo ""
	@echo -e "$(YELLOW)=== Package Coverage ===$(RESET)"
	@go tool cover -func=coverage.out | grep -v total | sort -k3 -nr | head -20
	@echo ""
	@echo -e "$(BLUE)For detailed HTML report, run: make cover-html$(RESET)"

test-coverage: cover-report ## alias for cover-report

# ==============================================================================
# Benchmark Targets
# ==============================================================================

bench: ## run all benchmarks
	@echo -e "$(CYAN)Running benchmarks...$(RESET)"
	@go test -bench=. -benchmem ./...
	@echo -e "$(GREEN)✅ Benchmarks completed$(RESET)"

bench-cpu: ## run CPU benchmarks with profiling
	@echo -e "$(CYAN)Running CPU benchmarks with profiling...$(RESET)"
	@go test -bench=. -benchmem -cpuprofile=cpu.prof ./...
	@echo -e "$(GREEN)✅ CPU benchmarks completed$(RESET)"
	@echo -e "$(YELLOW)Use 'go tool pprof cpu.prof' to analyze$(RESET)"

bench-mem: ## run memory benchmarks with profiling
	@echo -e "$(CYAN)Running memory benchmarks with profiling...$(RESET)"
	@go test -bench=. -benchmem -memprofile=mem.prof ./...
	@echo -e "$(GREEN)✅ Memory benchmarks completed$(RESET)"
	@echo -e "$(YELLOW)Use 'go tool pprof mem.prof' to analyze$(RESET)"

bench-compare: ## compare benchmarks (requires benchstat)
	@echo -e "$(CYAN)Comparing benchmarks...$(RESET)"
	@command -v benchstat >/dev/null 2>&1 || { echo "Installing benchstat $(BENCHSTAT_VERSION)..." && go install golang.org/x/perf/cmd/benchstat@$(BENCHSTAT_VERSION); }
	@go test -bench=. -count=5 ./... > new.bench
	@echo -e "$(GREEN)✅ Benchmark comparison data generated: new.bench$(RESET)"
	@echo -e "$(YELLOW)Run 'benchstat old.bench new.bench' to compare$(RESET)"

.PHONY: benchmark-report
benchmark-report: ## convert captured benchmark text to a schema v1 JSON report (INPUT=... METADATA=... OUTPUT=...)
	@if [ -z "$(INPUT)" ] || [ -z "$(METADATA)" ] || [ -z "$(OUTPUT)" ]; then \
		echo "benchmark-report requires INPUT=<bench text> METADATA=<metadata json> OUTPUT=<report json>" >&2; \
		exit 2; \
	fi
	@echo -e "$(CYAN)Converting benchmark report...$(RESET)"
	@GOWORK=off go run ./cmd/benchmark-report --input "$(INPUT)" --metadata "$(METADATA)" --output "$(OUTPUT)"
	@echo -e "$(GREEN)✅ Benchmark report written to $(OUTPUT)$(RESET)"

# benchmark-record measures the current source into OUTPUT_DIR. It refuses a
# dirty tree before building or writing anything (git status
# --porcelain --untracked-files=normal must be empty; ignored paths such as
# tmp/ and bin/ never appear there), so the metadata's sourceCommit always
# names the exact tree the samples came from. OUTPUT_DIR must be new or an
# empty directory. The benchmark builds its own private gz-git into a
# temporary directory (benchmarks buildPrivateBinary); the repository-root and
# PATH gz-git are never used or replaced. On any measurement failure the raw
# output is echoed and no report is written; the intermediate work directory
# is removed on success, failure, and interrupt alike.
.PHONY: benchmark-record
benchmark-record: ## record BenchmarkCLIStatus with metadata into OUTPUT_DIR=<new or empty dir> (requires a clean tree)
	@if [ -z "$(OUTPUT_DIR)" ]; then \
		echo "benchmark-record requires OUTPUT_DIR=<new or empty directory>" >&2; \
		exit 2; \
	fi
	@set -eu; \
	work_dir=$$(mktemp -d "$${TMPDIR:-/tmp}/gz-git-benchmark-record.XXXXXX"); \
	trap 'rm -rf "$$work_dir"' EXIT HUP INT TERM; \
	dirty=$$(git status --porcelain --untracked-files=normal); \
	if [ -n "$$dirty" ]; then \
		echo "benchmark-record refuses a dirty source tree:" >&2; \
		printf '%s\n' "$$dirty" >&2; \
		echo "Commit or clean the tree, then re-run. Ignored paths (tmp/, bin/) do not count." >&2; \
		exit 1; \
	fi; \
	out_dir="$(OUTPUT_DIR)"; \
	if [ -e "$$out_dir" ] && [ ! -d "$$out_dir" ]; then \
		echo "OUTPUT_DIR $$out_dir exists and is not a directory" >&2; \
		exit 1; \
	fi; \
	if [ -d "$$out_dir" ] && [ -n "$$(ls -A "$$out_dir")" ]; then \
		echo "OUTPUT_DIR $$out_dir exists and is not empty" >&2; \
		exit 1; \
	fi; \
	source_commit=$$(git rev-parse HEAD); \
	go_version=$$(go version); \
	git_version=$$(git --version); \
	go_os=$$(go env GOOS); \
	go_arch=$$(go env GOARCH); \
	observed_at=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	measurement_command='go test -run=^$$ -bench=^BenchmarkCLIStatus$$ -count=3 -benchtime=100ms -benchmem ./benchmarks'; \
	printf '{"sourceCommit":"%s","goVersion":"%s","gitVersion":"%s","os":"%s","arch":"%s","workload":"%s","observedAt":"%s","measurementCommand":"%s","note":"%s"}\n' \
		"$$source_commit" "$$go_version" "$$git_version" "$$go_os" "$$go_arch" \
		"gz-git status on a single-commit temporary repository (BenchmarkCLIStatus)" \
		"$$observed_at" "$$measurement_command" \
		"recorded by make benchmark-record; mean ns/op over 3 samples, not per-operation p95" \
		> "$$work_dir/metadata.json"; \
	echo -e "$(CYAN)Recording BenchmarkCLIStatus (3 samples) from $$source_commit...$(RESET)"; \
	if ! GOWORK=off go test -run='^$$' -bench='^BenchmarkCLIStatus$$' -count=3 -benchtime=100ms -benchmem ./benchmarks \
		> "$$work_dir/bench.txt" 2>&1; then \
		echo "benchmark measurement failed; no report was written. Raw output:" >&2; \
		cat "$$work_dir/bench.txt" >&2; \
		exit 1; \
	fi; \
	mkdir -p "$$out_dir"; \
	cp "$$work_dir/bench.txt" "$$out_dir/bench.txt"; \
	cp "$$work_dir/metadata.json" "$$out_dir/metadata.json"; \
	GOWORK=off go run ./cmd/benchmark-report --input "$$work_dir/bench.txt" --metadata "$$work_dir/metadata.json" --output "$$out_dir/report.json"; \
	echo -e "$(GREEN)✅ Recorded benchmarks into $$out_dir (bench.txt, metadata.json, report.json)$(RESET)"

# ==============================================================================
# Test Utilities
# ==============================================================================

test-race: ## run tests with race detection
	@echo -e "$(CYAN)Running tests with race detection...$(RESET)"
	@go test -race -short ./...
	@echo -e "$(GREEN)✅ Race detection tests completed$(RESET)"

test-verbose: ## run tests with verbose output
	@echo -e "$(CYAN)Running tests with verbose output...$(RESET)"
	@go test -v ./...
	@echo -e "$(GREEN)✅ Verbose tests completed$(RESET)"

test-timeout: ## run tests with custom timeout
	@echo -e "$(CYAN)Running tests with 30s timeout...$(RESET)"
	@go test -timeout=30s ./...
	@echo -e "$(GREEN)✅ Timeout tests completed$(RESET)"

test-list: ## list all available tests
	@echo -e "$(CYAN)Listing all available tests...$(RESET)"
	@go test -list . ./... | grep -E '^Test|^Benchmark'
	@echo -e "$(GREEN)✅ Test listing completed$(RESET)"

# ==============================================================================
# Test Information
# ==============================================================================

.PHONY: test-info

test-info: ## show testing information and available targets
	@echo -e "$(CYAN)"
	@echo "╔══════════════════════════════════════════════════════════════════════════════╗"
	@echo -e "║                         $(YELLOW)Testing Information$(CYAN)                             ║"
	@echo "╚══════════════════════════════════════════════════════════════════════════════╝"
	@echo -e "$(RESET)"
	@echo -e "$(GREEN)🧪 Test Categories:$(RESET)"
	@echo -e "  • $(CYAN)Unit Tests$(RESET)          Fast, isolated component tests"
	@echo -e "  • $(CYAN)Integration Tests$(RESET)   Docker-based service integration"
	@echo -e "  • $(CYAN)E2E Tests$(RESET)           End-to-end scenario testing"
	@echo ""
	@echo -e "$(GREEN)📊 Coverage Targets:$(RESET)"
	@echo -e "  • $(CYAN)cover$(RESET)               Display test coverage"
	@echo -e "  • $(CYAN)cover-html$(RESET)          Generate HTML coverage report"
	@echo -e "  • $(CYAN)cover-report$(RESET)        Detailed coverage analysis"
	@echo ""
	@echo -e "$(GREEN)⚡ Benchmark Targets:$(RESET)"
	@echo -e "  • $(CYAN)bench$(RESET)               Run all benchmarks"
	@echo -e "  • $(CYAN)bench-cpu$(RESET)           CPU benchmarks with profiling"
	@echo -e "  • $(CYAN)bench-mem$(RESET)           Memory benchmarks with profiling"
	@echo -e "  • $(CYAN)bench-compare$(RESET)       Compare benchmark results"
	@echo -e "  • $(CYAN)benchmark-report$(RESET)    Convert captured results to a JSON report"
	@echo ""
	@echo -e "$(GREEN)🔧 Test Utilities:$(RESET)"
	@echo -e "  • $(CYAN)test-race$(RESET)           Run with race detection"
	@echo -e "  • $(CYAN)test-verbose$(RESET)        Run with verbose output"
	@echo -e "  • $(CYAN)test-timeout$(RESET)        Run with custom timeout"
	@echo -e "  • $(CYAN)test-list$(RESET)           List all available tests"
