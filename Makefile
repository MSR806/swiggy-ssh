.PHONY: dev build up down logs ps migrate migrate-down test test-integration lint fmt

# ── Docker Compose ────────────────────────────────────────────────────────────

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

reset:
	docker compose down -v --remove-orphans

# ── local dev (app runs on host, Docker provides Postgres + Redis) ────────────

dev:
	go run ./cmd/swiggy-ssh

migrate:
	go run ./cmd/swiggy-ssh-migrate up

migrate-down:
	go run ./cmd/swiggy-ssh-migrate down

test:
	go test ./...

.PHONY: test-integration
test-integration:
	@test -n "$(TEST_DATABASE_URL)" || (echo "TEST_DATABASE_URL is required for integration tests" && exit 1)
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./internal/infrastructure/persistence/postgres/... -v

lint:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal
