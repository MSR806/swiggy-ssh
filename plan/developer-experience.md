# Developer Experience Plan

Make `ssh swiggy.dev` feel like a tiny dev tool that orders groceries.

Not grocery app in terminal.
Dev console for Instamart.

## What Code Already Has

Current app already has the big bones:

- SSH login with public key identity.
- Browser auth for Swiggy account link.
- Home screen.
- Instamart address select.
- Product search.
- Go-to items.
- Cart review.
- Checkout confirm.
- Order history.
- Order tracking.

So plan must be small.
No giant rewrite.
No new backend first.
Make current screens feel developer-native.

## Product Feeling

User should feel:

```text
I ssh into machine.
I grep groceries.
I stage items.
I review diff.
I ship order.
I watch logs.
```

Simple. Funny. Useful.

## Tight Scope

Build these first:

- Better words.
- Better shortcuts.
- Better status line.
- Cart diff before checkout.
- Log-style order result.
- Help screen.

Do not build these first:

- Full shell parser.
- JSON mode.
- Command history.
- Config files.
- Plugin system.
- Fancy telemetry.
- New backend concepts.

## V1 Changes

### 1. Rename Screen Copy

Use dev words, but do not confuse user.

Keep real action obvious.

| Current | Better |
|---|---|
| Search products | grep products |
| Your go-to items | recent cache |
| View cart | staged cart |
| Track active order | tail active order |
| Order history | deploy history |
| Change address | switch target address |
| Review cart | review staged cart |
| Confirm checkout | ship order |
| Order result | deploy logs |
| Delivering to | deploying to |

Good copy examples:

```text
grep products
type query, press enter
```

```text
staged cart
items ready to ship
```

```text
ship order
press y to deploy groceries
```

Bad copy examples:

```text
symlink bananas into production namespace
```

Too clever. User gets tired.

### 2. Add Tiny Session Status Bar

Every Instamart screen should show one small status row.

Example:

```text
env=instamart  auth=ok  addr=Home  cart=3  mode=grep
```

Rules:

- One line only.
- No secrets.
- No full address.
- No token info.
- Use current screen as `mode`.

Why good:

- Feels like `kubectl`, `docker`, `git status`.
- Helps user know where they are.
- Cheap to build in TUI rendering.

### 3. Make Cart Review Look Like Diff

Before checkout, show cart like a diff.

Example:

```diff
+ 2x Amul Milk 1L                 Rs 144
+ 1x Banana Robusta               Rs 54
+ Delivery fee                    Rs 25
+ To pay                          Rs 223
```

If cart empty:

```text
working tree clean. cart empty.
```

If multi-store:

```text
warn: cart spans 2 stores. Swiggy may split deploy.
```

Keep payment methods visible.
Checkout still needs explicit `y`.

### 4. Make Search Results Look Like Status Table

Search should feel like `grep` plus `kubectl get`.

Use table for scan speed.
Do not use YAML for many items.

Example:

```text
grep products: milk

  #   code  item                         pack      price
> 1   200   Amul Taaza Milk              1 L       Rs 72
  2   200   Nandini Toned Milk           500 ml    Rs 28
  3   200   Akshayakalpa Organic Milk    1 L       Rs 96
  4   409   Amul Gold Milk               1 L       out of stock
```

Rules:

- `200` means available.
- `409` means unavailable or out of stock.
- Keep item name, pack, and price aligned.
- Cursor stays on selected row.
- User presses enter to choose exact item.

### 5. Make Selected Item Look Like YAML Manifest

After user chooses item, quantity screen can look YAML-ish.

Use this only for one item.
Many items should stay table.

Example:

```yaml
item: Amul Taaza Milk
pack: 1 L
price: Rs 72
status: 200 available
action: stage item
quantity: 2
```

Why good:

- Feels like editing a deploy manifest.
- Easy to read.
- Fits the staging/cart metaphor.

### 6. Make Checkout Feel Like Deploy

Confirmation screen should look like a deploy gate.
Checkout joke should be reckless on purpose:

```text
git push --force groceries
```

But do not hide real meaning.
Always say this confirms the Instamart order.

Example:

```text
ship order

[ok] address selected
[ok] cart reviewed
[ok] payment method available
[ok] amount below test limit

deploying to: Home
payment: COD
total: Rs 223

press y to confirm order
aka git push --force groceries
```

Important:

- Still say this is an Instamart order.
- Still show address label and masked address line.
- Still show payment method.
- Still block without user confirmation.
- Use `git push --force`, not `--force-with-lease`. Joke should feel reckless.

### 7. Show Receipt As Logs

After checkout, show result as logs.

Example:

```text
deploy logs

[ok] git push --force origin groceries
[ok] Swiggy Instamart order placed successfully
[ok] payment method: COD
[ok] status: confirmed
[info] order_id=123456789
[info] stores=1
```

If error:

```text
[fail] checkout blocked
[hint] review cart again, then retry ship
```

