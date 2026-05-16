# Phase 1: Backend Domain And Application

## Goal

Create the client-agnostic Instamart backend shape. This phase should not call MCP and should not touch TUI routing beyond compile needs.

No new dependencies are allowed without explicit approval.

Caveman version: define grocery words, define provider door, add app rules, test rules.

## Files To Create Or Change

| Path | Work |
|---|---|
| `internal/domain/instamart/` | Add domain models, errors, provider port. |
| `internal/application/instamart/` | Add one application service/facade with safety gates. |
| `internal/application/instamart/*_test.go` | Add unit tests with hand-written fake provider. |

Suggested file split for readability:

- `internal/domain/instamart/models.go`
- `internal/domain/instamart/ports.go`
- `internal/domain/instamart/errors.go`
- `internal/application/instamart/service.go`
- `internal/application/instamart/service_test.go` or focused `*_test.go` files by behavior

Keep files cohesive. Do not create tiny files for every struct.

## Domain Models

Add only fields needed for v1 screens and safety checks:

- `Address`: ID, label/tag, redacted display line, redacted phone/category if useful. Area/city are optional and usually come later from cart address details, not initial `get_addresses`.
- `Location`: lat/lng for provider calls only. Never render or log it. Use pointer/validity checks in app inputs so missing coordinates are distinguishable from zero values.
- `Product`: display name, brand, stock flags, promoted/sponsored flag, product IDs, variations.
- `ProductVariation`: spin ID, pack/quantity description, display name, brand, price, stock flag, image URL.
- `Price`: MRP and offer/final price.
- `Cart`: selected address summary, hidden selected address location, items, bill breakdown, total, available payment methods, store IDs/count.
- `CartItem`: spin ID, name, quantity, store ID, stock flag, MRP/final price.
- `CartUpdateItem`: spin ID and quantity.
- `CartReviewSnapshot`: address ID, intended items, total rupees, and payment methods from the last cart review.
- `BillLine` and `BillBreakdown`: bill rows and final `ToPay`.
- `CheckoutResult`: message, order IDs/results, status, payment method, cart total, multi-store flag if useful.
- `OrderSummary`: order ID, status, item count, total, payment method, active flag, hidden delivery location if available.
- `OrderHistory`: orders and pagination flag if needed.
- `OrderHistoryQuery`: count, order type, active-only flag for the provider port.
- `TrackingStatus`: status message, ETA, items, polling interval.
- `ProductSearchResult`: products and next offset. MCP returns `nextOffset` as a string; decode as string or string/number tolerant, then convert only at app/TUI boundary if desired.

Use these concrete v1 field shapes unless implementation finds a compile conflict:

```go
type Address struct {
	ID          string
	Label       string
	DisplayLine string
	PhoneMasked string
	Category    string
}

type Location struct {
	Lat float64
	Lng float64
}

type Price struct {
	MRP        int
	OfferPrice int
}

type Product struct {
	ID              string
	ParentProductID string
	DisplayName     string
	Brand           string
	InStock         bool
	Available       bool
	Promoted        bool
	Variations      []ProductVariation
}

type ProductVariation struct {
	SpinID              string
	DisplayName         string
	Brand               string
	QuantityDescription string
	Price               Price
	InStock             bool
	ImageURL            string
}

type CartUpdateItem struct {
	SpinID   string
	Quantity int
}

type CartItem struct {
	SpinID     string
	Name       string
	Quantity   int
	StoreID    string
	InStock    bool
	MRP        int
	FinalPrice int
}

type BillLine struct {
	Label string
	Value string
}

type BillBreakdown struct {
	Lines       []BillLine
	ToPayLabel  string
	ToPayValue  string
	ToPayRupees int
}

type Cart struct {
	AddressID               string
	AddressLabel            string
	AddressDisplayLine      string
	AddressLocation         *Location
	Items                   []CartItem
	Bill                    BillBreakdown
	TotalRupees             int
	AvailablePaymentMethods []string
	StoreIDs                []string
}

type CartReviewSnapshot struct {
	AddressID     string
	Items         []CartUpdateItem
	ToPayRupees   int
	PaymentMethod string
}

type CheckoutResult struct {
	Message       string
	OrderIDs      []string
	Status        string
	PaymentMethod string
	CartTotal     int
	MultiStore    bool
}

type OrderSummary struct {
	OrderID       string
	Status        string
	ItemCount     int
	TotalRupees   int
	PaymentMethod string
	Active        bool
	Location      *Location
}

type OrderHistory struct {
	Orders  []OrderSummary
	HasMore bool
}

type OrderHistoryQuery struct {
	Count      int
	OrderType  string
	ActiveOnly bool
}

type TrackingStatus struct {
	OrderID                string
	StatusMessage          string
	SubStatusMessage       string
	ETAText                string
	ETAMinutes             int
	Items                  []CartItem
	PollingIntervalSeconds int
}

type ProductSearchResult struct {
	Products   []Product
	NextOffset string
}
```

## Domain Errors

Add sentinel errors in `internal/domain/instamart`:

- `ErrAddressRequired`
- `ErrVariantRequired`
- `ErrCartEmpty`
- `ErrCheckoutRequiresReview`
- `ErrCheckoutRequiresConfirmation`
- `ErrCheckoutAmountLimit`
- `ErrPaymentMethodUnavailable`
- `ErrTrackingLocationUnavailable`
- `ErrCancellationUnsupported`

## Provider Port

Define one provider interface in domain:

