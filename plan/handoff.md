# Instamart V1 Handoff

## Goal

Build the Instamart grocery flow for `ssh swiggy.dev` using Swiggy MCP `/im` tools as the backend API.

The next agent should implement this in phases, starting with backend domain/application code, then MCP provider/wiring, then TUI/SSH flow.

Caveman version: make groceries work. Backend first. MCP adapter next. TUI last. Do not build auth.

## Start Here

Read these files first:

| File | Why |
|---|---|
| `.agents/skills/instamart/SKILL.md` | Authoritative MCP `/im` workflow, tool names, request shapes, and safety rules. |
| `.agents/skills/swiggy-ssh-clean-architecture/SKILL.md` | Architecture boundaries and package placement. |
| `internal/presentation/tui/instamart.go` | Current placeholder Instamart TUI. |
| `internal/presentation/ssh/server.go` | Auth flow and current route to `InstamartPlaceholderView`. |
| `internal/domain/instamart/service.go` | Current placeholder Instamart domain service. |
| `internal/infrastructure/provider/swiggy/client.go` | Current placeholder Swiggy provider contract. |
| `cmd/swiggy-ssh/main.go` | Only composition root for concrete wiring. |

Load the `swiggy-ssh-clean-architecture` skill before changing packages, boundaries, provider placement, or wiring.

## Scope

- Implement Instamart only.
- Use MCP endpoint `POST https://mcp.swiggy.com/im`.
- Use these v1 `/im` tools: `get_addresses`, `search_products`, `your_go_to_items`, `get_cart`, `update_cart`, `clear_cart`, `checkout`, `get_orders`, `track_order`.
- `report_error` exists in `/im`, but is out of v1 scope.
- Keep v1 small: one domain provider port, one application service/facade, one MCP adapter, one TUI flow.
- Use `go test ./...` as the final verification command.

## Non-Goals

- Do not implement auth/token/session setup here. Another dev owns it.
- Do not implement Food or Dineout.
- Do not implement `report_error` in v1 unless explicitly requested.
- Do not persist a local cart unless a later requirement needs it.
- Do not add DB migrations for v1 unless a later requirement needs persisted Instamart state.
- Do not add audit logging writes in v1.
- Do not add new dependencies without explicit approval.
- Do not test a real checkout path unless the user explicitly approves it.

## Current State

- `internal/presentation/tui/instamart.go` renders a static placeholder menu with fake address/cart values.
- `internal/presentation/ssh/server.go` sends authenticated users to `InstamartPlaceholderView`.
- `internal/domain/instamart/service.go` only defines `Health(ctx)`.
- `internal/infrastructure/provider/swiggy/client.go` only defines `Ping(ctx)`.
- `cmd/swiggy-ssh/main.go` does not wire an Instamart provider or app service.
- `internal/platform/config/config.go` has `SWIGGY_PROVIDER`, but no MCP `/im` endpoint config.
- DB already has `terminal_sessions.selected_address_id`, but there is no update use case. Leave it alone for v1.
- Current SSH route reaches Instamart only after auth succeeds. Do not add an auth bypass in this work.

## DB Stance

- Keep selected address, intended cart, reviewed cart, checkout result, and tracking location in TUI/application memory for v1.
- No new Instamart tables.
- No cart/order persistence.
- No audit writes.
- If selected address persistence is needed later, add an identity/session application use case and Postgres method. Do not write DB code from TUI or SSH.

## Phase Order

1. `plan/01-backend-domain-application.md`: domain models, provider port, application service/facade, safety gates, unit tests.
2. `plan/02-mcp-provider-wiring.md`: MCP adapter, config, mock provider, main wiring, adapter tests.
3. `plan/03-tui-ssh-flow.md`: real TUI state flow, SSH route, presentation tests.

First implementation step: complete Phase 1 only. Start by replacing the placeholder `internal/domain/instamart/service.go` with domain models/errors/provider port, then add `internal/application/instamart` and its unit tests. Do not touch MCP, auth, SSH routing, or TUI except for compile-preserving placeholder adjustments during Phase 1.

## Hard Safety Gates

- `update_cart` replaces the whole Instamart cart. Always send the full intended cart list.
- Require exact product variation and `spinId` before any cart update.
- Call `get_cart` after cart mutation and before checkout.
- Before checkout, show address summary, items, bill breakdown, total, and returned payment methods.
- Checkout must require explicit confirmation inside application code, not only in TUI code.
- Block checkout when fresh cart total is `>= 1000` rupees.
- Use only payment methods returned by `get_cart`.
- Warn when cart has items from multiple stores.
- `track_order` requires hidden coordinates. If coordinates are unavailable, do not call MCP.
- Cancellation never calls MCP. Show: `To cancel your order, please call Swiggy customer care at 080-67466729.`
- Never log tokens, full phone numbers, full addresses, precise coordinates, or real order IDs.
- Render only the minimum order reference needed for user confirmation/tracking. Do not put order IDs in error strings.

## Architecture Rules

- Domain imports stdlib only.
- Application imports domain only.
- Presentation imports application/domain, never infrastructure.
- Infrastructure imports domain, never presentation.
- `cmd/swiggy-ssh/main.go` is the only place that wires concrete adapters.
- Prefer multiple small, cohesive files when it improves readability. Do not dump all domain models, service methods, adapter mapping, or TUI states into one giant file.
- Do not split just to split. Group by concept: models, ports, errors, service, MCP requests, MCP mapping, TUI model, TUI rendering, tests.
- Test business rules in application tests. Presentation-package tests are only for adapter behavior like rendering, key handling, and service-call sequencing with fake services.

## Completion Criteria

- Instamart TUI can select address, search products, choose exact variation, update cart, review cart, confirm checkout, and show order result.
- Mock provider supports the full local smoke flow without real MCP or real checkout.
- If separate auth handoff is not merged yet, verify the full flow with TUI/application tests and a fake app service; full SSH smoke waits for auth handoff.
- App layer enforces cart and checkout safety gates even if TUI has a bug.
- MCP adapter sends correct `/im` MCP `tools/call` requests.
- Unit tests cover application gates, MCP request/response mapping, and TUI sequencing.
- `go test ./...` passes without external services.
