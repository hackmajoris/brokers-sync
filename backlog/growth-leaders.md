# Widget: Growth Leaders

## What

Top three watchlist symbols by quarterly revenue growth, and the top three by quarterly earnings growth.

## Why

Both figures are fetched, mapped, and shown in the stock lookup modal — but no column and no widget surfaces them on the watchlist itself. Every cross-symbol comparison currently available is valuation or performance; growth is the missing third axis, and it is the one that explains why an expensive symbol is expensive.

## Data

`Position.quarterlyRevenueGrowth` and `Position.quarterlyEarningsGrowth`, plus their `*Interpretation` strings. Already on the presentation model.

## Implementation notes

- Two `SignalList` instances side by side, or one with a small toggle — two lists is simpler and matches the existing Gainers/Losers pairing.
- `note={i => fmtPct(...)}`.
- Earnings growth is far noisier than revenue growth (a small base swings it wildly); consider excluding symbols whose absolute value exceeds some sanity bound, or accept the noise and let the tooltip's interpretation explain it.

## Effort

Small. No backend change.
