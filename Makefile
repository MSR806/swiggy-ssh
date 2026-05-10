.PHONY: dev build up down logs ps migrate migrate-down test test-integration lint fmt deps-up deps-down deps-logs deps-ps deps-reset

# ── full-stack Docker Compose (app + deps) ───────────────────────────────────

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f app

ps:
	docker compose ps

# ── local dev (run app on host, deps in Docker) ──────────────────────────────

dev:
	go run ./cmd/swiggy-ssh

migrate:
	go run ./cmd/swiggy-ssh-migrate up

migrate-down:
	go run ./cmd/swiggy-ssh-migrate down

deps-up:
	docker compose up -d

deps-down:
	docker compose down

deps-logs:
	docker compose logs -f postgres redis

deps-ps:
	docker compose ps

deps-reset:
	docker compose down -v --remove-orphans

test:
	go test ./...

.PHONY: test-integration
test-integration:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required for integration tests" && exit 1)
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./internal/store/... -v

lint:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal
