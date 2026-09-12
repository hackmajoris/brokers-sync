# Backlog

Unbuilt ideas, one per file. Not a roadmap — nothing here is committed to.

Everything listed came out of the watchlist widget work; the three widgets that were built from that same set (Health × Valuation grid, Red flags, Income) are shipped and have no file here.

## Widgets — no new data needed

Every field is already on `Position` and already reaches the browser.

| Idea | Value |
|---|---|
| [Payout calendar](payout-calendar.md) | Only time-aware widget available from current data |
| [Growth leaders](growth-leaders.md) | Growth data is fetched but never surfaced on the watchlist |
| [Already held](already-held.md) | Distinguishes a new position from a top-up |
| [Momentum spread](momentum-spread.md) | Short vs long horizon, currently unreadable across a wide table |
| [Target progress](target-progress.md) | Makes the target column scannable as a whole |
| [Cap mix donut](cap-mix-donut.md) | Weakest of the set; listed for completeness |

## Widgets — need network requests

| Idea | Cost |
|---|---|
| [Sparklines](sparklines.md) | One history request per symbol; needs a caching or batching decision first |
| [Relative strength vs SPY](relative-strength-vs-spy.md) | One extra request per period, not per symbol |

## New API data

Fields the backend does not expose yet. Each unlocks widgets the current data cannot support.

| Field | Why it matters |
|---|---|
| [Next earnings date](api-earnings-date.md) | Nothing on the page is time-aware today |
| [Sector and industry](api-sector-industry.md) | Makes every cross-symbol comparison defensible instead of decorative |
| [Analyst target price](api-analyst-target-price.md) | Turns the user's solo target into a spread |
| [Recommendation trend](api-recommendation-trend.md) | The first real change signal, rather than another level |
| [50/200 day moving averages](api-moving-averages.md) | Trend without a price series; module already fetched |
| [Short interest](api-short-interest.md) | Risk dimension nothing else covers |
| [Beta](api-beta.md) | Cheap stand-in for the SPY comparison |

## If picking up just one

**Sector** and **earnings date**. Sector is the field that makes the comparisons already on screen legitimate — including the Health × Valuation grid, which currently crosses a utility and a software company as if their multiples meant the same thing. Earnings date is the only candidate that gives the page a reason to be opened on a particular day.
