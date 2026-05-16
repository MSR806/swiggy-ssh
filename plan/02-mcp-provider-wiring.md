# Phase 2: MCP Provider And Wiring

## Goal

Implement the real Swiggy MCP `/im` adapter and wire it through config and `main.go` without breaking Clean Architecture boundaries.

No new dependencies are allowed without explicit approval.

Caveman version: make provider call MCP. Wire in main. No auth work. No TUI work.

## Files To Create Or Change

| Path | Work |
|---|---|
| `internal/infrastructure/provider/swiggy/` | Add `MCPInstamartClient` implementing `instamart.Provider`. |
| `internal/infrastructure/provider/mock/` | Add mock Instamart provider for local/dev tests and smoke flow. |
| `internal/platform/config/config.go` | Add `/im` endpoint config. |
| `cmd/swiggy-ssh/main.go` | Wire provider and application service. |
| `internal/infrastructure/provider/swiggy/*_test.go` | Add adapter tests with `httptest.Server`. |

Suggested file split for readability:

- `internal/infrastructure/provider/swiggy/instamart_client.go`
- `internal/infrastructure/provider/swiggy/instamart_mcp.go` for JSON-RPC request/response helpers
- `internal/infrastructure/provider/swiggy/instamart_mapping.go` for MCP-to-domain mapping
- `internal/infrastructure/provider/swiggy/instamart_client_test.go` or focused adapter tests
- `internal/infrastructure/provider/mock/instamart.go`

Keep the public adapter surface small. Helper files should stay package-private.

## Config

Add:

```text
SWIGGY_MCP_IM_ENDPOINT=https://mcp.swiggy.com/im
```

Keep `SWIGGY_PROVIDER` for mock vs real selection if useful.

Use exact provider behavior for v1:

- Default `SWIGGY_PROVIDER=mock`.
- Accepted values: `mock`, `mcp`.
- Unknown value: fail fast at startup with a clear config error.
- `mock`: wire mock provider only. No real MCP calls.
- `mcp`: require a non-nil `RequestAuthorizer` from auth handoff. If auth handoff is unavailable, fail fast with `SWIGGY_PROVIDER=mcp requires MCP request authorizer from auth handoff`.
- Do not construct an unauthenticated real MCP client.

Do not import config from application or domain.

## MCP Adapter

Add a concrete adapter such as:

```go
type MCPInstamartClient struct {
	endpoint string
	httpClient *http.Client
	// request authorizer from separate auth/MCP-session work goes here
}
```

Constructor shape can be:

```go
func NewMCPInstamartClient(endpoint string, httpClient *http.Client, authorizer RequestAuthorizer) *MCPInstamartClient
```

Use stdlib `net/http`, `encoding/json`, and `httptest`. Do not add an MCP SDK or other dependency without explicit approval.

## Auth Handoff Boundary

Do not implement token lookup, token refresh, OAuth account loading, login-code flow, cookie construction, or MCP session creation in this phase.

The adapter may depend only on a minimal injected request authorizer supplied by the auth/MCP-session owner:

```go
type RequestAuthorizer interface {
	AuthorizeMCPRequest(ctx context.Context, req *http.Request) error
}
```

`MCPInstamartClient` should call this before sending requests. Tests should use a fake authorizer. If production auth wiring is not available yet, wire only the mock provider in `main.go` and leave real-provider construction behind a clear `TODO(auth handoff)` or startup error. Do not read tokens directly from persistence in this adapter.

## MCP Rules

- POST to `https://mcp.swiggy.com/im` by default.
- Use JSON-RPC method `tools/call`.
- Use unqualified tool names.
- `get_addresses` has no arguments.
- `search_products` arguments are `addressId`, `query`, `offset`.
- `your_go_to_items` arguments are `addressId`, `offset`.
- `get_cart` has no arguments.
- `update_cart` arguments are `selectedAddressId` and `items` with `spinId`, `quantity`.
- `clear_cart` has no arguments.
- `checkout` arguments are `addressId`, `paymentMethod`.
- `get_orders` should send/default `orderType: "DASH"`; include `count` and `activeOnly` as needed.
- `track_order` arguments are `orderId`, `lat`, `lng`.
- `report_error` is out of scope for v1.

## Response Mapping

Decode the JSON-RPC response first:

