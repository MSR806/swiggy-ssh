# Instamart Payment Bug Audit

## Bug

`update_cart` can return cart data with no `availablePaymentMethods`. TUI sees empty payment list and blocks checkout. But `get_cart` returns `Cash`, so user can pay.

## Likely Root Cause

Cart update response is not full checkout-ready cart. Payment methods are populated by fresh cart read, not by update call.

## Smallest Fix

After any Instamart `update_cart`, call `get_cart` before showing payment options or enabling checkout. Treat `get_cart.availablePaymentMethods` as source of truth. Do not trust payment methods from `update_cart`.

## Acceptance Checks

- Add item to Instamart cart.
- `update_cart` response may have no payment methods.
- App immediately refreshes with `get_cart`.
- Payment section shows `Cash` when `get_cart` returns it.
- Checkout button/flow is enabled only from refreshed `get_cart` data.
- No code path blocks checkout only because `update_cart.availablePaymentMethods` is empty.
