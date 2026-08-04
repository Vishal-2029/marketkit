# MarketKit — Makefile
# Usage: make <target>

.PHONY: help quickstart bootstrap smoke seed-demo dev prod deploy up down build logs seed run fmt lint tidy clean \
        web-install web-dev web-build web-preview start stop app swagger-docs apk publish release \
        test test-db-up test-db-down

# ─── Config ───────────────────────────────────────────────────────────────────
API_DIR     := api
WEB_DIR     := web
BINARY      := $(API_DIR)/tmp/server
COMPOSE_DEV := docker compose -f docker-compose.yml -f docker-compose.dev.yml
COMPOSE_PRD := docker compose -f docker-compose.yml

# ─── Default ──────────────────────────────────────────────────────────────────
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo ""
	@echo "  MarketKit"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""

# ─── Full Stack ───────────────────────────────────────────────────────────────
start: ## Start API (Docker) + admin web panel
	@echo "Building and starting backend..."
	$(COMPOSE_DEV) up -d --build --wait
	@echo "API is ready!"
	cd $(WEB_DIR) && npm run dev

app: ## Start Flutter app on phone/browser at :8081
	cd app && flutter run -d web-server --web-hostname 0.0.0.0 --web-port 8081

apk: ## Build release APK (production API URL)
	@./scripts/build-apk.sh

publish: ## Upload built APK to server so users get the in-app update prompt
	@./scripts/publish-apk.sh

release: ## Build and publish the APK in one step (avoids publishing a stale binary)
	@./scripts/build-apk.sh
	@./scripts/publish-apk.sh

stop: ## Stop backend containers (frontend: Ctrl+C in its terminal)
	$(COMPOSE_DEV) down

# ─── Frontend ─────────────────────────────────────────────────────────────────
web-install: ## Install frontend npm dependencies
	cd $(WEB_DIR) && npm install

web-dev: ## Start Vite dev server (http://localhost:5173)
	cd $(WEB_DIR) && npm run dev

web-build: ## Build frontend for production
	cd $(WEB_DIR) && npm run build

web-preview: ## Preview the production build locally
	cd $(WEB_DIR) && npm run preview

# ─── Quick start ──────────────────────────────────────────────────────────────
# Read the host port from the generated .env so quickstart polls the right one.
API_PORT = $(shell grep -E '^PORT=' .env 2>/dev/null | cut -d= -f2 || echo 3000)

quickstart: bootstrap ## FIRST RUN: generate .env files, start everything, seed demo data
	@echo ""
	@echo "==> Starting containers (first build takes a few minutes)…"
	$(COMPOSE_DEV) up --build -d
	@echo ""
	@echo "==> Waiting for the API to become healthy…"
	@until curl -sf http://localhost:$(API_PORT)/health >/dev/null 2>&1; do \
		printf '.'; sleep 2; \
	done; echo " ready"
	@$(MAKE) --no-print-directory seed-demo
	@echo ""
	@echo "  API          http://localhost:$(API_PORT)"
	@echo "  API docs     http://localhost:$(API_PORT)/docs/index.html"
	@echo "  Mailhog      http://localhost:8025"
	@echo ""
	@echo "  Admin panel:  make web-dev   -> http://localhost:5173"
	@echo "  Demo logins:  seller1@demo.marketkit.test / demo1234"
	@echo "  Stop with:    make down"

bootstrap: ## Generate .env and api/.env with random secrets (safe to re-run)
	@./scripts/bootstrap.sh

# ─── Backend (Docker) ─────────────────────────────────────────────────────────
dev: bootstrap ## Start backend services in dev mode (hot reload + Mailhog)
	$(COMPOSE_DEV) up --build

dev-test: ## Start backend with api/.env.test (Docker Compose)
	@echo "Starting dev backend with api/.env.test (Razorpay Test)..."
	@cp -f $(API_DIR)/.env.test $(API_DIR)/.env
	$(COMPOSE_DEV) up --build

dev-live: ## Start backend with api/.env.live (Docker Compose)
	@echo "Starting dev backend with api/.env.live (Razorpay Live)..."
	@cp -f $(API_DIR)/.env.live $(API_DIR)/.env
	$(COMPOSE_DEV) up --build

prod: ## Start backend in production mode (detached)
	$(COMPOSE_PRD) up --build -d

deploy: ## Build web assets (Docker) and start production stack
	@./scripts/deploy.sh

