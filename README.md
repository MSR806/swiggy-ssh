# swiggy.dev SSH

Order Swiggy Instamart groceries from your terminal over SSH.

```
ssh swiggy.dev
```

> **Status**: Auth & identity foundation complete. Instamart integration in progress.

---

## What it is

A Go SSH server that lets you browse and order from Swiggy Instamart without leaving your terminal. Connect with your existing SSH key — the first time you connect you link your Swiggy account via a browser login code. After that, it remembers you.

---

## Prerequisites

- **Docker** with the Compose plugin
- An **SSH key pair** (Ed25519 recommended)
- Go 1.23+ *(only needed for local dev without Docker)*

---

## Quick start

```bash
# 1. Clone and enter the repo
git clone <repo-url>
cd swiggy-ssh

# 2. Copy environment config
cp .env.example .env

# 3. Build and start everything
make up
```

That's it. Compose builds the app image, starts Postgres and Redis, runs migrations, then starts the SSH + HTTP servers — in the right order automatically.

Connect in a second terminal:

```bash
ssh -p 2222 -i ~/.ssh/id_ed25519 localhost
```

**First connection** — the terminal shows a URL and a short login code. Open the URL in your browser, submit the code, and the session continues.

**Returning connections** skip the login step entirely — account home shows directly.

---

## Commands

### Full stack (Docker Compose)

| Command | What it does |
|---|---|
| `make up` | Build image + start everything (app, migrate, Postgres, Redis) |
| `make down` | Stop and remove all containers |
| `make build` | Rebuild the app image without starting |
| `make logs` | Tail app logs |
| `make ps` | Show container status |

### Local dev (app on host, deps in Docker)

| Command | What it does |
|---|---|
| `make deps-up` | Start Postgres + Redis only |
| `make deps-down` | Stop Postgres + Redis |
| `make deps-reset` | Wipe all local data (fresh start) |
| `make deps-ps` | Check container status |
| `make deps-logs` | Tail Postgres + Redis logs |
| `make migrate` | Apply all pending DB migrations |
| `make migrate-down` | Roll back one migration step |
| `make dev` | Run the app on host (requires deps-up + migrate first) |

### Code

| Command | What it does |
|---|---|
| `make test` | Run all unit tests (no DB/Redis needed) |
| `make test-integration` | Run Postgres integration tests (requires `TEST_DATABASE_URL`) |
| `make lint` | Run `go vet` |
| `make fmt` | Run `gofmt` |

---

## Environment variables

Copy `.env.example` to `.env` — all defaults work out of the box for local development.

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `local` | Environment name. Set to `production` to enable prod safeguards. |
| `SSH_ADDR` | `:2222` | SSH server listen address |
| `HTTP_ADDR` | `:8080` | HTTP server listen address (browser login page) |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Base URL shown to SSH users in the login prompt |
| `DATABASE_URL` | `postgres://swiggy:swiggy@localhost:5432/swiggy_ssh?sslmode=disable` | Postgres connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `LOGIN_CODE_TTL` | `10m` | How long a login code stays valid |
| `TOKEN_ENCRYPTION_KEY` | *(dev default)* | Base64url-encoded 32-byte AES-256 key for token encryption. **Set a real key in production.** |
| `SSH_HOST_KEY_PATH` | `.local/ssh_host_ed25519_key` | Path to the persistent SSH host key |
| `POSTGRES_PORT` | `5432` | Host port for the Compose Postgres container |
| `REDIS_PORT` | `6379` | Host port for the Compose Redis container |

### Generate a production encryption key

```bash
openssl rand -base64 32 | tr '+/' '-_' | tr -d '='
```

Set the output as `TOKEN_ENCRYPTION_KEY` in your production environment. The default dev key is publicly known from the source code and must never be used in production — the server refuses to start with it when `APP_ENV=production`.

---

## How the auth flow works

1. **SSH connect** — your Ed25519 public key fingerprint is used to look up or create your user record in Postgres.
2. **Login code** — a short-lived `XXXX-XXXX` code is issued and displayed in the terminal. Only the SHA-256 hash is stored in Redis — the raw code is never persisted.
3. **Browser confirm** — open `http://localhost:8080/login`, submit the code. The browser page calls `CompleteLoginCode`; the SSH session polls every 2 seconds.
4. **Account check** — after confirmation, the auth service either creates a new mock Swiggy account (first time) or validates your existing one. Expired or reconnect-required accounts trigger a fresh login code automatically.
5. **Returning users** — if your account is already valid, steps 2–4 are skipped entirely. You go straight to the home screen.

