# Phase 3: TUI And SSH Flow

## Goal

Replace the Instamart placeholder with a real SSH TUI flow that calls the application service and enforces user-visible ordering steps.

No new dependencies are allowed without explicit approval.

Caveman version: screens only. User picks address, product, pack, qty, sees cart, confirms. TUI calls app service only.

## Files To Create Or Change

| Path | Work |
|---|---|
| `internal/presentation/tui/instamart.go` | Replace placeholder flow with real state machine. |
| `internal/presentation/tui/*_test.go` | Add thin TUI adapter tests only if useful. Business rules stay in application tests. |
| `internal/presentation/ssh/server.go` | Route authenticated users into real Instamart TUI. |
| `cmd/swiggy-ssh/main.go` | Pass Instamart service or TUI factory into SSH server. |

Suggested TUI file split for readability:

- `internal/presentation/tui/instamart.go` for entry view/model wiring
- `internal/presentation/tui/instamart_state.go` for state/action types
- `internal/presentation/tui/instamart_render.go` for render helpers
- `internal/presentation/tui/instamart_update.go` for Bubbletea update flow if it grows
- `internal/presentation/tui/instamart_test.go` or focused tests by state

Keep TUI files thin. Business rules still belong in application, not render/update helpers.

## TUI Dependency Shape

The TUI should receive an application service/facade interface, not infrastructure.

Acceptable shape:

```go
type InstamartService interface {
	GetAddresses(ctx context.Context) ([]instamart.Address, error)
	SearchProducts(ctx context.Context, input app.SearchProductsInput) (instamart.ProductSearchResult, error)
	GetGoToItems(ctx context.Context, input app.GetGoToItemsInput) (instamart.ProductSearchResult, error)
	GetCart(ctx context.Context) (instamart.Cart, error)
	UpdateCart(ctx context.Context, input app.UpdateCartInput) (instamart.Cart, error)
	Checkout(ctx context.Context, input app.CheckoutInput) (instamart.CheckoutResult, error)
	GetOrders(ctx context.Context, input app.GetOrdersInput) (instamart.OrderHistory, error)
	TrackOrder(ctx context.Context, input app.TrackOrderInput) (instamart.TrackingStatus, error)
}
```

Use the actual package names chosen in Phase 1.

Keep this interface in presentation and include only methods the TUI actually calls. Do not mirror future service methods preemptively.

## TUI States

- Address selection: load addresses, show summaries, require one choice.
- Keep selected address in TUI/application memory for v1. Do not update `terminal_sessions.selected_address_id` from TUI/SSH.
- Instamart home: search products, go-to items, view cart, track order, order history, change address.
- Search input: collect query and call app search with selected address.
- Go-to items: call app go-to items with selected address.
- Product list: show products and variations. Mark promoted products as featured/sponsored when `isPromoted=true`.
- Variant picker: require exact variation/pack and `spinId`.
- Quantity picker: choose quantity for exact variation.
- Cart edit: maintain intended cart from latest cart response.
- Cart review: show address summary, items, bill breakdown, total, payment methods.
- Checkout confirmation: ask for explicit confirmation immediately before checkout.
- Order result: show checkout message as returned.
- Order history: show recent/active order summaries.
- Tracking: call tracking only when hidden coordinates exist.
- Cancellation guidance: show support message and do not call provider.

## Cart State Rule

MCP `update_cart` replaces the whole cart.

The TUI must maintain an authoritative intended-cart list from the latest `get_cart` or `update_cart` response.

Every add, remove, and quantity change sends the full intended list to app `UpdateCart`.

Do not call update cart directly from a product row. Only call after variation and quantity are selected.

## Checkout UX

Before calling app `Checkout`, render:

- Address summary, redacted where possible.
- Cart items and quantities.
- Bill breakdown.
- Final total.
- Payment methods returned by cart.
- Multi-store warning when store count is greater than 1.

Then ask for explicit confirmation.

Pass the reviewed cart snapshot to app `Checkout` with `Confirmed true` only after the user confirms.

The app still calls fresh `GetCart` and enforces review match, confirmation, amount limit, payment method, and multi-store warning. The TUI is not the only safety layer.

## Tracking UX

Track order only when a hidden `Location` is available from cart/order/provider responses.

Never display coordinates.

If coordinates are missing, show a safe message such as:

```text
Tracking is unavailable for this order in the terminal. Please check the Swiggy Instamart app.
```

## Cancellation UX

For cancellation requests, do not call MCP.

Render exactly:

```text
To cancel your order, please call Swiggy customer care at 080-67466729.
```

## SSH Routing

In `internal/presentation/ssh/server.go`:

- Extend `SSHServer` with Instamart app service or TUI factory injected from `main.go`.
- Keep auth flow unchanged.
- After auth succeeds, render the real Instamart TUI.
- Keep guest/reauth/revoked behavior unchanged unless the auth owner changes it.
- Do not import infrastructure.
- Do not add DB writes from SSH for address/cart/order state.

## TUI Adapter Tests

These are not business-rule tests. They live beside the TUI package only because they verify presentation adapter behavior: rendering, key handling, and calls into a fake app service.

Keep them minimal. If a behavior can be tested in application, test it there instead.

Add tests with fake app service:

- Address list renders and selection is required before search.
- Search input calls service with selected address.
- Product results render variations with pack and price.
- Product results mark promoted products as featured/sponsored.
- Cart is not updated until exact variation is selected.
- Quantity change sends full intended cart list.
- Cart review renders address summary, items, bill lines, total, and payment methods.
- Checkout screen blocks without explicit confirmation.
- Checkout confirmation calls app with `Confirmed true`.
- Multi-store warning renders.
- Tracking without coordinates renders safe app message.
- Cancellation renders customer-care guidance and makes no provider call.
- Sensitive fields are not rendered.
- No DB write is attempted from TUI or SSH.

## Manual Smoke Flow

Full SSH smoke requires auth/session handoff to be merged first. Current SSH routing reaches Instamart only after auth succeeds. Do not add a test-only auth bypass in this phase.

If auth handoff is available, suggested local commands are:

```bash
SWIGGY_PROVIDER=mock make dev
ssh -p 2222 localhost
```

If auth handoff is not available, use TUI/application tests with a fake app service for the full flow and document that SSH smoke is blocked on auth handoff.

If the local host key/user setup differs, use the repo's existing SSH dev command and document the exact command in PR notes.

Use mock provider first:

1. Start app locally.
2. SSH into local server.
3. Select Instamart.
4. Select address.
5. Search for a product.
6. Select exact variation and quantity.
7. Review cart.
8. Confirm checkout below `1000` rupees.
9. View order result.

Only test real MCP after auth/session work is available and the user explicitly approves a real checkout path.

## Phase Done When

- `InstamartPlaceholderView` is no longer used for successful authenticated Instamart flow.
- TUI calls only application service/facade, not infrastructure.
- SSH imports no infrastructure packages.
- TUI tests pass.
- `go test ./...` passes.
