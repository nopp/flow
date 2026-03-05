.PHONY: run run-dev run-prod build test tidy kind-up-local kind-up-local-podman runner-build runner-test runner-test-podman runner-debug-up runner-debug-exec runner-debug-inspect runner-debug-down runner-debug-up-podman

CONTAINER_CLI ?= docker
REGISTRY_HOST ?= localhost
REGISTRY_PORT ?= 5001
RUNNER_IMAGE_TAG ?= debug
RUNNER_IMAGE ?= $(REGISTRY_HOST):$(REGISTRY_PORT)/noppflow-runner:$(RUNNER_IMAGE_TAG)
RUNNER_DEBUG_CONTAINER ?= noppflow-runner-debug

run: run-dev

# Local development: SQLite (data/cicd.db)
run-dev:
	go run ./cmd/cicd

# Production: MySQL. Set DB_DSN (and optionally DB_DRIVER=mysql) before running, e.g.:
#   export DB_DSN='user:password@tcp(host:3306)/dbname?parseTime=true'
#   make run-prod
# Or use a .env file: source config/database.env && make run-prod
run-prod:
	@if [ -z "$$DB_DSN" ]; then echo "Error: set DB_DSN for MySQL (e.g. export DB_DSN='user:pass@tcp(host:3306)/dbname?parseTime=true')"; exit 1; fi
	DB_DRIVER=mysql go run ./cmd/cicd

build:
	go build -o bin/cicd ./cmd/cicd

test:
	go test ./...

tidy:
	go mod tidy

# Bootstrap local kind + registry + noppflow + runner + optional seeded app
# Optional overrides:
#   DB_MODE=mysql make kind-up-local
#   SEED_TEST_APP=false make kind-up-local
kind-up-local:
	./k8s/setup-kind-local.sh

# Same bootstrap flow using Podman runtime
kind-up-local-podman:
	CONTAINER_CLI=podman ./k8s/setup-kind-local.sh

# Build runner image only (useful for debugging runner tooling)
runner-build:
	$(CONTAINER_CLI) build -t $(RUNNER_IMAGE) -f Dockerfile.runner .

# Build runner image and validate required binaries exist and execute
runner-test:
	CONTAINER_CLI=$(CONTAINER_CLI) RUNNER_IMAGE=$(RUNNER_IMAGE) ./k8s/test-runner-image.sh

runner-test-podman:
	$(MAKE) CONTAINER_CLI=podman runner-test

# Start runner debug container and keep it alive for manual inspection
runner-debug-up: runner-build
	-$(CONTAINER_CLI) rm -f $(RUNNER_DEBUG_CONTAINER) >/dev/null 2>&1
	$(CONTAINER_CLI) run -d --name $(RUNNER_DEBUG_CONTAINER) --entrypoint /bin/sh $(RUNNER_IMAGE) -c "sleep 3600"

# Podman convenience target
runner-debug-up-podman:
	$(MAKE) CONTAINER_CLI=podman runner-debug-up

# Open interactive shell in debug container
runner-debug-exec:
	$(CONTAINER_CLI) exec -it $(RUNNER_DEBUG_CONTAINER) /bin/sh

# Quick check for kubectl/kaniko binaries in runner image
runner-debug-inspect:
	$(CONTAINER_CLI) exec $(RUNNER_DEBUG_CONTAINER) /bin/sh -lc 'whoami; command -v kubectl || true; command -v kaniko-executor || true; ls -l /usr/bin/kubectl /usr/local/bin/kubectl 2>/dev/null || true; kubectl version --client 2>/dev/null || true'

# Remove debug container
runner-debug-down:
	-$(CONTAINER_CLI) rm -f $(RUNNER_DEBUG_CONTAINER)
