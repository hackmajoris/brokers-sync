# API Data: Next Earnings Date

## What

Expose the next scheduled earnings report date per symbol.

## Why

The highest-value missing field. Nothing currently on the watchlist page is time-aware — it is entirely a snapshot of static ratios. An earnings date is the one piece of information that makes the page worth opening on a particular morning rather than any morning. It also gates every other reading on the page: a valuation judgement made the day before a report has a short shelf life.

## Unlocks

- "Reporting this week" list.
- A marker on the symbol cell for imminent reports.
- Suppressing or caveating the valuation widgets for symbols about to report.

## Source

`calendarEvents.earnings` in the Yahoo quoteSummary response. The backend already calls quoteSummary for other modules in `internal/prices/ticker_indicators.go`, so this is an additional module on an existing call path rather than a new integration.

## Implementation notes

- Yahoo returns an estimated date range for some symbols, not a single confirmed date. The type must carry that ambiguity rather than flattening it to one date and implying false precision.
- Needs plumbing through the same chain as every other indicator: `go-finance` client → `ticker_indicators.go` → `stats.go` → JSON field → `RawPosition` → `mapPosition` → `Position`.
- May require a `go-finance` release if the client does not expose this module yet.

## Effort

Medium. The plumbing is well-trodden; the upstream client may need the accessor first.
