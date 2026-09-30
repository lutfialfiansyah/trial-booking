# trial-booking Makefile
# Load .env if it exists so variables are available to all recipes.
ifneq (,$(wildcard ./.env))
include .env
export
endif

POSTGRES_USER ?= postgres
POSTGRES_PASSWORD ?= password
POSTGRES_DB ?= trial-booking
POSTGRES_PORT ?= 5432
POSTGRES_HOST ?= localhost

DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

GOOSE ?= goose
MIGRATIONS_DIR := migrations
SERVICE := postgres

.PHONY: help tools init-migration migrate-new db-up db-down db-stop db-reset db-logs db-psql db-tables db-status print-db-url migrate-up migrate-down migrate-status setup build run-api run-seed seed demo test test-integration test-usecase test-concurrency vet docker-build docker-up docker-down docker-reset docker-logs docker-demo kill-port clean

help:
	@echo "trial-booking database commands:"
	@echo ""
	@echo "  make tools            Check for goose, install it if missing"
	@echo "  make setup            Full setup: db-up, init-migration, migrate-up, verify"
	@echo ""
	@echo "  make db-up            Start PostgreSQL (docker compose up -d postgres)"
	@echo "  make db-down          Stop and remove containers (docker compose down)"
	@echo "  make db-stop          Stop only the postgres container"
	@echo "  make db-reset         Recreate the DB volume and re-run migrations"
	@echo "  make db-logs          Tail postgres logs"
	@echo "  make db-status        Show docker compose service status"
	@echo ""
	@echo "  make init-migration   Generate the initial migration (if missing)"
	@echo "  make migrate-new name=<name>  Create a new migration file"
	@echo "  make migrate-up       Apply all pending migrations"
	@echo "  make migrate-down     Roll back the last migration"
	@echo "  make migrate-status   Show migration status"
	@echo ""
	@echo "  make db-psql          Open psql inside the postgres container"
	@echo "  make db-tables        List database tables"
	@echo "  make print-db-url     Print the generated DATABASE_URL"
	@echo ""
	@echo "  make build            Build all packages (go build ./...)"
	@echo "  make run-api          Run the API binary (go run ./cmd/api)"
	@echo "  make run-seed         Run the seed binary (go run ./cmd/seed)"
	@echo "  make seed             Reset and load deterministic demo data"
	@echo "  make demo             Run the end-to-end curl demo (scripts/demo.sh)"
	@echo "  make test             Run all tests (go test ./...)"
	@echo "  make test-integration Run repository integration tests (requires DB up)"
	@echo "  make test-usecase     Run usecase integration tests (requires DB up)"
	@echo "  make test-concurrency Run concurrency integration tests with -race (requires DB up)"
	@echo "  make vet              Run go vet ./..."
	@echo ""
	@echo "  make docker-build     Build the API Docker image"
	@echo "  make docker-up        Build and start the full stack (postgres + api)"
	@echo "  make docker-down      Stop and remove the stack"
	@echo "  make docker-reset     Recreate the stack and its volumes"
	@echo "  make docker-logs      Tail API container logs"
	@echo "  make docker-demo      Run the demo against the dockerized API"
	@echo ""
	@echo "  make kill-port        Kill any local process holding port 8080"
	@echo "  make clean            Kill port 8080 and tear down the Docker stack"

tools:
	@if command -v $(GOOSE) >/dev/null 2>&1; then \
		echo "goose is installed: $$($(GOOSE) --version 2>/dev/null || echo '$(GOOSE)')"; \
	else \
		echo "goose not found. Attempting to install via go install..."; \
		if command -v go >/dev/null 2>&1 && go install github.com/pressly/goose/v3/cmd/goose@latest; then \
			echo "goose installed successfully."; \
			echo "Make sure $$(go env GOPATH)/bin is on your PATH."; \
		else \
			echo ""; \
			echo "Automatic installation failed. Please install goose manually:"; \
			echo "  go install github.com/pressly/goose/v3/cmd/goose@latest"; \
			echo "  # or via Homebrew:"; \
			echo "  brew install goose"; \
			echo "Then ensure the binary is on your PATH."; \
			exit 1; \
		fi; \
	fi

init-migration:
	@./scripts/generate_initial_migration.sh

migrate-new:
	@if [ -z "$(name)" ]; then \
		echo "Error: please provide a migration name."; \
		echo "Usage: make migrate-new name=add_payment_index"; \
		exit 1; \
	fi
	@./scripts/create_migration.sh "$(name)"

db-up:
	docker compose up -d $(SERVICE)
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker compose exec -T $(SERVICE) pg_isready -U $(POSTGRES_USER) -d "$(POSTGRES_DB)" >/dev/null 2>&1; do \
		printf '.'; \
		sleep 1; \
	done
	@echo ""
	@echo "PostgreSQL is ready."

db-down:
	docker compose down

db-stop:
	docker compose stop $(SERVICE)

db-reset:
	docker compose down -v --remove-orphans
	$(MAKE) db-up
	$(MAKE) migrate-up

db-logs:
	docker compose logs -f $(SERVICE)

db-psql:
	docker compose exec $(SERVICE) psql -U $(POSTGRES_USER) -d "$(POSTGRES_DB)"

db-tables:
	docker compose exec -T $(SERVICE) psql -U $(POSTGRES_USER) -d "$(POSTGRES_DB)" -c "\dt"

db-status:
	docker compose ps

print-db-url:
	@echo "$(DATABASE_URL)"

migrate-up:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-down:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

migrate-status:
	$(GOOSE) -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" status

setup: db-up init-migration migrate-up db-tables print-db-url
	@echo "Setup complete."

build:
	go build ./...

run-api:
	go run ./cmd/api

run-seed:
	go run ./cmd/seed

test:
	go test ./...

test-integration:
	go test -tags=integration ./internal/repository/postgres/ -v

test-usecase:
	go test -tags=integration ./internal/usecase/ -v

test-concurrency:
	go test -race -tags=integration ./internal/usecase/ -run 'Concurrent' -v

vet:
	go vet ./...

seed:
	go run ./cmd/seed

demo:
	bash scripts/demo.sh

docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down: kill-port
	docker compose down

docker-reset:
	docker compose down -v --remove-orphans
	docker compose up -d --build

docker-logs:
	docker compose logs -f api

docker-demo:
	SKIP_SEED=1 bash scripts/demo.sh

kill-port:
	@echo "Killing any process using port 8080..."
	@-lsof -ti :8080 | xargs kill -9 2>/dev/null || true
	@echo "Port 8080 cleared."

clean: kill-port docker-down
	@echo "Environment fully cleaned."
