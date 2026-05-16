# Instamart TUI Payment And UX Fix

## Goals

- Fix payment method state after adding an item from search so checkout can use the available `Cash` method without requiring a manual back-and-forth refresh.
- Make the search flow self-explanatory: typing filters, `Enter` moves to results, arrow keys pick rows only after results are active.
- Simplify the staged cart and final bill so the user's focus goes to address, items, bill, payment, and the next action.
- Make the checkout confirmation feel like a deliberate deploy gate with a visible `git push --force groceries` command.

## Known Bug

- After searching and adding an item, cart data can include items/bill but payment method may not be selected yet.
- Going back and returning to cart refreshes state and sets payment to `Cash`.
- Expected behavior: after cart update and cart load, the model should derive payment from `currentCart.AvailablePaymentMethods` immediately.

## UX Problems To Fix

- Search preview displays selectable-looking rows while the input is still focused, but down arrow does nothing until `Enter` commits the query.
- The cart bill is visually dense and mixes diff markers, totals, warnings, payment, and next-step hints without a clear hierarchy.
- Checkout confirmation hides the dangerous deploy metaphor in small text instead of making the action obvious.
- Copy is developer-themed, but not consistently instructional.

## Implementation Plan

- Audit state transitions for `paymentMethod`, `currentCart`, `reviewedCart`, add-to-cart, and cart load.
- Add a single payment derivation helper and call it whenever a fresh cart is accepted.
- Change search input rendering to show preview as a read-only grep preview with a strong `press Enter to inspect results` hint.
- Change product results rendering to make arrow/enter controls obvious once results are active.
- Rework cart review into four clear sections: deploying to, staged items, bill diff, payment gate.
- Rework checkout confirmation into a focused command screen centered around `git push --force groceries` and `press y to deploy`.
- Add or update tests for immediate payment selection after cart load/update and key UX copy.

## Verification

- Run targeted Instamart TUI tests.
- Run `go test ./...`.
- Use subagent review for root cause and UX readability after implementation.
