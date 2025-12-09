.PHONY: help dev dev-docker stop build test test-integration clean db-migrate db-rollback docker-logs

# Default target
help:
	@echo "Study in Woods - Backend v2 - Available Commands"
	@echo "=================================================="
	@echo "Development:"
	@echo "  make dev              - Start development server with hot reload (local)"
	@echo "  make dev-docker       - Start all services with Docker Compose"
	@echo "  make stop             - Stop all Docker services"
	@echo ""
	@echo "Build:"
	@echo "  make build            - Build production binary"
	@echo "  make build-docker     - Build Docker image"
	@echo ""
	@echo "Testing:"
	@echo "  make test             - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-coverage    - Run tests with coverage"
	@echo ""
	@echo "Database:"
	@echo "  make db-up            - Start PostgreSQL and Redis containers"
	@echo "  make db-down          - Stop PostgreSQL and Redis containers"
	@echo "  make db-migrate       - Run database migrations"
	@echo "  make db-rollback      - Rollback last migration"
	@echo "  make db-reset         - Reset database (drop and recreate)"
	@echo "  make db-seed          - Seed database with test data"
	@echo "  make db-users         - Create admin and test users"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-logs      - View Docker logs"
	@echo "  make docker-ps        - Show Docker container status"
	@echo "  make docker-clean     - Clean Docker volumes and containers"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean            - Clean build artifacts"
	@echo "  make fmt              - Format Go code"
	@echo "  make lint             - Run linters"
	@echo "  make deps             - Download dependencies"

# Development
dev:
	@echo "Starting development server with hot reload..."
	air -c .air.toml

dev-docker:
	@echo "Starting all services with Docker Compose..."
	docker-compose up -d
	@echo "Services started. Check status with: make docker-ps"

stop:
	@echo "Stopping all Docker services..."
	docker-compose down

# Build
build:
	@echo "Building production binary..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o bin/server .
	@echo "Binary created: bin/server"

build-docker:
	@echo "Building Docker image..."
	docker build -t study-woods-api:latest .

# Testing
test:
	@echo "Running unit tests..."
	go test -v -short ./...

test-integration:
	@echo "Running integration tests..."
	go test -v ./... -tags=integration

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Database
db-migrate:
	@echo "Running database migrations..."
	go run migrate_gorm.go

db-rollback:
	@echo "Rolling back last migration..."
	@echo "TODO: Implement rollback"

db-reset:
	@echo "Resetting database..."
	docker-compose exec postgres psql -U postgres -c "DROP DATABASE IF EXISTS study_in_woods;"
	docker-compose exec postgres psql -U postgres -c "CREATE DATABASE study_in_woods;"
	@echo "Database reset. Run migrations: make db-migrate"

db-seed:
	@echo "Seeding database with initial data..."
	go run cmd/seed/main.go

db-users:
	@echo "Creating admin and test users..."
	go run scripts/create_users.go

db-up:
	@echo "Starting PostgreSQL and Redis containers..."
	./docker_essentials start

db-down:
	@echo "Stopping PostgreSQL and Redis containers..."
	./docker_essentials stop

# Docker
docker-logs:
	docker-compose logs -f

docker-ps:
	docker-compose ps

docker-clean:
	@echo "Cleaning Docker volumes and containers..."
	docker-compose down -v
	docker system prune -f

# Utilities
clean:
	@echo "Cleaning build artifacts..."
	rm -rf tmp/ bin/ coverage.out coverage.html

fmt:
	@echo "Formatting Go code..."
	go fmt ./...
	gofmt -s -w .

lint:
	@echo "Running linters..."
	golangci-lint run ./... || echo "golangci-lint not installed or found issues"

deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
