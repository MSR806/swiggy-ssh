---
name: food
description: Use when working with Swiggy MCP /food tools, food delivery restaurant search, menus, food cart, coupons, order placement, history, or tracking flows.
---

# Swiggy MCP `/food` Tool Usage

Generated and verified on 2026-05-16 against `https://mcp.swiggy.com/food`.

This file covers Swiggy Food delivery only. Tool names are unqualified in MCP requests: POST to `/food`, then call tool name `search_restaurants`, not `food.search_restaurants`.

## MCP Assumption

Connection and session setup is out of scope for this skill. Assume the caller already has a working MCP client wired to:

```text
POST https://mcp.swiggy.com/food
```

This skill only documents tool names, arguments, workflows, response shapes, and safety gates.

List tools:

```json
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
```

Call a tool:

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "search_restaurants",
    "arguments": {
      "addressId": "<addressId>",
      "query": "pizza",
      "offset": 0
    }
  }
}
```

## Workflow Tree

1. `get_addresses`
2. Select `addressId`
3. `search_restaurants` with `addressId`, `query`, optional `offset`
4. Pick an `OPEN` restaurant and persist `restaurantId` and `restaurantName`
5. Browse with `get_restaurant_menu` or search a dish with `search_menu`
6. For customized items, choose variants/addons from `search_menu`
7. `update_food_cart`
8. `get_food_cart`
9. Optional: `fetch_food_coupons` then `apply_food_coupon` if a valid coupon exists
10. Show full cart, available payment methods, and delivery address
11. Require explicit confirmation
12. `place_food_order`
13. `track_food_order` and/or `get_food_orders`

Important: each food menu item uses either legacy `variants` or `variantsV2`. Use the same format returned by `search_menu`; do not mix both.

Addon flow for customized items:

1. Add item with variants only.
2. Call `get_food_cart`.
3. Read `valid_addons` for the chosen variant.
4. Add only valid addon choices and respect min/max constraints.
5. Call `get_food_cart` again immediately after the addon update so the caller has the real cart state.

## Tool Catalog

| Tool | Description | Required args | Optional args | How to use |
|---|---|---|---|---|
| `get_addresses` | Gets saved delivery addresses. | - | - | First call for Food delivery flows. |
| `search_restaurants` | Searches restaurants for delivery. | `addressId`, `query` | `offset` | Persist `restaurantId`, `restaurantName`, and availability. |
| `search_menu` | Searches menu items/dishes and returns customization ids. | `addressId`, `query` | `restaurantIdOfAddedItem`, `vegFilter`, `offset` | Use before adding specific dishes; returns variant/addon ids. |
| `get_restaurant_menu` | Browses restaurant menu by category page. | `addressId`, `restaurantId` | `page`, `pageSize` | Use for discovery and pagination; use `search_menu` before cart add. |
| `get_food_cart` | Gets current Food cart and payment methods. | `addressId` | `restaurantName` | Call after cart mutation and before `place_food_order`. |
| `update_food_cart` | Adds/updates cart items, variants, and addons. | `restaurantId`, `cartItems`, `addressId` | `restaurantName` | `cartItems[]` requires `menu_item_id`, `quantity`; variants/addons depend on `search_menu`. |
| `flush_food_cart` | Clears the Food cart. | - | - | Use for cleanup or explicit empty-cart action. |
| `place_food_order` | Places a real Food delivery order. | `addressId` | `paymentMethod` | Only after `get_food_cart`, address/payment/total display, and explicit confirmation. |
| `fetch_food_coupons` | Fetches coupons/offers for a restaurant/address. | `restaurantId`, `addressId` | `couponCode` | Use before checkout; only recommend COD-compatible coupons. |
| `apply_food_coupon` | Applies a coupon to Food cart. | `couponCode`, `addressId` | `cartId` | Use only with a valid coupon code. |
| `get_food_orders` | Gets Food order history or active orders. | `addressId` | `activeOnly` | Use for recent/past/current Food orders. |
| `get_food_order_details` | Gets detailed Food order information. | `orderId` | - | Use with an order id from `get_food_orders`. |
| `track_food_order` | Tracks active Food delivery orders. | - | `orderId` | Use for active/in-progress Food order ETA/status. |
| `report_error` | Generates MCP team error report. | `tool`, `errorMessage` | `domain`, `flowDescription`, `toolContext`, `userNotes` | Include IDs like `orderId`, `restaurantId`, `couponCode`, `menu_item_id`. |

## Real Response Examples

### `search_restaurants`

Call used:

```json
{
  "addressId": "<addressId>",
  "query": "pizza",
  "offset": 0
}
```

Observed response text excerpt:

```text
Found 10 restaurants for "pizza":
1. Olio - The Wood Fired Pizzeria (Ad) — Pizzas, Pastas, Italian, Fast Food, Snacks, Beverages, Desserts | 4.1★ | 43 min | ₹300 for two (ID: 603191)
2. La Pino'z Pizza (Ad) — Pizzas, Pastas, Italian, Desserts, Beverages | 4.1★ | 34 min | ₹250 for two (ID: 211192)
...
8. Domino's Pizza (Ad) — Pizzas, Italian, Pastas, Desserts | 4.5★ | 20 min | ₹400 for two (ID: 23788)
```

Persist restaurant id, name, ETA, distance if present, and availability status. Do not proceed with closed/unavailable restaurants.

### `get_restaurant_menu`

Call used:

```json
{
  "addressId": "<addressId>",
  "restaurantId": "23788",
  "page": 1,
  "pageSize": 3
}
```

Observed response text excerpt:

```text
Menu for Domino's Pizza (ID: 23788)
Page 1 — 3 of 29 categories. Use page=2 for more.

