# API Data: Analyst Target Price

## What

Expose the mean analyst price target and the number of analysts behind it.

## Why

The watchlist's target price is the user's own number, set by hand and seeded at a naive 20% below the current price. It has nothing to compare against. An analyst mean turns a solo guess into a spread — "my target is 15% below the street's" is a far more useful sentence than either number alone.

## Unlocks

- Spread column or widget: user target vs street target.
- Sanity check on the seeded default target.
- A "street sees upside" signal list.

## Source

`financialData.targetMeanPrice`, with `numberOfAnalystOpinions` from the same module. The backend already fetches `financialData` for debt-to-equity (`fetchDebtToEquity` in `go-finance`), so the module is already on the call path.

## Implementation notes

- Ship the analyst count alongside the target. A mean over two analysts and a mean over thirty are not the same number, and displaying the first without its count is misleading.
- Wide-coverage US large caps will have this; smaller and non-US symbols often will not. The null case is common, not exceptional.

## Effort

Medium-low. Same module as an existing fetch.
