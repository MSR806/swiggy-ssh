# Instamart TUI Psychology Audit

## Principles

- Make path obvious. User should know: search, cart, checkout, done.
- One main action per screen. One strongest visual target.
- Keys must stay same across screens. Same key, same meaning.
- Mode must be loud. Searching is not browsing. Cart is not checkout.
- Dangerous action needs friction. Safe action needs speed.
- Remove thinking tax. Do not make user remember counts, totals, or next step.

## Search Psychology

- Put cursor in search by default. Search screen means type now.
- Show selected result with strong contrast, not just tiny marker.
- Keep result movement muscle memory: `j/k`, arrows, enter.
- Add-to-cart should feel immediate and reversible.
- After add, keep user near same result. Do not jump unless action changes mode.
- Show cart count and total while searching. User should feel progress without opening cart.
- Empty state should suggest concrete searches, not explain system.

## Cart Psychology

- Cart is review and adjust mode. Make item list primary.
- Quantity controls must be visible near selected item.
- Increase, decrease, remove must be predictable and local to focused item.
- Total, fees, and final payable must stay visible.
- Checkout action should be visually separate from item edits.
- Empty cart should point back to search with one key.

## Checkout Psychology

- Checkout is commitment mode. Make that unmistakable.
- Show final payable, address, delivery estimate, and payment method before confirm.
- Confirm key should not be same as normal select unless screen clearly changed.
- Use a two-step danger pattern for placing order: review, then confirm.
- Keep cancel/back obvious until order is placed.
- After order placed, remove ambiguity: show success, order id, and tracking next step.

## What To Remove

- Remove repeated helper text that competes with primary action.
- Remove hidden shortcuts that only docs explain.
- Remove screen states where focus is visually unclear.
- Remove confirmations for harmless actions.
- Remove clever labels. Use plain verbs: search, add, remove, checkout, confirm.
- Remove any flow where user must remember prior screen details.

## Acceptance Checks

- A new user can search and add first item without reading docs.
- Focus is identifiable within one second on every screen.
- Same navigation keys work the same everywhere.
- Cart count and payable are visible before checkout.
- Checkout cannot happen by accidental rapid enter from search/cart.
- Removing items is easier than placing order.
- Back/cancel is always visible before final confirmation.
- No screen has two equally loud primary actions.
