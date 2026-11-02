# Amazon product research

* Normalize user product links to `https://www.amazon.com/dp/<ASIN>` to remove tracking parameters and preserve the exact product identifier.
* The web tool may return Amazon as thousands of lines of raw HTML. Use targeted web `find` calls for `productTitle`, the requested feature, `priceToPay`, and `One-Time Price` instead of opening the page from line zero repeatedly.
* Verify pack count, size, and variant. Packaging features such as pull tabs can differ across sizes within one brand; generic “easy” marketing does not prove a can is opener-free.
* Separate one-time prices from Subscribe & Save tiers, coupons, and unit prices. Cached HTML can contain hidden or conflicting availability fields, so do not promise checkout price or stock.
* Grocery fulfillment has multiple paths. Since the 2025 expansion, some perishable/frozen groceries can share Amazon's Same-Day cart with other eligible goods; they do not invariably require a separate Fresh/Whole Foods order. Distinguish regular non-Prime free-shipping offers, Prime Same-Day thresholds, and separate grocery-service carts. Ask which offer/account status only if needed; never claim an item counts toward a threshold without its applicable eligibility message or checkout evidence. Thresholds depend on location; consult current official guidance such as https://www.aboutamazon.com/news/retail/how-to-order-groceries-amazon .
* Manufacturer pages and retailer descriptions can corroborate packaging. Deal sites can help discover an ASIN through read-only product links, but old advertised prices should not be treated as current. Strip affiliate parameters from the final user-facing Amazon link.
* Save verified ASINs, sources, variant details, observed prices and their limitations in the relevant task note in `context/`. Shopping research does not authorize purchases or cart changes.

## Authorized Subscribe & Save cancellations

* Use the connected browser and follow `workflow/playwright-workflow.md`. Check existing tabs for the signed-in management page before navigating elsewhere. Subscription management lives at `https://www.amazon.com/auto-deliveries`.
* Inspect **Your Subscriptions**, not just the upcoming-deliveries carousel. Clicking Your Subscriptions may only scroll to the list. Global **Edit subscriptions** opens a schedule editor; cancellation is under each item's **Edit** → **Cancel subscription** → **Cancel my subscription**.
* The cancellation reason can be optional; leave it unset rather than inventing a reason. Review consequences: Amazon may warn it also cancels orders not yet in the delivery process.
* Wait for **Cancellation Confirmed** for each item. UI refs may change when dialogs close or a card is removed, so refresh the relevant snapshot/find before using the next item's controls.
* Reload the management page to verify the remaining active list. Empty-state text may be scoped to an address; inspect available address controls if present, and do not claim an all-marketplace audit.
* If initial orders may be affected, inspect the relevant recent order status read-only and distinguish it from cancellation of future recurring deliveries. Do not cancel a separate order unless authorized.
* Save names, outcome, timestamp (UTC), and useful source identifiers in a task note, excluding payment and address details. Remove tool-generated snapshots and automatic console files after preserving the necessary evidence; do not enable extra logging.
