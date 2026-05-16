# Instamart TUI Readability Audit

Goal: less clever fog. More obvious terminal grocery tool.

## Search Input / Preview

- Pain: preview rows look selectable, but focus still in query box.
- Fix: label preview as read-only: `preview only`. Say `Enter opens results` near table.
- Fix: hide cursor arrow in preview table. Cursor means movable thing.
- Fix: show query state: `typing: amul milk` and `matched: 5`.

## Selectable Results

- Pain: product table and preview table look same.
- Fix: active results title: `results: choose exact pack`.
- Fix: keep `>` only here, never in preview.
- Fix: add one top hint: `j/k move, enter stage, 1-9 quick pick`.
- Fix: unavailable rows need dull style, not just `409` text.

## Cart Bill Readability

- Pain: cart review is diff-ish but still one dense block.
- Fix: split into chunks: `target`, `items`, `bill`, `payment`, `next`.
- Fix: make `To Pay` last, bold, with blank line before it.
- Fix: discount rows use `-`; fees/taxes use `+`; subtotal no scary green plus.
- Fix: item rows should align qty, name, price. Long names must not break bill column.

## Checkout Deploy Gate

- Pain: real order warning competes with joke command.
- Fix: make gate loud and plain: `REAL ORDER. Press y only if cart is right.`
- Fix: show command as main object: `git push --force groceries`.
- Fix: keep checklist before command, final action after command.
- Fix: `Enter` must never imply confirm. Footer should not mention enter here.

## Footer Consistency

- Pain: footer verbs drift: `ship`, `confirm order`, `home`, `cancel`.
- Fix: same verbs everywhere: `grep`, `stage`, `cart`, `ship`, `tail`, `home`, `quit`.
- Fix: always include back key when not home: prefer `b home` or `esc back`, not mixed.
- Fix: footer must describe active screen only. No hidden actions.
- Fix: checkout footer: `y deploy`, `n cancel`, `? help`. Nothing else.

## Focus Hierarchy

- Pain: header, status, title, warnings, body, footer all compete.
- Fix: one bright thing per screen: current action.
- Fix: warnings only amber/red. Status success line should not steal focus.
- Fix: keep session status small and stable: `env auth addr cart mode`.
- Fix: put next action above footer when dangerous or important.

## Extra Cuts

- Pain: `deploying to` appears many places.
- Fix: header owns address. Body repeats only on cart and checkout.
- Pain: home cart count uses item kinds in one place, quantities in status.
- Fix: use quantity count everywhere or label `lines` versus `items`.
- Pain: loading copy changes by mood.
- Fix: standard shape: `<verb>...` then status says `<done> in <time>`.
- Pain: help screen can become stale.
- Fix: generate help from same key map as footer, or audit every footer change.

## Do Next

- First: separate preview table from result table visually.
- Second: chunk cart review.
- Third: tighten checkout gate copy.
- Fourth: normalize footer verbs.
- Fifth: make focus rule testable with copy tests.
