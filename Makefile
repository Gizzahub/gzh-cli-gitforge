# Makefile - gzh-cli-gitforge CLI Tool
# Modular Makefile structure with comprehensive functionality
# Git Automation Library and CLI Tool

# ==============================================================================
# Project Configuration
# ==============================================================================

# Project metadata
projectname := gzh-cli-gitforge
executablename := gz-git
# Version from VERSION file (preferred) or git tag (fallback)
VERSION ?= $(shell cat VERSION 2>/dev/null || git describe --tags --abbrev=0 2>/dev/null || echo "0.1.0")

# Go configuration
export GOPROXY=https://proxy.golang.org,direct
export GOSUMDB=sum.golang.org

# Colors for output (shared across all include files)
export CYAN := \033[36m
export GREEN := \033[32m
export YELLOW := \033[33m
export RED := \033[31m
export BLUE := \033[34m
export MAGENTA := \033[35m
export RESET := \033[0m

# ==============================================================================
# Include Modular Makefiles
# ==============================================================================

include .make/deps.mk    # Dependency management
include .make/build.mk   # Build and installation
include .make/test.mk    # Testing and coverage
include .make/quality.mk # Code quality and linting
include .make/tools.mk   # Tool installation and management
include .make/dev.mk     # Development workflow
include .make/docker.mk  # Docker operations

# ==============================================================================
# Enhanced Help System
# ==============================================================================

.DEFAULT_GOAL := help

.PHONY: help help-build help-test help-quality help-deps help-dev help-docker help-tools

help: ## show main help menu with categories
	@echo -e "$(CYAN)"
	@echo "╔══════════════════════════════════════════════════════════════════════════════╗"
	@echo -e "║                        $(MAGENTA)gzh-cli-gitforge Makefile Help$(CYAN)                      ║"
	@echo -e "║                    $(YELLOW)Git Repository Management CLI Tool$(CYAN)                      ║"
	@echo "╚══════════════════════════════════════════════════════════════════════════════╝"
	@echo -e "$(RESET)"
	@echo -e "$(GREEN)📋 Main Categories:$(RESET)"
	@echo -e "  $(YELLOW)make help-build$(RESET)    🔨 Build, installation, and deployment"
	@echo -e "  $(YELLOW)make help-test$(RESET)     🧪 Testing, benchmarks, and validation"
	@echo -e "  $(YELLOW)make help-quality$(RESET)  ✨ Code quality, formatting, and linting"
	@echo -e "  $(YELLOW)make help-deps$(RESET)     📦 Dependency management and updates"
	@echo -e "  $(YELLOW)make help-dev$(RESET)      🛠️  Development tools and workflow"
	@echo -e "  $(YELLOW)make help-docker$(RESET)   🐳 Docker operations and containers"
	@echo -e "  $(YELLOW)make help-tools$(RESET)    🔧 Tool installation and management"
	@echo ""
	@echo -e "$(GREEN)🚀 Quick Commands:$(RESET)"
	@echo -e "  $(CYAN)make start$(RESET)         Start development (run)"
	@echo -e "  $(CYAN)make stop$(RESET)          Stop development server"
	@echo -e "  $(CYAN)make restart$(RESET)       Restart development server"
	@echo -e "  $(CYAN)make status$(RESET)        Check development server status"
	@echo -e "  $(CYAN)make quick$(RESET)         Quick check (format + lint + unit tests)"
	@echo -e "  $(CYAN)make full$(RESET)          Full quality check (comprehensive)"
	@echo -e "  $(CYAN)make setup-all$(RESET)     Complete project setup"
	@echo ""
	@echo -e "$(GREEN)💡 Pro Tips:$(RESET)"
	@echo -e "  • Use $(YELLOW)'make quick'$(RESET) for fast development iteration"
	@echo -e "  • Use $(YELLOW)'make full'$(RESET) before pushing to ensure quality"
	@echo -e "  • Use $(YELLOW)'make setup-all'$(RESET) for first-time project setup"
	@echo "  • All commands support tab completion if bash-completion is installed"
	@echo ""
	@echo -e "$(BLUE)📖 Documentation: $(RESET)https://github.com/gizzahub/gzh-cli"

help-build: ## show build and deployment help
	@echo -e "$(GREEN)🔨 Build and Installation Commands:$(RESET)"
	@echo -e "  $(CYAN)build$(RESET)              Build golang binary ($(executablename))"
	@echo -e "  $(CYAN)install$(RESET)            Install golang binary to GOPATH/bin"
	@echo -e "  $(CYAN)install-git-plugin$(RESET) Install as git plugin (git forge)"
	@echo -e "  $(CYAN)run$(RESET)                Run the application"
	@echo -e "  $(CYAN)bootstrap$(RESET)          Install build dependencies"
	@echo -e "  $(CYAN)clean$(RESET)              Clean up build artifacts and binaries"
	@echo -e "  $(CYAN)release-dry-run$(RESET)    Run goreleaser in dry-run mode"
	@echo -e "  $(CYAN)release-snapshot$(RESET)   Create a snapshot release"
	@echo -e "  $(CYAN)release-check$(RESET)      Check goreleaser configuration"
	@echo -e "  $(CYAN)build-info$(RESET)         Show build environment information"

help-test: ## show testing help
	@echo -e "$(GREEN)🧪 Testing and Validation Commands:$(RESET)"
	@echo -e "  $(CYAN)test$(RESET)               Run all tests with coverage"
	@echo -e "  $(CYAN)test-unit$(RESET)          Run only unit tests (exclude integration/e2e)"
	@echo -e "  $(CYAN)test-integration$(RESET)   Run Docker-based integration tests"
	@echo -e "  $(CYAN)test-e2e$(RESET)           Run End-to-End test scenarios"
	@echo -e "  $(CYAN)test-all$(RESET)           Run all tests (unit, integration, e2e)"
	@echo -e "  $(CYAN)cover$(RESET)              Display test coverage"
	@echo -e "  $(CYAN)cover-html$(RESET)         Generate HTML coverage report"
	@echo -e "  $(CYAN)cover-report$(RESET)       Generate detailed coverage report"
	@echo -e "  $(CYAN)bench$(RESET)              Run all benchmarks"
	@echo -e "  $(CYAN)test-info$(RESET)          Show testing information and targets"

help-quality: ## show quality help
	@echo -e "$(GREEN)✨ Code Quality Commands:$(RESET)"
	@echo -e "  $(CYAN)fmt$(RESET)                Format Go files with gofumpt and gci"
	@echo -e "  $(CYAN)lint-check$(RESET)         Check lint issues without fixing"
	@echo -e "  $(CYAN)lint-fix$(RESET)           Run golangci-lint with auto-fix"
	@echo -e "  $(CYAN)security$(RESET)           Run all security checks"
	@echo -e "  $(CYAN)analyze$(RESET)            Run comprehensive code analysis"
	@echo -e "  $(CYAN)quality$(RESET)            Run comprehensive quality checks"
	@echo -e "  $(CYAN)quality-fix$(RESET)        Apply automatic quality fixes"
	@echo -e "  $(CYAN)pre-commit$(RESET)         Run pre-commit hooks"
	@echo -e "  $(CYAN)quality-info$(RESET)       Show quality tools and targets"

help-deps: ## show dependency help
	@echo -e "$(GREEN)📦 Dependency Management Commands:$(RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile.deps.mk 2>/dev/null | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(CYAN)%-20s$(RESET) %s\\n", $$1, $$2}' | head -10 || echo "  $(YELLOW)Run 'make deps-help' for dependency commands$(RESET)"

help-dev: ## show development workflow help
	@echo -e "$(GREEN)🛠️  Development Workflow Commands:$(RESET)"
	@echo -e "  $(CYAN)dev$(RESET)                Standard development workflow (format, lint, test)"
	@echo -e "  $(CYAN)dev-fast$(RESET)           Quick development cycle (format and unit tests only)"
	@echo -e "  $(CYAN)verify$(RESET)             Complete verification before PR"
	@echo -e "  $(CYAN)ci-local$(RESET)           Run full CI pipeline locally"
	@echo -e "  $(CYAN)pr-check$(RESET)           Pre-PR submission check"
	@echo -e "  $(CYAN)comments$(RESET)           Show all TODO/FIXME/NOTE comments in codebase"
	@echo -e "  $(CYAN)changelog$(RESET)          Generate changelog"
	@echo -e "  $(CYAN)docs-serve$(RESET)         Serve documentation locally"
	@echo -e "  $(CYAN)dev-info$(RESET)           Show development environment information"

help-docker: ## show Docker help
	@echo -e "$(GREEN)🐳 Docker Commands:$(RESET)"
	@echo -e "  $(CYAN)docker-build$(RESET)       Build Docker image"
	@echo -e "  $(CYAN)docker-run$(RESET)         Run Docker container"
	@echo -e "  $(CYAN)docker-stop$(RESET)        Stop and remove Docker containers"
	@echo -e "  $(CYAN)docker-logs$(RESET)        Show Docker container logs"
	@echo -e "  $(CYAN)docker-optimize$(RESET)    Analyze Docker image for optimization"
	@echo -e "  $(CYAN)docker-scan$(RESET)        Scan Docker image for vulnerabilities"
	@echo -e "  $(CYAN)docker-clean$(RESET)       Clean up Docker containers and images"
	@echo -e "  $(CYAN)docker-info$(RESET)        Show Docker information and targets"

help-tools: ## show tools help
	@echo -e "$(GREEN)🔧 Tool Management Commands:$(RESET)"
	@echo -e "  $(CYAN)install-tools$(RESET)      Install all development tools"
	@echo -e "  $(CYAN)tools-status$(RESET)       Check installed tool status"
	@echo -e "  $(CYAN)generate-mocks$(RESET)     Generate all mock files using gomock"
	@echo -e "  $(CYAN)pre-commit-install$(RESET) Install pre-commit hooks"
	@echo -e "  $(CYAN)tools-info$(RESET)         Show comprehensive tool information"

# ==============================================================================
# Project Information
# ==============================================================================

.PHONY: info about

info: ## show project information and current configuration
	@echo -e "$(CYAN)"
	@echo "╔══════════════════════════════════════════════════════════════════════════════╗"
	@echo -e "║                         $(MAGENTA)gzh-cli Project Information$(CYAN)                   ║"
	@echo "╚══════════════════════════════════════════════════════════════════════════════╝"
	@echo -e "$(RESET)"
	@echo -e "$(GREEN)📋 Project Details:$(RESET)"
	@echo -e "  Name:           $(YELLOW)$(projectname)$(RESET)"
	@echo -e "  Executable:     $(YELLOW)$(executablename)$(RESET)"
	@echo -e "  Version:        $(YELLOW)$(VERSION)$(RESET)"
	@echo ""
	@echo -e "$(GREEN)🏗️  Build Environment:$(RESET)"
	@echo "  Go Version:     $$(go version | cut -d' ' -f3)"
	@echo -e "  GOPROXY:        $(GOPROXY)"
	@echo -e "  GOSUMDB:        $(GOSUMDB)"
	@echo "  GOPATH:         $$(go env GOPATH)"
	@echo "  GOROOT:         $$(go env GOROOT)"
	@echo ""
	@echo -e "$(GREEN)📁 Key Features:$(RESET)"
	@echo "  • Multi-platform Git repository cloning (GitHub, GitLab, Gitea, Gogs)"
	@echo "  • Package manager updates (asdf, Homebrew, SDKMAN)"
	@echo "  • Development environment management (AWS, Docker, Kubernetes)"
	@echo "  • Network environment transitions (WiFi, VPN, DNS, proxy)"
	@echo "  • JetBrains IDE settings monitoring and sync fixes"
	@echo ""
	@echo -e "$(GREEN)🔧 Available Modules:$(RESET)"
	@echo -e "  • $(CYAN)Build & Deploy$(RESET)      (Makefile.build.mk)  - Build, installation, and release"
	@echo -e "  • $(CYAN)Testing$(RESET)             (Makefile.test.mk)   - Unit, integration, and e2e tests"
	@echo -e "  • $(CYAN)Code Quality$(RESET)        (Makefile.quality.mk) - Formatting, linting, and security"
	@echo -e "  • $(CYAN)Dependencies$(RESET)        (Makefile.deps.mk)   - Dependency management and updates"
	@echo -e "  • $(CYAN)Development$(RESET)         (Makefile.dev.mk)    - Development workflow and tools"
	@echo -e "  • $(CYAN)Docker$(RESET)              (Makefile.docker.mk) - Container operations and optimization"
	@echo -e "  • $(CYAN)Tools$(RESET)               (Makefile.tools.mk)  - Tool installation and management"