Do not hide real error.
Make it readable.

### 8. Add Help Screen

Add one help screen reachable from `?`.

Keep it tiny.

Example:

```text
swiggy.dev keys

j/k        move
/          grep products
c          staged cart
enter      choose
+/-        change quantity
p          ship from cart
b          back home
q          quit
```

Why now:

- Current TUI already has key hints.
- Help screen makes shortcuts discoverable.
- No backend needed.

## Search Input Bug Plan

Current search has three UX bugs:

- No blinking cursor while typing.
- Space key does not register reliably.
- Search only runs after enter, so user cannot preview results while typing.

Fix this before making search pretty.

### 1. Show Visible Cursor

Search input should render a cursor after the query.

Example:

```text
grep products

query: milk tea█
```

Rules:

- Cursor must be visible even when query is empty.
- Cursor can be simple block `█` or underscore `_`.
- Blinking is nice, but visible cursor is required.
- Do not add new dependency just for cursor.

Good enough V1:

```text
query: █
```

Better later:

```text
query: milk█
```

### 2. Make Space Key Work

Search must accept multi-word queries.

Examples:

```text
milk tea
amul dark chocolate
brown bread
```

Likely fix:

- Keep current manual input handling.
- Append `" "` when key is `space` or literal space.
- Keep existing rune handling for letters, numbers, and symbols.
- Backspace should remove one rune, including spaces.

Acceptance:

```text
query: amul milk
```

must stay exactly `amul milk`, not `amulmilk`.

### 3. Add Debounced Live Preview

While user types, show live results on same search screen.

Do not move to product list automatically.

Enter is the commit action.

Flow:

```text
type query -> debounce -> show preview -> user keeps typing
enter -> open actual product list with current preview
```

Example:

```text
grep products: milk

query: milk█

preview
  #   code  item                         pack      price
> 1   200   Amul Taaza Milk              1 L       Rs 72
  2   200   Nandini Toned Milk           500 ml    Rs 28
  3   409   Amul Gold Milk               1 L       out of stock

enter open results    esc home
```

Rules:

- Debounce around `300ms` to `400ms`.
- Do not search for empty query.
- Maybe wait until query has at least `2` characters.
- Show spinner plus `scanning index...` while preview request is running.
- If user types again, older result must not overwrite newer result.
- Use query/version check to ignore stale responses.

Loader copy:

```text
grep products: milk

query: milk█

⠋ scanning index...
```

Do not say `searching...`.
Search language is `grep`.
Loader language is `scanning index`.

### 4. Enter Opens Real Results

Enter should not start a brand new surprise flow if preview exists.

It should commit current query.

Behavior:

- If preview exists for current query, enter opens product list using preview data.
- If preview is still loading, enter can show loading then open result.
- If no preview exists yet, enter runs search once and opens result.
- If query is empty, show error.

This gives user control:

```text
live preview is for looking
enter is for choosing
```

### 5. Technical Shape

Keep changes in TUI layer.

Likely model fields:

```go
searchQuery string
searchPreviewQuery string
searchPreviewRows []productVariationRow
searchPreviewLoading bool
searchPreviewVersion int
```

Likely new messages:

```go
type searchDebounceMsg struct {
    query string
    version int
}

type searchPreviewMsg struct {
    query string
    version int
    result domaininstamart.ProductSearchResult
    err error
}
```

Important:

- No domain change.
- No provider change.
- No new dependency.
- No command parser.
- Keep product list screen for final committed results.

## Implementation Map

Mostly presentation layer.

Touch these files first:

- `internal/presentation/tui/instamartflow/screen_home.go`
- `internal/presentation/tui/instamartflow/screen_products.go`
- `internal/presentation/tui/instamartflow/screen_cart.go`
- `internal/presentation/tui/instamartflow/screen_orders.go`
- `internal/presentation/tui/instamartflow/view.go`
- `internal/presentation/tui/instamartflow/flow.go`

Maybe touch tests:

- `internal/presentation/tui/instamartflow/flow_test.go`
- `internal/presentation/tui/tui_test.go`

Do not touch domain for V1.
Do not touch provider for V1.
Do not touch checkout rules for V1.

## Build Order

1. Change labels and empty states.
2. Add status bar helper.
3. Render search results as status table.
4. Render selected item as YAML manifest.
5. Render cart review as diff.
6. Render checkout confirm as deploy gate.
7. Render checkout result as logs.
8. Add `?` help screen.
9. Update tests for new copy.

## Acceptance Criteria

V1 is done when:

- Home still works.
- Address select still works.
- Product search still works.
- Quantity update still works.
- Cart review still shows items, bill, payment methods, and address.
- Checkout still requires explicit confirmation.
- Order success still shows Swiggy response message.
- Cancel guidance still shows Swiggy customer care number.
- `go test ./...` passes.

## One-Line Goal

Make the current Instamart TUI feel like this:

```text
git add snacks && git push --force groceries
```

But keep the code boring and safe.
