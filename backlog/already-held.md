# Widget: Already Held

## What

Marks watchlist symbols the user already owns, per the imported broker portfolio, showing the current position size.

## Why

The watchlist is framed as "companies you do not own yet", but nothing enforces or reflects that. A symbol on both lists is a fundamentally different decision — a top-up, sized against an existing position — and today the user has to remember which is which by switching tabs.

## Data

No new API. `loadCachedPortfolio()` in `web/src/services/portfolioService.ts` already returns the mapped portfolio from local cache, including `open_positions` with quantity and market value.

## Implementation notes

- Cache may be empty (a user who never imported a portfolio). The widget must render nothing at all in that case, not an empty shell.
- Cross-reference on `symbol`; broker symbol formats may differ from the watchlist's search-derived symbols — verify before assuming an exact match.
- Beyond the widget, consider a small badge on the symbol cell in the table. That is the more useful half of the idea and should probably ship with it.

## Effort

Small-to-medium. The risk is symbol normalisation, not the UI.
