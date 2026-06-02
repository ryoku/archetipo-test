BIN_DIR := bin
WEB_DIR := web

# ---------------------------------------------------------------------------
# Aggregate targets (run both go + web)
# ---------------------------------------------------------------------------

.PHONY: all build lint clean fmt

all: fmt lint build go\:test

build: web\:build go\:build
lint:  go\:lint web\:lint
clean: go\:clean web\:clean

fmt: go\:fmt

# ---------------------------------------------------------------------------
# Go targets
# ---------------------------------------------------------------------------

.PHONY: go\:build go\:test go\:lint go\:fmt go\:tidy go\:clean

go\:build:
	go build -tags prod -o $(BIN_DIR)/server ./cmd/server
	go build -tags prod -o $(BIN_DIR)/kubegate ./cmd/kubegate

go\:test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

go\:lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "❌ golangci-lint not found. Install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

go\:fmt:
	gofmt -w $(shell find . -name '*.go' \
		-not -path './vendor/*' \
		-not -path './$(WEB_DIR)/node_modules/*' \
		-not -path './tmp/*')

go\:tidy:
	go mod tidy

go\:clean:
	rm -rf $(BIN_DIR) coverage.out

# ---------------------------------------------------------------------------
# Web targets
# ---------------------------------------------------------------------------

.PHONY: web\:install web\:build web\:lint web\:test web\:clean

web\:install:
	cd $(WEB_DIR) && pnpm install --frozen-lockfile

web\:build: web\:install
	cd $(WEB_DIR) && pnpm build

web\:lint: web\:install
	cd $(WEB_DIR) && pnpm lint

web\:test: web\:install
	cd $(WEB_DIR) && pnpm test

web\:clean:
	rm -rf $(WEB_DIR)/dist $(WEB_DIR)/coverage

# ---------------------------------------------------------------------------
# Dev / Ops targets
# ---------------------------------------------------------------------------

.PHONY: dev dev-stop dev-stop-clean dev-smoke sonar migrate migrate-down

dev:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env  (then fill in any overrides)"; false; }
	docker compose up -d --wait
	@[ -d tmp/gitops-mock.git ] || (mkdir -p tmp && git init --bare tmp/gitops-mock.git && echo "→ Gitops mock repo initialized at tmp/gitops-mock.git")
	cd $(WEB_DIR) && pnpm install --prefer-offline
	@(cd $(WEB_DIR) && pnpm dev) & VITE_PID=$$!; trap "kill $$VITE_PID 2>/dev/null" EXIT INT TERM; \
		set -a && . ./.env && set +a && (command -v air >/dev/null 2>&1 && air || go run ./cmd/server)

dev-stop:
	docker compose down

dev-stop-clean:
	docker compose down -v

dev-smoke:
	@bash scripts/smoke-dev.sh

sonar:
	sonar-scanner

migrate:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env"; false; }
	set -a && . ./.env && set +a && go run ./cmd/migrate -direction up

migrate-down:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env"; false; }
	set -a && . ./.env && set +a && go run ./cmd/migrate -direction down
