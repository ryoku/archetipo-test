BIN_DIR := bin

.PHONY: build test vet clean dev dev-stop dev-stop-clean dev-smoke migrate migrate-down

build:
	cd web && npm ci && npm run build
	go build -o $(BIN_DIR)/server ./cmd/server
	go build -o $(BIN_DIR)/kubegate ./cmd/kubegate

test:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)

dev:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env  (then fill in any overrides)"; false; }
	docker compose up -d --wait
	@[ -d tmp/gitops-mock.git ] || (mkdir -p tmp && git init --bare tmp/gitops-mock.git && echo "→ Gitops mock repo initialized at tmp/gitops-mock.git")
	cd web && npm install --prefer-offline
	@(cd web && npm run dev) & VITE_PID=$$!; trap "kill $$VITE_PID 2>/dev/null" EXIT INT TERM; \
		set -a && . ./.env && set +a && (command -v air >/dev/null 2>&1 && air || go run ./cmd/server)

dev-stop:
	docker compose down

dev-stop-clean:
	docker compose down -v

dev-smoke:
	@bash scripts/smoke-dev.sh

migrate:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env"; false; }
	set -a && . ./.env && set +a &&go run ./cmd/migrate -direction up

migrate-down:
	@[ -f .env ] || { echo "❌ .env not found. Run: cp .env.example .env"; false; }
	set -a && . ./.env && set +a &&go run ./cmd/migrate -direction down