## Minimum 50% off
  - Onion Fresh Pan — ₹99 | Veg, has variants (ID: 192044804)
...
## Recommended
  - Chicken Dominator Pizza — ₹369 | Non-veg, has variants, has addons (ID: 25933457)
  - Veg Extravaganza Pizza — ₹319 | Veg, has variants, has addons (ID: 110240705)
```

Use `page`/`pageSize` for category pagination. To add an item, use `search_menu` for full customization details.

### `search_menu`

Call used:

```json
{
  "addressId": "<addressId>",
  "query": "margherita pizza",
  "restaurantIdOfAddedItem": "23788",
  "vegFilter": 1,
  "offset": 0
}
```

Observed response text excerpt:

```text
Found 10 menu items for "margherita pizza":
1. Margherita Pizza Regular — ₹109 | Veg | 4.6★ (81) (ID: 163982423)
2. Margherita Pizza — ₹109 | Veg | 4.6★ (2.2K+) (ID: 17857751)
   Variants (Crust): [New Hand Tossed (group:36843806, var:114791995), ...]
   Variants (Size): [Regular (group:36843808, var:114792005), Medium (...), Large (...)]
   Addons (Extra Cheese Regular): [Extra Cheese ₹50 (group:132388512, choice:100001287)]
```

Persist `menu_item_id`, variant `group_id`/`variation_id`, and addon `group_id`/`choice_id`.

### `update_food_cart`

Call used with a simple item without variants/addons:

```json
{
  "restaurantId": "23788",
  "restaurantName": "Domino's Pizza",
  "addressId": "<addressId>",
  "cartItems": [
    {
      "menu_item_id": "163982423",
      "quantity": 1,
      "variants": [],
      "variantsV2": [],
      "addons": []
    }
  ]
}
```

Observed response text:

```text
Cart updated.
Restaurant: Domino's Pizza
Items (1):
  - Margherita Pizza Regular — ₹109 (ID: 163982423)

Item total: ₹109
Delivery: FREE
Taxes & charges: ₹51.38
TO PAY: ₹160
```

### `get_food_cart`

Observed response text:

```text
Restaurant: Domino's Pizza
Items (1):
  - Margherita Pizza Regular — ₹109 (ID: 163982423)

Item total: ₹109
Delivery: FREE
Taxes & charges: ₹51.38
TO PAY: ₹160

Payment methods: Cash
```

Checkout precondition: display items, totals, payment methods, and delivery address before `place_food_order`.

### `fetch_food_coupons`

Call used:

```json
{
  "restaurantId": "23788",
  "addressId": "<addressId>",
  "couponCode": ""
}
```

Observed response:

```text
Found 0 coupons (0 applicable):

To apply a coupon, use apply_food_coupon with the coupon code and addressId.
```

### `get_food_orders`

Observed response text excerpt:

```text
Found 5 orders:
1. Order <orderId> — Popeyes | Delivered | ₹720
2. Order <orderId> — Subway | Delivered | ₹512
...
```

### `get_food_order_details`

Observed response:

```text
Retrieved details for order <orderId> - Delivered (Popeyes)
```

### `track_food_order`

Observed response:

```text
Tracking 1 active order
```

### `flush_food_cart`

Observed response:

```text
Flushed Food cart successfully
```

## Tools Not Executed And Why

| Tool | Reason |
|---|---|
| `apply_food_coupon` | `fetch_food_coupons` returned zero applicable coupons for the tested restaurant/address. |
| `place_food_order` | Would place a real Food order; no Food order confirmation was requested. |
| `report_error` | Only for reporting failures to Swiggy MCP team; no report was requested. |

## Safety Rules

- Never store or log full phone numbers, full addresses, precise coordinates, or real order IDs unless required and protected.
- `place_food_order` creates a real Food delivery order. Call it only after the user has seen final address, items, bill total, and payment method, then explicitly confirms.
- Only use payment methods returned by `get_food_cart`.
- Do not place Food orders with cart value `₹1000` or more; direct the user to the Swiggy app.
- For cancellation requests, do not call MCP tools. Use Swiggy customer care or the Swiggy app.

## Coverage Check

| Endpoint | Tools in live catalog | Tools listed here | Missing |
|---|---:|---:|---:|
| `/food` | 14 | 14 | 0 |
