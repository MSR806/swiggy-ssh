---
name: swiggy-ssh-auth
description: Use when building or modifying the end-to-end SSH + browser authentication flow for swiggy-ssh. Covers host key management, SSH public key identity, login code flow, OAuth token lifecycle, re-authentication, HTTP login page, and database schema.
license: MIT
metadata:
  author: swiggy-ssh team
---

# swiggy-ssh Authentication Flow

This skill documents the complete end-to-end authentication architecture of swiggy-ssh — from a user running `ssh swiggy.dev` to a fully authenticated session backed by a Swiggy OAuth account.

---

## Architecture Overview

Two servers run in parallel:

```
swiggy.dev
├── SSH server  (port 2222)  ← user connects via terminal
└── HTTP server (port 8080)  ← user opens in browser to complete login
```

They share one thing: a **login code** stored in Redis. That code is the bridge between the terminal session and the browser login.

---

## Concept 1: Server Identity — Host Key

**What it is:** A long-lived asymmetric key pair that identifies the server itself, not any user.

**How it works:**
- On first boot, `internal/presentation/ssh/hostkey.go` generates an Ed25519 key pair and saves it to disk at `SSH_HOST_KEY_PATH` with permissions `0600`
- On every subsequent boot, it loads the saved key
- The key is registered with the SSH library via `serverConfig.AddHostKey(hostSigner)` in `internal/presentation/ssh/server.go:102`
- The SSH library uses it automatically during the handshake — no application code needed

**Why it matters:** Without the host key, the SSH library refuses to start. The client uses it to verify it is talking to the real server (TOFU — Trust On First Use on first connection, then `~/.ssh/known_hosts` on subsequent connections).

**Scaling concern:** Each instance generates its own key. Multiple instances behind a load balancer will present different fingerprints to clients, causing scary "fingerprint changed" warnings. Fix: share the host key via a secrets manager (e.g. AWS Secrets Manager) or inject it as an environment variable at deploy time, so all instances use the same key.

**Key file:** `internal/presentation/ssh/hostkey.go`

---

## Concept 2: User Identity — SSH Public Key + Fingerprint

**What it is:** A returning user's SSH key can identify their durable account. Clients may also connect without a key, or with an unknown key, and receive a guest session.

**How it works:**
- The server allows SSH connections with no client key through a guest keyboard-interactive auth path
- When a public key is provided, the server accepts it for transport and preserves metadata in `ssh.Permissions`
- The server computes a short unique fingerprint from the public key: `ssh.FingerprintSHA256(key)`
- Known fingerprints are stored in `ssh_identities.public_key_fingerprint` in Postgres
- On reconnect with a known, non-revoked key, `ResolveSSHIdentityUseCase.Execute(ctx, input)` looks up the fingerprint → finds the linked user
- Unknown fingerprints return `ErrNotFound`; they do **not** auto-create `users` or `ssh_identities`
- Revoked known keys return `ErrSSHIdentityRevoked` and are not treated as durable identities
- No-key and unknown-key SSH connections start guest terminal sessions with no `user_id` or `ssh_identity_id`; unknown-key sessions keep the fingerprint on the terminal session for observability

**Why accept any key?** The SSH key is not an access gate. It is only a durable-account lookup hint. Unknown keys are deliberately guest-only so random SSH keys do not create persistent users or key bindings.

**Fingerprint vs public key:**
- Public key: long raw string (`ssh-ed25519 AAAA...`)
- Fingerprint: short hash (`SHA256:uNiVztksCsDhcc0u9e8Bz...`) — easier to store and compare

**Key files:** `internal/presentation/ssh/server.go:150-158`, `internal/domain/identity/identity.go`

---

## Concept 3: Login Code Flow

**What it is:** A short one-time code (like an OTP) that bridges the SSH terminal session to the browser login.

**States:**
```
PENDING   → generated, waiting for user to login in browser
COMPLETED → user logged in successfully
CANCELLED → expired or user cancelled
```

