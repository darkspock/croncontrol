# CronControl Justfile

# Default: show available commands
default:
    @just --list

# ============================================================================
# Setup
# ============================================================================

# Start PostgreSQL only (minimum for development)
setup:
    docker compose up -d postgres
    @echo "Waiting for PostgreSQL..."
    @until docker compose exec postgres pg_isready -U croncontrol > /dev/null 2>&1; do sleep 1; done
    just migrate
    @echo "Ready! Run 'just dev' to start CronControl."

# Start all services (PostgreSQL + OpenSearch + Dashboards + pgAdmin)
setup-full:
    docker compose --profile full up -d
    @echo "Waiting for services..."
    @until docker compose exec postgres pg_isready -U croncontrol > /dev/null 2>&1; do sleep 1; done
    just migrate
    @echo "Ready! PostgreSQL :5432 | OpenSearch :9200 | Dashboards :5601 | pgAdmin :5050"

# Stop all services
down:
    docker compose --profile full down

# ============================================================================
# Development
# ============================================================================

# Run with hot reload (air)
dev:
    air

# Run directly
run:
    go run .

# ============================================================================
# Code Generation
# ============================================================================

# Generate all (API + DB)
generate: generate-api generate-db

# Generate API code from OpenAPI spec
generate-api:
    oapi-codegen --config .oapi-codegen.yaml api/openapi.yaml

# Generate DB code from SQL queries
generate-db:
    sqlc generate

# ============================================================================
# Database Migrations (Atlas)
# ============================================================================

# Apply pending migrations
migrate:
    atlas migrate apply --env local

# Create a new migration from schema.sql diff
migrate-new name:
    atlas migrate diff {{name}} --env local

# Show migration status
migrate-status:
    atlas migrate status --env local

# Validate migration integrity
migrate-validate:
    atlas migrate validate --env local

# Seed demo data (server must be running)
seed port="8080":
    ./scripts/seed.sh http://localhost:{{port}}

# ============================================================================
# Testing
# ============================================================================

# Run all tests
test:
    go test ./... -v

# Run unit tests only (skip integration)
test-unit:
    go test ./... -v -short

# Run tests with coverage
test-coverage:
    go test ./... -v -coverprofile=coverage.out
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report: coverage.html"

# ============================================================================
# Linting
# ============================================================================

# Run all linters via pre-commit
lint:
    pre-commit run --all-files

# Run golangci-lint only
lint-go:
    golangci-lint run ./...

# ============================================================================
# Build
# ============================================================================

# Build production binary (with embedded frontend)
build:
    cd frontend && npm ci --legacy-peer-deps && npm run build
    rm -rf internal/frontend/dist && cp -r frontend/dist internal/frontend/dist
    CGO_ENABLED=0 go build -ldflags="-s -w" -o croncontrol .

# Build Docker image
docker-build:
    docker build -t croncontrol:latest .

# ============================================================================
# Utilities
# ============================================================================

# Clean generated files and build artifacts
clean:
    rm -rf croncontrol coverage.out coverage.html tmp/
    rm -rf internal/api/api.gen.go
    rm -rf db/generated/
