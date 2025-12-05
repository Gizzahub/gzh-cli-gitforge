# gzh-cli-gitforge Makefile
# Modular Makefile structure - includes from .make/

.DEFAULT_GOAL := help

# ==============================================================================
# Include Modular Makefiles
# ==============================================================================

include .make/vars.mk
include .make/build.mk
include .make/test.mk
include .make/quality.mk
include .make/deps.mk
include .make/tools.mk
include .make/dev.mk
include .make/docker.mk

# ==============================================================================
# Help Target
# ==============================================================================

.PHONY: help clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

clean: ## Clean build artifacts and cache files
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f $(COVERAGE_OUT) $(COVERAGE_HTML) bench.txt
	@echo "✅ Cleaned"