```go
type Provider interface {
	GetAddresses(ctx context.Context) ([]Address, error)
	SearchProducts(ctx context.Context, addressID, query string, offset int) (ProductSearchResult, error)
	YourGoToItems(ctx context.Context, addressID string, offset int) (ProductSearchResult, error)
	GetCart(ctx context.Context) (Cart, error)
	UpdateCart(ctx context.Context, selectedAddressID string, items []CartUpdateItem) (Cart, error)
	ClearCart(ctx context.Context) error
	Checkout(ctx context.Context, addressID, paymentMethod string) (CheckoutResult, error)
	GetOrders(ctx context.Context, input OrderHistoryQuery) (OrderHistory, error)
	TrackOrder(ctx context.Context, orderID string, location Location) (TrackingStatus, error)
}
```

`OrderHistoryQuery` should default to Instamart `orderType` value `DASH` when not set.

Set `activeOnly=true` only when the user explicitly asks for active/current/ongoing orders.

## Application Service

Create `internal/application/instamart.Service` or similarly named facade.

Use one service for v1 to avoid constructor sprawl. Split later only if the file becomes hard to reason about.

Expose simple methods:

- `GetAddresses(ctx)`
- `SearchProducts(ctx, input)`
- `GetGoToItems(ctx, input)`
- `GetCart(ctx)`
- `UpdateCart(ctx, input)`
- `ClearCart(ctx)`
- `Checkout(ctx, input)`
- `GetOrders(ctx, input)`
- `TrackOrder(ctx, input)`
- `HandleCancellation(ctx)` returning `ErrCancellationUnsupported`

Use these input shapes unless implementation finds an existing naming conflict:

```go
type SearchProductsInput struct {
	AddressID string
	Query     string
	Offset    int
}

type GetGoToItemsInput struct {
	AddressID string
	Offset    int
}

type UpdateCartInput struct {
	SelectedAddressID string
	Items             []instamart.CartUpdateItem
}

type CheckoutInput struct {
	AddressID      string
	PaymentMethod  string
	Confirmed      bool
	ReviewedCart   *instamart.CartReviewSnapshot
}

type GetOrdersInput struct {
	Count      int
	OrderType  string
	ActiveOnly bool
}

type TrackOrderInput struct {
	OrderID   string
	Location  *instamart.Location
}
```

## Application Rules

- Search and go-to items require selected address.
- Cart update requires selected address.
- Cart update requires exact selected variation via `spinId`.
- Cart update receives the complete intended cart list because MCP `update_cart` replaces the whole cart.
- Empty cart update is rejected unless the user explicitly chose clear cart.
- Checkout must call provider `GetCart` immediately before provider checkout.
- Checkout must return `ErrCheckoutRequiresReview` when no reviewed cart snapshot is provided.
- Checkout must compare the fresh cart to the reviewed cart snapshot. Address, items, total, and payment method must still match.
- Checkout input must include `Confirmed bool`.
- Checkout returns `ErrCheckoutRequiresConfirmation` when `Confirmed` is false.
- Checkout parses fresh cart total and returns `ErrCheckoutAmountLimit` for totals `>= 1000` rupees.
- Checkout rejects a payment method not returned by fresh cart.
- Checkout returns or marks a multi-store warning when store count is greater than 1.
- Tracking requires hidden coordinates and returns `ErrTrackingLocationUnavailable` if location is nil or invalid.
- Cancellation returns `ErrCancellationUnsupported` and never calls provider.

Checkout review match algorithm:

- Compare `ReviewedCart.AddressID` to fresh `Cart.AddressID`.
- Compare `ReviewedCart.PaymentMethod` to the selected payment method and require it in fresh `Cart.AvailablePaymentMethods`.
- Compare `ReviewedCart.ToPayRupees` to fresh `Cart.Bill.ToPayRupees` or `Cart.TotalRupees`.
- Compare items order-insensitively by `SpinID -> Quantity`.
- Ignore display-only fields like item name, bill labels, image URL, and address display line.
- If any check fails, return `ErrCheckoutRequiresReview` and do not call provider `Checkout`.

## Persistence Stance

- Do not add DB migrations in Phase 1.
- Do not add Instamart repositories in v1.
- Keep selected address, intended cart, reviewed cart, checkout result, and tracking location in memory.
- Existing `terminal_sessions.selected_address_id` stays untouched unless a later task adds a session update use case.
- Do not write audit events in v1.

## Tests

Add application unit tests with hand-written fake provider:

- Search rejects empty address ID.
- Go-to items rejects empty address ID.
- Update cart rejects empty address ID.
- Update cart rejects missing `spinId`.
- Update cart passes the full replacement item list to provider.
- Update cart rejects empty list unless clear cart is called.
- Checkout rejects missing confirmation.
- Checkout returns `ErrCheckoutRequiresReview` without reviewed cart snapshot.
- Checkout gets fresh cart before checkout and rejects cart changes after review.
- Checkout blocks total `>= 1000` rupees.
- Checkout rejects unavailable payment method.
- Checkout succeeds with returned payment method and total below limit.
- Checkout surfaces multi-store warning/result.
- Track order rejects nil/invalid location and does not call provider.
- Cancellation returns unsupported error and does not call provider.

## Phase Done When

- Domain package has no repo imports.
- Application package imports only stdlib plus `internal/domain/instamart`.
- Application tests pass with `go test ./internal/application/instamart`.
- `go test ./...` passes. Later phases must stay behind existing placeholders or mock wiring until implemented. Do not leave intentional compile or test failures.
