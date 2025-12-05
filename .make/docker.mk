# .make/docker.mk - Docker targets (optional)
# Container build, optimization, and deployment

# Docker configuration
DOCKER_REGISTRY ?= gizzahub
DOCKER_IMAGE_NAME ?= $(BINARY_NAME)
DOCKER_TAG ?= $(VERSION)
DOCKER_FULL_IMAGE := $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):$(DOCKER_TAG)
DOCKER_LATEST := $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):latest

.PHONY: docker-build docker-build-dev docker-build-prod docker-push
.PHONY: docker-run docker-stop docker-logs docker-clean
.PHONY: docker-scan docker-size docker-lint

# ==============================================================================
# Docker Build
# ==============================================================================

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_FULL_IMAGE)..."
	@docker build -t $(DOCKER_FULL_IMAGE) .
	@docker tag $(DOCKER_FULL_IMAGE) $(DOCKER_LATEST)
	@echo "✅ Docker image built"

docker-build-dev: ## Build development Docker image
	@echo "Building development Docker image..."
	@docker build --target development -t $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME):dev .

docker-build-prod: ## Build production Docker image
	@echo "Building production Docker image..."
	@docker build --target production -t $(DOCKER_FULL_IMAGE) .
	@docker tag $(DOCKER_FULL_IMAGE) $(DOCKER_LATEST)

docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing Docker image..."
	@docker push $(DOCKER_FULL_IMAGE)
	@docker push $(DOCKER_LATEST)

# ==============================================================================
# Docker Runtime
# ==============================================================================

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker run -d --name $(BINARY_NAME) $(DOCKER_FULL_IMAGE)

docker-stop: ## Stop and remove Docker container
	@echo "Stopping Docker container..."
	@docker stop $(BINARY_NAME) 2>/dev/null || true
	@docker rm $(BINARY_NAME) 2>/dev/null || true

docker-logs: ## Show Docker container logs
	@docker logs $(BINARY_NAME) 2>/dev/null || echo "No container running"

docker-exec: ## Execute shell in running container
	@docker exec -it $(BINARY_NAME) /bin/sh

# ==============================================================================
# Docker Analysis
# ==============================================================================

docker-scan: ## Scan Docker image for vulnerabilities
	@echo "Scanning Docker image..."
	@if command -v trivy >/dev/null 2>&1; then \
		trivy image $(DOCKER_FULL_IMAGE); \
	else \
		echo "Install trivy for scanning: https://github.com/aquasecurity/trivy"; \
	fi

docker-size: ## Analyze Docker image size
	@echo "Docker image size:"
	@docker images $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME) --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}"

docker-lint: ## Lint Dockerfile
	@if command -v hadolint >/dev/null 2>&1; then \
		hadolint Dockerfile; \
	else \
		echo "Install hadolint: https://github.com/hadolint/hadolint"; \
	fi

# ==============================================================================
# Docker Cleanup
# ==============================================================================

docker-clean: ## Clean Docker containers and images
	@echo "Cleaning Docker..."
	@docker container prune -f
	@docker image prune -f

docker-clean-all: docker-stop docker-clean ## Comprehensive Docker cleanup
	@docker images $(DOCKER_REGISTRY)/$(DOCKER_IMAGE_NAME) -q | xargs -r docker rmi -f
	@echo "✅ Docker cleanup completed"

# ==============================================================================
# Docker Compose (optional)
# ==============================================================================

docker-compose-up: ## Start services with docker-compose
	@if [ -f "docker-compose.yml" ]; then \
		docker-compose up -d; \
	else \
		echo "No docker-compose.yml found"; \
	fi

docker-compose-down: ## Stop services with docker-compose
	@if [ -f "docker-compose.yml" ]; then \
		docker-compose down; \
	fi