---

## Project layout

```
cmd/
  swiggy-ssh/           # Main server entrypoint (SSH + HTTP)
  swiggy-ssh-migrate/   # DB migration CLI (up / down / drop)

internal/
  auth/                 # Domain: OAuthAccount, LoginCode, AuthService, TokenEncryptor
  cache/                # Redis adapter: login-code service, Redis client
  config/               # Env-based config with safe defaults
  crypto/               # AES-256-GCM token encryption + NoOp for tests
  httpserver/           # HTTP delivery adapter: /login page, /health
  identity/             # SSH key → user resolution, session tracking
  instamart/            # Instamart domain (stub, in progress)
  logging/              # Structured slog setup
  provider/
    mock/               # In-memory mock provider for local dev + tests
    swiggy/             # Real Swiggy provider (stub, in progress)
  sshserver/            # SSH delivery adapter: connection handling, screen routing
  store/                # Postgres repositories + migrations
  tui/                  # Terminal screens (Bubbletea v1 + Lipgloss)
```

---

## Local development (app on host)

If you prefer a faster code/run loop with the app running directly on your machine:

```bash
# Start only the dependencies
make deps-up

# Apply migrations
make migrate

# Run the app
make dev
```

Changes to Go code take effect immediately on the next `make dev` run — no image rebuild needed.

---

## Database migrations

Migrations are embedded in the binary using Go `embed` — no external tools needed.

```bash
# Apply all pending migrations
make migrate

# Roll back one step
make migrate-down

# Drop all tables (local/dev only — refused in production)
go run ./cmd/swiggy-ssh-migrate drop
```

Migration files live in `internal/store/migrations/` as paired `*.up.sql` / `*.down.sql` files.

To reset your local database completely:

```bash
make deps-reset   # wipes Docker volumes
make deps-up      # start fresh containers
make migrate      # re-apply migrations
```

---

## Running tests

```bash
# Unit tests — no database or Redis required
make test

# Postgres integration tests
TEST_DATABASE_URL="postgres://swiggy:swiggy@localhost:5432/swiggy_ssh?sslmode=disable" make test-integration
```

Unit tests cover: auth service state machine, login code lifecycle, token encryption, TUI screen rendering, SSH session routing, HTTP login handlers, identity resolution.

---

## Port conflicts

The default ports are `2222` (SSH), `8080` (HTTP), `5432` (Postgres), `6379` (Redis). Override any of them in `.env`:

```env
SSH_PORT=2223
HTTP_PORT=8081
POSTGRES_PORT=5433
REDIS_PORT=6380
```

If you change `HTTP_PORT`, also update `PUBLIC_BASE_URL` so the login URL shown in the terminal is correct:

```env
PUBLIC_BASE_URL=http://localhost:8081
```

For local dev (app on host), use matching env vars when starting:

```bash
POSTGRES_PORT=5433 REDIS_PORT=6380 make deps-up
DATABASE_URL=postgres://swiggy:swiggy@localhost:5433/swiggy_ssh?sslmode=disable \
REDIS_URL=redis://localhost:6380/0 make dev
```

---

## Troubleshooting

**`ssh localhost -p 2222` gives `Host key verification failed`**
The server generates a new host key on first run at `.local/ssh_host_ed25519_key`. If you wiped and restarted, remove the old entry:
```bash
ssh-keygen -R [localhost]:2222
```

**`connection refused` on port 2222**
The server isn't running. Run `make up` (Docker) or `make dev` (local) first.

**Browser login page shows "Login code not found or expired"**
The 10-minute TTL elapsed, or the code was already used. Reconnect via SSH to get a new code.

**App can't connect to Postgres or Redis**
Check containers are healthy: `make deps-ps`. Verify `DATABASE_URL` and `REDIS_URL` match your port settings.

**Simulate reconnect-required**
```sql
UPDATE oauth_accounts
SET status = 'reconnect_required'
WHERE user_id = '<your-user-id>';
```
Then reconnect via SSH — a new login code will be issued automatically.

---

## Tech stack

| Layer | Technology |
|---|---|
| Language | Go 1.23 |
| SSH server | `golang.org/x/crypto/ssh` |
| HTTP server | stdlib `net/http` + `html/template` |
| TUI | Bubbletea v1 + Lipgloss (Charm) |
| Database | Postgres via `pgx/v5` |
| Cache / login codes | Redis via `go-redis/v9` |
| Migrations | `golang-migrate/migrate` (embedded) |
| Token encryption | AES-256-GCM (stdlib `crypto/aes`) |
| Local dependencies | Docker Compose |
