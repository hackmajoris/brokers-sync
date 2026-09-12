# API Data: Short Interest

## What

Expose short interest as a percentage of float, and optionally days-to-cover.

## Why

A risk dimension nothing else on the page covers. The existing red flags are all balance-sheet based; short interest is a market-positioning signal, and a heavily shorted name behaves differently from its fundamentals in ways the current widgets cannot anticipate.

## Unlocks

- Squeeze-risk flag in the Red flags widget.
- Context for a symbol whose price action contradicts its fundamentals.

## Source

`defaultKeyStatistics.shortPercentOfFloat`, plus `sharesShortPriorMonth` for a trend.

## Implementation notes

- Yahoo's short data lags — it is exchange-reported on a delay of roughly two weeks. Label it with its as-of date or it will be read as live.
- Thin coverage outside US listings.
- Resist presenting it as a trade signal; it belongs as a risk flag alongside the existing ones.

## Effort

Low-medium. Single field, single module.
