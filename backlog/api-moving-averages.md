# API Data: 50 and 200 Day Moving Averages

## What

Expose the 50-day and 200-day moving averages per symbol.

## Why

The page has a 52-week range gauge, which is a snapshot of position within a band, but nothing about direction. Moving averages give trend without needing a full price series per symbol — two numbers instead of 250 candles.

## Unlocks

- Golden cross / death cross flag (50 crossing 200).
- "Above/below 200d" as a trend filter chip.
- Trend context next to the existing range gauge.
- A much cheaper partial substitute for per-symbol sparklines.

## Source

`summaryDetail.fiftyDayAverage` and `summaryDetail.twoHundredDayAverage`. The `summaryDetail` module is already fetched for dividend yield and payout ratio, so these are free on an existing call.

## Implementation notes

- A cross is an event between two snapshots; detecting it needs the previous values stored, same constraint as `api-recommendation-trend.md`. Without history, only the current above/below state is available — still useful, but not a cross signal.
- Cheapest entry on this list: the module is already being fetched.

## Effort

Low for the above/below state. Medium if the cross event is wanted, because that needs stored history.
