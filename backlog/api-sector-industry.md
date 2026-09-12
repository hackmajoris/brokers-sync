# API Data: Sector and Industry

## What

Expose each symbol's sector and industry classification.

## Why

The second-highest-value missing field. Every cross-symbol comparison on the page today is apples-to-oranges: a P/E of 40 means something different for software than for a utility, and the Health × Valuation grid silently crosses them as if they were comparable. Sector is the field that makes peer comparison legitimate rather than decorative.

## Unlocks

- Sector mix donut (more meaningful than the cap-mix idea).
- "Cheapest in sector" rather than "cheapest on the list".
- Sector-relative colouring on the P/E and EV/EBITDA columns.
- A sector filter chip.
- Legitimises the debt-to-equity red flag, which currently uses a blunt 200% threshold precisely because sector is unknown.

## Source

`assetProfile.sector` and `assetProfile.industry` in quoteSummary.

## Implementation notes

- Static per symbol — a strong candidate for long-lived caching, unlike the price-derived indicators.
- Yahoo's sector taxonomy is its own; do not assume GICS names.
- The payoff is not the donut, it is every existing comparison becoming defensible. Worth prioritising above most of the widget backlog for that reason.

## Effort

Medium-low. Static data, simple field, large downstream benefit.