**Flow:**
1. User connects via SSH and lands on the home screen
2. User chooses Instamart
3. If login is required, server generates a random login code and stores its hash in Redis with a TTL (default 10 minutes, configured via `LOGIN_CODE_TTL`)
4. Server shows the user: `"Open swiggy.dev/login — Enter code: ABCD-1234"`
5. Server starts polling every 2 seconds
6. User opens browser, visits `/login`, enters the code
7. HTTP server marks the code as `COMPLETED` in Redis
8. Poll loop detects `COMPLETED` → session proceeds. For guest sessions this unlocks Instamart only for the current SSH session; it is repeated on every reconnect.

**Key files:** `internal/presentation/ssh/server.go`, `internal/application/auth/ensure_valid_account.go`, `internal/presentation/http/handlers.go`

---

## Concept 4: HTTP Login Page

**What it is:** A minimal web server with 3 routes that serves the browser-facing login form.

**Routes:**
```
GET  /health  → {"status":"ok"} — health check
GET  /login   → shows login form (enter code from terminal)
POST /login   → submits code → marks it COMPLETED in Redis → shows success page
```

**Current state:** The `/login` page accepts a code but does NOT yet redirect to Swiggy OAuth. Anyone who knows the code can complete login. The Swiggy OAuth integration needs to be added here.

**Planned flow (when Swiggy is integrated):**
```
User visits /login
      ↓
Redirect to Swiggy OAuth login page
      ↓
User logs in with Swiggy credentials
      ↓
Swiggy redirects back to /login with auth code
      ↓
Server exchanges code for access token
      ↓
Marks login code COMPLETED + saves token
```

**Key file:** `internal/presentation/http/handlers.go`

---

## Concept 5: OAuth Token Lifecycle & Re-authentication

**What it is:** After browser login, the server stores a Swiggy OAuth access token linked to the user. Tokens expire and must be refreshed via re-authentication.

**Token states:**
```
active              → valid, no action needed
expired             → token TTL passed, trigger re-auth
reconnect_required  → token invalidated externally, trigger re-auth
revoked             → hard block, user must contact support
```

**Re-auth flow (`EnsureValidAccountUseCase.Execute` in `auth/ensure_valid_account.go`):**
```
User connects
     ↓
Check token status
     ↓
active             → "Welcome back!"
expired/reconnect  → show new login code in terminal
                     user logs in browser again
                     new token saved → session continues
revoked            → "Access revoked. Contact support."
```

**Current state:** Tokens are mocked (`mock-token-<userID>`), expiring after 24 hours. Real Swiggy token refresh is not yet implemented.

**Key file:** `internal/application/auth/ensure_valid_account.go`

---

## Concept 6: Database Schema

Four linked tables in Postgres:

```
users
  id, display_name, email, created_at, last_seen_at

ssh_identities             (one user → many SSH keys)
  id, user_id → users.id
  public_key_fingerprint   ← unique, indexed, used for lookup
  public_key               ← full public key stored
  label                    ← e.g. "MacBook Pro"
  first_seen_at, last_seen_at
  revoked_at               ← set when key is revoked (e.g. lost laptop)

oauth_accounts             (one user → one account per provider)
  id, user_id → users.id
  provider                 ← e.g. "swiggy"
  provider_user_id
  encrypted_access_token   ← AES-256-GCM encrypted, never stored in plaintext
  token_expires_at, scopes, status

terminal_sessions          (one user → many sessions)
  id, user_id → users.id
  ssh_identity_id → ssh_identities.id
  ssh_fingerprint
  current_screen, selected_address_id
  created_at, last_seen_at, ended_at
```

**Key files:** `internal/infrastructure/persistence/postgres/migrations/000001_auth_identity.up.sql`, `internal/infrastructure/persistence/postgres/postgres.go`

---

## Concept 7: Token Encryption

**What it is:** OAuth access tokens are encrypted with AES-256-GCM before being written to the database, and decrypted in memory when needed.

**Why:** If the database is compromised, encrypted tokens are useless without the encryption key. The key is stored separately.

