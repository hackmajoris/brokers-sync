# API Data: Beta

## What

Expose each symbol's beta coefficient.

## Why

Cheap answer to "is this gainer just tracking the market?". The Gainers rail currently rewards high-beta names on any up day. Beta is a static approximation of what the relative-strength-vs-SPY idea measures live, at zero request cost.

## Unlocks

- Context on the mover rails.
- A rough volatility read for a list that otherwise has none.
- A low-cost stand-in for `relative-strength-vs-spy.md` — worth comparing the two before building both.

## Source

`defaultKeyStatistics.beta` in quoteSummary.

## Implementation notes

- Beta is backward-looking and period-dependent; Yahoo's is typically five-year monthly. State the basis or it invites over-reading.
- Weaker than an actual index comparison, but free. If the SPY-history widget gets built, beta becomes redundant.

## Effort

Low. Single field.
