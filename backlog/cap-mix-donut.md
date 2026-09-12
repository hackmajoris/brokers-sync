# Widget: Market Cap Mix

## What

A donut splitting the watchlist into mega / large / mid / small cap buckets.

## Why

Weakest of the free widgets, listed for completeness. A watchlist is a shopping list, not a portfolio — its cap distribution carries much less meaning than a portfolio's would. Worth building only if the user finds the list drifting toward one size without noticing.

## Data

`Position.marketCap` and `marketCapInterpretation`. `DonutChart` exists in `web/src/components/charts/`.

## Implementation notes

- Bucket thresholds must be stated somewhere visible; there is no universal definition of "mid cap".
- `marketCapInterpretation` may already carry a bucket label from upstream — prefer it over inventing thresholds in the frontend.

## Effort

Small. Low value; consider dropping rather than building.