**How:**
- Encryption key: 32-byte key, base64url-encoded, set via `TOKEN_ENCRYPTION_KEY` environment variable
- Local dev fallback: `DefaultDevTokenEncryptionKey` in `config.go:21` — **publicly known, never use in production**
- Decrypt only happens in memory at runtime inside the Postgres persistence adapter

**Key files:** `internal/infrastructure/crypto/aes.go`, `internal/platform/config/config.go`

---

## Complete End-to-End Flow

```
1. ssh swiggy.dev
        ↓
2. SSH handshake
   - Client verifies server host key fingerprint (TOFU)
   - Encrypted tunnel established
        ↓
3. Server accepts SSH with or without a client public key
   - If a key is provided, computes its fingerprint
   - Looks up fingerprint in DB → known key resolves durable user identity
   - Unknown key or no key → guest session, no user/key rows created
        ↓
4. User chooses Instamart from the terminal home screen
        ↓
5. If login is required, server generates a login code (stored hashed in Redis, 10min TTL)
   - Shows user: "Open swiggy.dev/login — Enter code: ABCD-1234"
   - Starts polling Redis every 2 seconds
        ↓
6. User opens browser → visits swiggy.dev/login
   - Enters code (future: redirected to Swiggy OAuth first)
   - Code marked COMPLETED in Redis
        ↓
7. Poll loop detects COMPLETED
   - Known durable user: `EnsureValidAccountUseCase.Execute` checks/creates the stored OAuth account, and re-auths if expired
   - Guest: no OAuth account is written to Postgres; login completion applies only to this terminal session
        ↓
8. Terminal proceeds to the current Instamart placeholder
```

---

## What Is Not Yet Implemented

| Missing piece | Where to add it |
|---------------|----------------|
| Swiggy OAuth login redirect | `internal/presentation/http/handlers.go:handleLoginGet` |
| Swiggy OAuth callback handler | New route `GET /login/callback` in `internal/presentation/http/server.go` |
| Real Swiggy token exchange | `internal/infrastructure/provider/swiggy/client.go` — implement `Client` interface |
| Real token refresh on re-auth | `internal/application/auth/ensure_valid_account.go:refreshMockAccount` |

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SSH_ADDR` | `:2222` | SSH server listen address |
| `HTTP_ADDR` | `:8080` | HTTP server listen address |
| `SSH_HOST_KEY_PATH` | `.local/ssh_host_ed25519_key` | Path to host key file |
| `TOKEN_ENCRYPTION_KEY` | hardcoded dev key | AES-256 key (base64url, 32 bytes) — **must be set in production** |
| `DATABASE_URL` | `postgres://...localhost` | Postgres connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `PUBLIC_BASE_URL` | `http://localhost:8080` | Base URL shown to users in login prompt |
| `LOGIN_CODE_TTL` | `10m` | How long a login code is valid |
| `SWIGGY_PROVIDER` | `mock` | Set to `swiggy` when real integration is ready |
| `APP_ENV` | `local` | Environment name (`local`, `production`, etc.) |

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Multiple servers show different host key fingerprints | Share host key via secrets manager — all instances must use the same key |
| `TOKEN_ENCRYPTION_KEY` not set in production | Set to a securely generated 32-byte base64url key — the dev default is public |
| Login code accepted without Swiggy login | Swiggy OAuth is not yet integrated — `/login` currently accepts any code submission |
| User sees "fingerprint changed" warning | Old host key on disk differs from new one — delete `.local/ssh_host_ed25519_key` or restore the correct key |
| Token expired but re-auth not triggered | Check `ValidateTokenForUse` — token status must be `expired` or `reconnect_required` to trigger re-auth |
| Unknown SSH key creates a persistent user | This is no longer allowed — `ResolveSSHIdentityUseCase` must return `ErrNotFound`, and SSH presentation starts a guest session |
| Guest auth persists across reconnects | Guest login is session-only by design; users must reconnect with a known key for durable OAuth fast-path behavior |