```go
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *jsonRPCError   `json:"error"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type mcpToolResult struct {
	Content []mcpToolContent `json:"content"`
	IsError bool             `json:"isError,omitempty"`
}

type mcpToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}
```

Unwrap in this order:

1. If JSON-RPC `error` is present, return a safe adapter error.
2. Try to decode `result` directly as the Instamart tool envelope.
3. If direct decode does not yield an envelope, decode `result` as `mcpToolResult`.
4. Find the first content item with `type == "text"` and JSON text.
5. Decode that text into the Instamart tool envelope.
6. If tool envelope `success == false`, return a safe adapter error using `error.message`.

Instamart tool envelope:

```go
type instamartToolEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error"`
}
```

Adapter tests must cover direct `result`, text-content `result`, JSON-RPC error, and `success:false` tool error. No real MCP call should be needed to understand response unwrapping.

Map MCP responses into domain models:

- Addresses from `data.addresses[]`: `id`, `addressLine`, `phoneNumber`, `addressCategory`, `addressTag`. Do not assume area/city on the initial address list.
- Products from `data.products[]`, including `isPromoted` when present.
- Variations from `products[].variations[]`.
- Search/go-to `data.nextOffset` is string-shaped in observed MCP responses. Decode as string or tolerate string/number.
- Cart selected address details into safe summary plus hidden location.
- Cart items, bill breakdown, total amount, payment methods, and store IDs.
- Checkout message as-is, status, payment method, cart total.
- Checkout must support single-order responses and multi-store per-order result arrays. Report each order result separately.
- Orders and active/history metadata.
- Tracking status, ETA, items, polling interval.

## Sensitive Data

- Never log access tokens.
- Never log full phone numbers.
- Never log full addresses.
- Never log precise coordinates.
- Never log real order IDs unless there is a protected audit/logging requirement.
- Store coordinates only in memory/domain structs for provider calls.

## Error Mapping

- MCP `success: false` should become safe domain/application-facing errors.
- Preserve enough tool/error context for debugging in tests.
- Do not leak sensitive request or response fields in error strings.

## Main Wiring

In `cmd/swiggy-ssh/main.go`:

1. Load `SWIGGY_MCP_IM_ENDPOINT`.
2. Create mock Instamart provider when `SWIGGY_PROVIDER=mock`.
3. Create `MCPInstamartClient` when real provider is selected.
4. Create `application/instamart.Service` with the selected provider.
5. Pass the service or TUI factory into `sshserver.New(...)`.

Presentation packages must not import `internal/infrastructure/provider/swiggy`.

## Mock Provider

Add a deterministic mock provider in `internal/infrastructure/provider/mock` implementing `instamart.Provider`.

It must support the complete local smoke flow:

- Saved addresses.
- Product search and go-to items with variations.
- Full-cart replacement on `UpdateCart`.
- `GetCart` with bill breakdown and `Cash` payment method.
- Checkout result below `1000` rupees without calling real MCP.
- Active/history orders and tracking response.

Keep mock data fake. No real addresses, phone numbers, tokens, coordinates, or order IDs.

## Adapter Tests

Use `httptest.Server`. Do not hit real Swiggy in tests.

Add tests for:

- Client posts to configured endpoint path.
- Request uses JSON-RPC method `tools/call`.
- Response unwrap tests cover direct `result`, text `result.content[]`, JSON-RPC error, and tool `success:false`.
- Tool names are unqualified.
- `search_products` request body matches skill shape.
- `update_cart` request body sends full `items[]`.
- `checkout` request body sends `addressId` and `paymentMethod` exactly as provided.
- Application tests verify the payment method was returned by fresh `GetCart`.
- `get_orders` sends/defaults `orderType: "DASH"`.
- `track_order` sends `orderId`, `lat`, `lng`.
- Address response maps to domain address summaries.
- Product variations map to `spinId`, pack, price, and stock fields.
- Search response maps `nextOffset` string correctly.
- Promoted products map to the domain promoted/sponsored flag.
- Cart maps bill lines, total, payment methods, store IDs, and hidden location.
- Checkout maps message as-is.
- MCP errors map to safe errors.

## Phase Done When

- Infrastructure imports domain but not presentation.
- Presentation still does not import infrastructure.
- `cmd/swiggy-ssh/main.go` is the only concrete wiring point.
- Adapter unit tests pass without external services.
- Mock provider supports local flow without external services.
- `go test ./...` passes.
