# Widget: Relative Strength vs SPY

## What

Each symbol's return over the selected period, minus SPY's return over the same period. Answers whether a "gainer" is actually outperforming or just riding the market.

## Why

The Gainers rail currently rewards beta. On a day the whole market is up 2%, a symbol up 2.1% appears in the rail as a winner; it is noise. Subtracting the index turns the rail from "what moved" into "what moved *for its own reasons*".

## Data

One extra `fetchHistory('SPY', ...)` call per period — not per symbol. Cheap relative to the sparkline idea.

Alternative: `beta` from `defaultKeyStatistics` would approximate this with no history fetch at all, but is a static coefficient rather than a live comparison. See `api-beta.md`.

## Implementation notes

- The period must match the active timeframe pill exactly, refetching when it changes.
- Cache the SPY series per period for the session.
- Consider whether the index should be configurable (SPY vs a European index) given the portfolio may not be US-centric.

## Effort

Small-to-medium. One request, one subtraction, but touches the timeframe plumbing.
