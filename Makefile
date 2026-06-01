.PHONY: up down logs build test lint simulate clean help

# Default target.
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Docker Compose ──────────────────────────────────────────────────────────

up: ## Start all services
	docker compose up --build -d
	@echo "✅ LLMWatch is running:"
	@echo "   Dashboard:  http://localhost:3000"
	@echo "   API:        http://localhost:8080"
	@echo "   Prometheus: http://localhost:9090"
	@echo "   Grafana:    http://localhost:3001 (admin/admin)"

down: ## Stop all services
	docker compose down

down-v: ## Stop all services and remove volumes
	docker compose down -v

logs: ## Follow all service logs
	docker compose logs -f

logs-api: ## Follow API server logs
	docker compose logs -f go-api

logs-consumer: ## Follow consumer logs
	docker compose logs -f go-consumer

ps: ## Show service status
	docker compose ps

# ── Go Build ────────────────────────────────────────────────────────────────

build: ## Build all Go binaries
	go build ./cmd/api
	go build ./cmd/consumer
	go build ./cmd/simulator

build-api: ## Build just the API binary
	go build -o bin/llmwatch-api ./cmd/api

build-consumer: ## Build just the consumer binary
	go build -o bin/llmwatch-consumer ./cmd/consumer

build-simulator: ## Build the simulator binary
	go build -o bin/llmwatch-simulator ./cmd/simulator

# ── Testing ─────────────────────────────────────────────────────────────────

test: ## Run all unit tests with race detector
	go test -race -count=1 ./...

test-verbose: ## Run tests with verbose output
	go test -race -v -count=1 ./...

test-integration: ## Run integration tests (requires Docker)
	go test -race -tags=integration -v -count=1 ./integration/...

test-cover: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ── Linting ─────────────────────────────────────────────────────────────────

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Run gofmt
	gofmt -w .

vet: ## Run go vet
	go vet ./...

# ── Frontend ────────────────────────────────────────────────────────────────

frontend-dev: ## Start the frontend dev server
	cd frontend && npm run dev

frontend-build: ## Build the frontend for production
	cd frontend && npm run build

frontend-install: ## Install frontend dependencies
	cd frontend && npm install

# ── Simulator ───────────────────────────────────────────────────────────────

simulate: build-simulator ## Run the event simulator (2 RPS by default)
	./bin/llmwatch-simulator -rps 2

simulate-fast: build-simulator ## Run the event simulator at 10 RPS
	./bin/llmwatch-simulator -rps 10

# ── Utilities ───────────────────────────────────────────────────────────────

env: ## Copy .env.example to .env
	@[ -f .env ] || cp .env.example .env && echo "Created .env from .env.example"

clean: ## Remove built binaries and coverage files
	rm -rf bin/ coverage.out coverage.html

tidy: ## Run go mod tidy
	go mod tidy

.env:
	cp .env.example .env
