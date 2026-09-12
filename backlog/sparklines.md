# Widget: Sparklines

## What

A 30-day price line on each mover card, and optionally in the symbol cell of the table.

## Why

The original mobile design called for these. The code comment in `WatchlistTab.tsx` records why they were dropped: a watchlist item carries indicators, not a price series, and drawing a trend from noise would have been a lie. That constraint is removable — `fetchHistory()` exists and returns real candles.

## Data

`fetchHistory(symbol, range, interval)` in `web/src/services/portfolioService.ts`, returning `HistoryData` / `Candle[]`.

## Cost

One request per symbol. A 30-symbol watchlist means 30 requests on load — this is the reason it has not been built, and the reason it needs a caching decision before any UI work:

- Cache in `localStorage` keyed by symbol + day, so a revisit within the same session costs nothing.
- Or fetch lazily — only the handful of symbols visible in the mover rails, not the whole table.
- Or add a batch history endpoint server-side, which turns N requests into one and is the only option that scales past a long list.

## Implementation notes

- `Candlestick` and `LineChart` already exist; a sparkline is a stripped `LineChart` with no axes.
- Decide the failure mode up front: a symbol whose history fails must render the card without a line, never a broken or empty chart frame.

## Effort

Medium, dominated by the caching/batching decision rather than the drawing.