up: bootstrap ## Start backend containers without rebuilding
	$(COMPOSE_DEV) up

down: ## Stop and remove all containers
	$(COMPOSE_DEV) down

down-v: ## Stop containers and delete volumes (wipes database)
	$(COMPOSE_DEV) down -v

build: ## Rebuild Docker images from scratch
	$(COMPOSE_DEV) build --no-cache

logs: ## Tail logs for all services
	$(COMPOSE_DEV) logs -f

logs-api: ## Tail API logs only
	$(COMPOSE_DEV) logs -f api

restart-api: ## Restart only the API container
	$(COMPOSE_DEV) restart api

# ─── Backend (Local Go, no Docker) ───────────────────────────────────────────
run: ## Run the API server locally (uses localhost Postgres)
	cd $(API_DIR) && DATABASE_URL="postgres://marketkit:marketkit@localhost:5433/marketkit?sslmode=disable" go run ./cmd/api

seed: ## Seed database with admin accounts and plans (runs inside api container)
	$(COMPOSE_DEV) exec api go run /app/seed.go

seed-demo: ## Fill the database with demo sellers, products, purchases and wallets
	$(COMPOSE_DEV) exec api go run /app/seed/demo.go

seed-demo-reset: ## Remove seeded demo data, then re-seed it
	$(COMPOSE_DEV) exec api go run /app/seed/demo.go -reset

smoke: ## End-to-end check against the running stack (money path + attack paths)
	@API=http://localhost:$(API_PORT) ./scripts/smoke.sh

tidy: ## Tidy go.mod and go.sum
	cd $(API_DIR) && go mod tidy

fmt: ## Format all Go source files
	cd $(API_DIR) && gofmt -w ./..

lint: ## Run go vet (static analysis)
	cd $(API_DIR) && go vet ./...

build-bin: ## Build the API binary locally to api/tmp/server
	cd $(API_DIR) && go build -o tmp/server ./cmd/api

# ─── Tests ────────────────────────────────────────────────────────────────────
# Tests run against a real, throwaway Postgres — never the dev/prod database
# (models rely on Postgres-only features: gen_random_uuid(), jsonb, SELECT
# ... FOR UPDATE row locking, which the wallet ledgers depend on).
TEST_DB_CONTAINER := smh-test-postgres
TEST_DB_PORT       := 5434
TEST_DATABASE_URL  := postgres://test:test@localhost:$(TEST_DB_PORT)/smh_test?sslmode=disable

test-db-up: ## Start the throwaway test Postgres (isolated from dev data)
	@docker rm -f $(TEST_DB_CONTAINER) >/dev/null 2>&1 || true
	docker run -d --name $(TEST_DB_CONTAINER) -p $(TEST_DB_PORT):5432 \
		-e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=smh_test \
		postgres:16-alpine >/dev/null
	@echo "Waiting for test postgres..."
	@until docker exec $(TEST_DB_CONTAINER) psql -U test -d smh_test -c 'SELECT 1' >/dev/null 2>&1; do sleep 1; done

test-db-down: ## Stop and remove the throwaway test Postgres
	@docker rm -f $(TEST_DB_CONTAINER) >/dev/null 2>&1 || true

test: test-db-up ## Run Go tests against a throwaway Postgres, then tear it down
	@# -p 1: every package migrates the same throwaway database on startup, and
	@# on a freshly-created one they race. The suite runs in seconds, so
	@# serialising costs little and removes a confusing first-run failure.
	@(cd $(API_DIR) && TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -p 1 ./...); \
	status=$$?; \
	$(MAKE) test-db-down; \
	exit $$status

# ─── Database ─────────────────────────────────────────────────────────────────
db-shell: ## Open a psql shell inside the postgres container
	$(COMPOSE_DEV) exec postgres psql -U marketkit -d marketkit

db-reset: ## Drop and recreate the database (wipes all data)
	$(COMPOSE_DEV) exec postgres psql -U marketkit -c "DROP DATABASE IF EXISTS marketkit;"
	$(COMPOSE_DEV) exec postgres psql -U marketkit -c "CREATE DATABASE marketkit;"

# ─── Cleanup ──────────────────────────────────────────────────────────────────
swagger: ## Generate swagger docs (swaggo/swag) into api/docs
	cd $(API_DIR) && go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go -o docs

clean: ## Remove local build artifacts
	rm -rf $(API_DIR)/tmp
