# AGENTS.md

## What this is

`ssh swiggy.dev` — an SSH server for ordering Swiggy Instamart groceries from the terminal. Users connect with an SSH key, link their Swiggy account via a browser login-code flow, and interact through a Bubbletea TUI.

**Current state**: Auth & identity foundation complete. Instamart integration is next.

---

## Module and baseline

- Module path: `swiggy-ssh`
- Go: `1.23.0` / `toolchain go1.23.6` — do not bump
- No new dependencies without explicit instruction

---

## Architecture

Ports & Adapters (Clean Architecture). Three layers:

- **Delivery adapters**: `internal/sshserver`, `internal/httpserver`, `internal/tui`
- **Domain / application**: `internal/auth`, `internal/identity`, `internal/instamart`
- **Infrastructure**: `internal/store`, `internal/cache`, `internal/crypto`, `internal/provider/`

**Hard rule**: delivery adapters must never import infrastructure packages. `cmd/swiggy-ssh/main.go` is the only wiring point.

**Client-agnostic backend**: domain services have no SSH/HTTP/TUI imports. SSH is the first client; future clients (web, WhatsApp, agentic) reuse the same services.

---

## Where things live

| What you need | Where to look |
|---|---|
| Domain types, ports, error sentinels | `internal/auth/auth.go`, `internal/identity/identity.go` |
| Auth orchestration (first-auth, reauth, revoked) | `internal/auth/service.go` |
| SSH connection + session routing | `internal/sshserver/server.go` |
| Browser login page handlers | `internal/httpserver/` |
| TUI screens (Bubbletea v1 + Lipgloss) | `internal/tui/tui.go` |
| Postgres repositories | `internal/store/postgres.go` |
| DB schema + migrations | `internal/store/migrations/` |
| Redis login-code service | `internal/cache/redis_logincode.go` |
| Token encryption (AES-256-GCM) | `internal/crypto/aes.go` |
| Config + env vars | `internal/config/config.go` |
| Wiring entrypoint | `cmd/swiggy-ssh/main.go` |
| In-memory mocks for tests | `internal/provider/mock/` |
| Docker / Compose setup | `Dockerfile`, `compose.yaml` |

---

## Key constraints

- Raw login codes are **never stored** — only SHA-256 hex in Redis
- `OAuthAccount.AccessToken` is **never logged or rendered** — store decrypts on read, callers get plaintext
- `TOKEN_ENCRYPTION_KEY` default is dev-only — production guard in `main.go` refuses startup with it
- `NoOpEncryptor` is for tests only — never wire in production
- Mock tokens are `mock-token-<userID>` — no real Swiggy credentials ever committed
- Every service with time logic has an injectable `now func() time.Time` — never call `time.Now()` inside a service method directly

---

## Dependency import rules

| Package | Must NOT import |
|---|---|
| `internal/auth` | anything from this repo |
| `internal/identity` | `store`, `cache`, `auth`, delivery adapters |
| `internal/tui` | `store`, `cache`, `crypto`, `sshserver`, `httpserver`, `identity` |
| `internal/httpserver` | `store`, `cache`, `crypto`, `sshserver`, `tui`, `identity` |
| `internal/sshserver` | `store`, `cache`, `crypto` |
| `internal/store` | `sshserver`, `httpserver`, `tui`, `cache` |
| `internal/cache` | `store`, `sshserver`, `httpserver`, `tui`, `identity` |
| `internal/crypto` | anything from this repo |

---

## Testing

- `go test ./...` must pass with no external services
- Integration tests in `internal/store/` skip unless `TEST_DATABASE_URL` is set — run with `make test-integration`
- No mocks library — all mocks are hand-written structs
- Interactive TUI tests use `context.WithTimeout(200ms)` so Bubbletea exits cleanly
- Read existing tests before writing new ones to follow established patterns

---

## Running

```bash
make up       # full stack via Docker Compose (builds image, runs migrations, starts everything)
make down     # stop everything

make deps-up  # deps only (Postgres + Redis) for local dev
make dev      # run app on host (after deps-up + make migrate)
make test     # unit tests
```

---

## Linear workflow

- Move issues to **In Progress** before starting work
- Move to **Done** after reviewer approval
- Post a comment on the issue summarising what was built and what tests pass
- Use the **dev + reviewer agent pattern**: new dev session per issue, new reviewer session per issue; send fixes back to the same dev session and re-review with the same reviewer session
- Linear project: **ssh swiggy.dev** — look up issue IDs via the Linear tool before updating

---

## What's next

- **Instamart integration** (SWGY-15+): product search, cart, checkout via real Swiggy API
- **Keyboard input wiring**: `HomeView`, `LoginSuccessView`, `InstamartView` have `In io.Reader` fields ready — pass `ssh.Channel` as `In` to enable cursor movement in real sessions
- **`UpdateCurrentScreen`**: `TerminalSession.CurrentScreen` is set once at session start and never updated — needs a tracker method when screen navigation is wired
- **Real Swiggy provider**: `internal/provider/swiggy/client.go` is a stub
- **Audit logging**: `audit_events` table is in the schema, no writes yet
