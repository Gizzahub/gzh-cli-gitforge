# .make/deps.mk - Dependency management
# Included by main Makefile

.PHONY: deps deps-update deps-verify deps-clean

deps: ## Download and tidy dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✅ Dependencies updated"

deps-update: ## Update all dependencies to latest
	@echo "Updating dependencies..."
	$(GO) get -u ./...
	$(GOMOD) tidy
	@echo "✅ Dependencies updated to latest"

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "✅ Dependencies verified"

deps-clean: ## Clean module cache
	@echo "Cleaning module cache..."
	$(GO) clean -modcache
	@echo "✅ Module cache cleaned"
