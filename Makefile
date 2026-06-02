BIN_DIR := bin

.PHONY: build test vet clean dev dev-stop dev-stop-clean dev-smoke sonar migrate migrate-down

build:
	cd web && pnpm install --frozen-lockfile && pnpm build
	go build -tags prod -o $(BIN_DIR)/server ./cmd/server
	go build -tags prod -o $(BIN_DIR)/kubegate ./cmd/kubegate

tidy:
	go mod tidy

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR) web/dist

dev:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env  (then fill in any overrides)"; false; }
	docker compose up -d --wait
	@[ -d tmp/gitops-mock.git ] || (mkdir -p tmp && git init --bare tmp/gitops-mock.git && echo "→ Gitops mock repo initialized at tmp/gitops-mock.git")
	cd web && pnpm install --prefer-offline
	@(cd web && pnpm dev) & VITE_PID=$$!; trap "kill $$VITE_PID 2>/dev/null" EXIT INT TERM; \
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
	set -a && . ./.env && set +a &&go run ./cmd/migrate -direction up

migrate-down:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env"; false; }
	set -a && . ./.env && set +a &&go run ./cmd/migrate -direction down
