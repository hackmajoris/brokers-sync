# Widget: Payout Calendar

## What

A list of watchlist symbols whose next dividend payment falls inside the next 30 days, each showing the date and the yield.

## Why

Every widget on the watchlist today is static — a snapshot of ratios that do not move day to day. Nothing on the page is time-aware, so nothing ever tells the user "look at this *now*". A payout window is the one deadline the data already knows about.

## Data

`Position.payoutDate` (string) and `Position.payoutDateInterpretation`, already mapped in `web/src/services/portfolioService.ts` from `payout_date`. Sourced upstream from `go-finance`'s `GetPayoutDate`, which returns a zero time when Yahoo reports none — check for that rather than assuming a valid date.

## Implementation notes

- Parse `payoutDate`, drop entries in the past or more than 30 days out.
- Sort ascending; the nearest date is the point of the widget.
- Reuse `SignalList` in `WatchlistTab.tsx` with `note={i => shortDate(i.indicators?.payoutDate)}`.
- Zero-time and unparseable dates must be excluded, not rendered as "Invalid Date".

## Effort

Small. No backend change, no new request. One filter, one date formatter, one `SignalList`.
