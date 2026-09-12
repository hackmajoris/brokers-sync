# API Data: Recommendation Trend

## What

Expose the distribution of analyst recommendations (strong buy / buy / hold / sell) over the last few months, or at minimum the month-over-month change.

## Why

Almost everything on the watchlist is a level — a ratio, a price, a rating. Nothing is a *change*. Upgrades and downgrades are a genuine "something happened here since you last looked" signal, which is exactly what a watchlist is for and exactly what it currently cannot tell you.

## Unlocks

- "Changed this month" list — the first real event feed on the page.
- Pairs with the existing Health × Valuation grid: a cheap-and-healthy symbol being downgraded is a different story from one being upgraded.

## Source

`recommendationTrend` module in quoteSummary.

## Implementation notes

- Store at least two periods so a delta can be computed; a single snapshot of current ratings is a level again, not a change, and would miss the point of the widget.
- Coverage is thin outside large caps.
- Consider how stale the cached indicator data is — a change signal is only meaningful if the refresh cadence is shorter than the thing it is detecting.

## Effort

Medium. Needs a decision on storing history rather than just the latest value.
