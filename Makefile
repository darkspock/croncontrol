# CronControl Makefile
# Usage: make start       — full stack (PostgreSQL + app + seed)
#        make start-full  — full stack + OpenSearch
#        make stop        — stop everything

.PHONY: start start-full stop setup setup-full build build-frontend \
        dev run test lint migrate seed clean help

# ============================================================================
# Main targets
# ============================================================================

## Start everything: PostgreSQL + migrations + build frontend + app
start: setup build-frontend
	@echo "Starting CronControl on http://localhost:8090..."
	go run .

## Start with OpenSearch logging backend
start-full: setup-full
	CC_LOGGING_BACKEND=opensearch CC_LOGGING_OPENSEARCH_URL=http://localhost:9200 go run .

## Stop all Docker services
stop:
	docker compose --profile full down

# ============================================================================
# Setup
# ============================================================================

## Start PostgreSQL and apply migrations
setup:
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL..."
	@until docker compose exec postgres pg_isready -U croncontrol > /dev/null 2>&1; do sleep 1; done
	@$(MAKE) migrate --no-print-directory
	@echo "PostgreSQL ready on :5435"

## Start all services (PostgreSQL + OpenSearch + Dashboards + pgAdmin)
setup-full:
	docker compose --profile full up -d
	@echo "Waiting for services..."
	@until docker compose exec postgres pg_isready -U croncontrol > /dev/null 2>&1; do sleep 1; done
	@$(MAKE) migrate --no-print-directory
	@echo "PostgreSQL :5435 | OpenSearch :9200 | Dashboards :5601 | pgAdmin :5050"

# ============================================================================
# Development
# ============================================================================

## Run with hot reload (requires air: go install github.com/air-verse/air@latest)
dev: setup
	air

## Run directly (no hot reload)
run:
	go run .

## Seed demo data (server must be running)
seed:
	./scripts/seed.sh http://localhost:8090

# ============================================================================
# Build
# ============================================================================

## Build production binary with embedded frontend
build: build-frontend
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$$(git describe --tags --always) -X main.commit=$$(git rev-parse --short HEAD)" -o croncontrol .
	@echo "Built: ./croncontrol"

## Build worker binary
build-worker:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o croncontrol-worker ./cmd/croncontrol-worker
	@echo "Built: ./croncontrol-worker"

## Build CLI
build-cli:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o cronctl ./cmd/cronctl
	@echo "Built: ./cronctl"

## Build MCP server
build-mcp:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o croncontrol-mcp ./cmd/croncontrol-mcp
	@echo "Built: ./croncontrol-mcp"

## Build all binaries
build-all: build build-worker build-cli build-mcp

## Build frontend (Vite)
build-frontend:
	cd frontend && npm ci --legacy-peer-deps && npm run build
	rm -rf internal/frontend/dist && cp -r frontend/dist internal/frontend/dist

## Build Docker image
docker-build:
	docker build -t croncontrol:latest .

# ============================================================================
# Database
# ============================================================================

## Apply pending migrations (incremental, safe for existing DBs)
migrate:
	@atlas migrate apply --env local 2>/dev/null || \
		(echo "Atlas not available, applying migrations manually..." && \
		for f in migrations/*.sql; do \
			echo "Applying $$f..." && \
			docker compose exec -T postgres psql -U croncontrol -d croncontrol < "$$f" 2>&1 | grep -v "already exists" || true; \
		done && echo "Migrations applied.")

## Create a new migration
migrate-new:
	@test -n "$(NAME)" || (echo "Usage: make migrate-new NAME=add_feature" && exit 1)
	atlas migrate diff $(NAME) --env local

## Show migration status
migrate-status:
	atlas migrate status --env local

# ============================================================================
# Testing & Linting
# ============================================================================

## Run all tests
test:
	go test ./...

## Run tests with verbose output
test-v:
	go test ./... -v

## Run tests with coverage report
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Run linters
lint:
	go vet ./...
	@golangci-lint run ./... 2>/dev/null || echo "golangci-lint not installed, skipping"

# ============================================================================
# Code Generation
# ============================================================================

## Generate all code (API + DB)
generate: generate-api generate-db

## Generate API types from OpenAPI spec
generate-api:
	oapi-codegen --config .oapi-codegen.yaml api/openapi.yaml

## Generate DB code from SQL queries
generate-db:
	sqlc generate

# ============================================================================
# Utilities
# ============================================================================

## Clean build artifacts
clean:
	rm -rf croncontrol croncontrol-worker cronctl croncontrol-mcp
	rm -rf coverage.out coverage.html tmp/

# ============================================================================
# Deploy
# ============================================================================

SERVER := root@49.13.224.222
DEPLOY_DIR := /opt/croncontrol

## Deploy to production (git push)
deploy:
	git push production main

## Setup production server (run once)
deploy-setup:
	ssh $(SERVER) "mkdir -p $(DEPLOY_DIR)/app && git init --bare $(DEPLOY_DIR)/repo.git"
	scp deploy/setup.sh $(SERVER):$(DEPLOY_DIR)/app/deploy/setup.sh
	ssh $(SERVER) "bash $(DEPLOY_DIR)/app/deploy/setup.sh"

## Add production git remote
deploy-remote:
	git remote add production $(SERVER):$(DEPLOY_DIR)/repo.git || echo "Remote 'production' already exists"

## SSH into server
ssh:
	ssh $(SERVER)

## Check production health
deploy-health:
	@curl -sf https://croncontrol.dev/health | python3 -m json.tool || echo "Health check failed"

## Show this help
help:
	@echo "CronControl Makefile"
	@echo ""
	@echo "Quick start:"
	@echo "  make start       Start PostgreSQL + migrations + CronControl"
	@echo "  make start-full  Same but with OpenSearch logging"
	@echo "  make stop        Stop all Docker services"
	@echo ""
	@echo "Development:"
	@echo "  make dev         Hot reload with air"
	@echo "  make run         Run directly"
	@echo "  make seed        Seed demo data"
	@echo "  make test        Run tests"
	@echo "  make lint        Run linters"
	@echo ""
	@echo "Build:"
	@echo "  make build       Production binary (with frontend)"
	@echo "  make build-all   All 4 binaries"
	@echo "  make docker-build Docker image"
	@echo ""
	@echo "Database:"
	@echo "  make migrate     Apply migrations"
	@echo "  make migrate-new NAME=x  Create new migration"
