# Widget: Target Progress

## What

A row of horizontal bars, one per symbol, showing how far the current price sits from the user's buy target.

## Why

The Target Diff column already carries this number, but as text, in one cell of a wide scrolling table. Turning it into a length makes the whole list scannable at once — which is the actual question the user has ("what is closest to my target?"), asked of the whole list rather than one row at a time.

## Data

`targetGap()` already exists in `WatchlistTab.tsx`. `HorizBar` already exists in `web/src/components/charts/`.

## Implementation notes

- Read `HorizBar`'s props before assuming it fits; it may be built for a fixed domain.
- Sort by proximity to target, closest first. Symbols with no target set are excluded, not shown at zero.
- Overlaps with the existing "Target hit" signal list — that one shows symbols already past the target, this shows the approach. Decide whether both earn their space or one replaces the other.

## Effort

Small, assuming `HorizBar` is reusable as-is.
