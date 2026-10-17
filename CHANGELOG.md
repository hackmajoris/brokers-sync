# Changelog

Release notes for each version, newest first. Generated from GitHub releases.

## [v0.22.1] - 2026-10-17

**Improvements**
- fix ci f673994
- ci fix 317fa57
- added changelog.md d22ca82

## [v0.22.0] - 2026-10-17

**New Features**
- spread indicator df83ec2
- etf holdings | etf TER | ticker currency ef62d90
- light theme 986a0ca
- turnstile integration 1d746c0
- split watchlist feature in a new standalone app ab0feae

**Improvements**
- tsgo | vp+ | tooling upgrade 8d1f209
- footer details 86f3932
- updates | internal scripts stats bbc2380
- forbid direct API Gateway access 4330874

**Other**
- ui: fix mobile layout 19e15d7
- ui: replace logo bf752c4

## [v0.21.2] - 2026-10-04

**Bug Fixes**
- tax calculation 9db0dd7

## [v0.21.1] - 2026-10-04

**Improvements**
- xtb parser fix 5f5db1e

## [v0.21.0] - 2026-09-26

**New Features**
- monthly dividends breakdown 17cf81a

**Improvements**
- fix merged .gitignore line for bin/ d009cf6
- untrack bin/server build output 4da170d

## [v0.20.1] - 2026-09-22

**Improvements**
- upgrade to go-finance v0.12.0 bd2d71a

## [v0.20.0] - 2026-09-16

**New Features**
- news section 49bfc5b

## [v0.19.1] - 2026-09-13

**Other**
- ui: enhance mobile ticker overview 9656d67

## [v0.19.0] - 2026-09-12

**New Features**
- Sector-relative valuation on the positions and watchlist tables: each symbol's P/E and EV/EBITDA are now compared against a market-cap-weighted aggregate for its sector, and coloured accordingly
- Analyst ratings column showing Yahoo's average recommendation
- Summary widgets above the positions table: period selector, breadth, top movers, and signal lists for 52-week extremes, financial red flags, dividend income, and a health-versus-valuation grid
- Collapsible signal cards on the watchlist, so the summary no longer crowds out the table

**Improvements**
- Searching the watchlist now opens a symbol's details instead of adding it to the list; adding is an explicit action
- The watchlist table scrolls with the page rather than inside its own frame
- The column-mode switch now sits beside the table it affects

**Bug Fixes**
- Adding a symbol already on the watchlist no longer erases its note, target price, and pin

## [v0.18.0] - 2026-09-02

**New Features**
- mobile view enhancement 2962525

## [v0.17.1] - 2026-08-27

**New Features**
- enhance mobile view layout 164b7d3

## [v0.17.0] - 2026-08-27

**New Features**
- mobile list layout 34bd1ed

## [v0.16.0] - 2026-08-27

**New Features**
- pinned tickers | compare mode b3c3029

## [v0.15.0] - 2026-08-23

**New Features**
- add day/week/month/10yr performance columns to positions and watchlist 8b6192e

**Other**
- bugfix: fix zip stale data d20179d

## [v0.14.0] - 2026-08-18

**New Features**
- watchlist: new tickers default to a target 20% below the current price, plus a per-row reset button 2a6cddb
- watchlist: Target Diff column showing percent to target, coloured red/green 2a6cddb

**Improvements**
- watchlist: note moved before the remove column, remove is now an icon 2a6cddb
- indicator columns reordered: YTD/3Y/5Y and 52 week ahead of the target columns 2a6cddb

## [v0.13.0] - 2026-08-18

**Improvements**
- watchlist indicators: list-scope fetch, shared Yahoo client, 5-minute in-memory cache (8s -> 3.9s cold, instant warm) e91e631
- CloudFront SPA configs a8606ec

**Other**
- watchlist table: sortable columns, pinned symbol column, target-price highlight 2f5798b

## [v0.12.1] - 2026-08-17

**Bug Fixes**
- treat HTML watchlist responses as an invalid code cbfefb5
- declare JSON content type on watchlist code creation d0b3fd9

## [v0.12.0] - 2026-08-17

**New Features**
- show position indicators on the watchlist 79355c2
- add watchlist tab at /watchlist 1396159
- add watchlist table and AWS cost guardrails e983ce5
- add watchlist backend keyed by portfolio code bbd818d

**Improvements**
- add watchlist and cost guardrails implementation plan 80ac3dd

**Bug Fixes**
- 404 unregistered API routes instead of serving the SPA 14107eb
- drop Lambda reserved concurrency 259bb3b

PR #1 @hackmajoris

## [v0.11.0] - 2026-08-14

**New Features**
- show all trades in the Trades tab 00aa560

## [v0.10.1] - 2026-08-14

**Bug Fixes**
- tradeville: accept dot-separated dates (DD.MM.YYYY) so second statement file parses; fixes lost transactions / wrong holdings 68657d0

## [v0.10.0] - 2026-08-12

**New Features**
- bind more indicators (16 fundamentals in ticker modal: market cap, price/sales, price/book, FCF yield, margins, YoY growth, cash/debt/net, dividend yield, payout ratio & date) fc37fa2
- add 10Y and Max chart ranges to the lookup modal fc37fa2

**Bug Fixes**
- bump go-finance to v0.7.1 to escape poisoned v0.7.0 proxy cache e9e5eb0

## [v0.9.0] - 2026-08-12

**New Features**
- show all kind of indicators in the modal on ticker click 6940696

## [v0.8.0] - 2026-08-05

**New Features**
- health score | valuation score ed8d40d

## [v0.7.0] - 2026-08-05

**New Features**
- free cash flow | ebitda | detb-to-equity | cash flow - net income 3ad3236

## [v0.6.0] - 2026-08-05

**New Features**
- YTD/3/5 price return performance 3ebd355

## [v0.5.0] - 2026-08-05

**New Features**
- PE/Forward PE ratio f791df1

## [v0.4.0] - 2026-08-04

**Other**
- ux: positions columns sorting de60ed3

## [v0.3.0] - 2026-08-04

**New Features**
- 52 weeks low/high 5c458da

**Improvements**
- fix 0e034aa

## [v0.2.0] - 2026-07-26

**Improvements**
- add brokers withdrawal amount column

## [v0.1.0] - 2026-07-26

**New Features**
- enhance json format 915d349
- web app 9f13884
- zip persist and refresh f9007e6
- docker compose 697a5fd
- show transfer in / out amounts ef3e3cc
- show total broker amounts at the broker list d9bad8b
- add withdrowal column at the brokers table b93d105

**Improvements**
- initial configs 501e07f
- more brokers d22931e
- fixes adbbf27
- use go-finance package 27b58b5
- cdk stack and docs update 87179fd
- cdk deploy configs 05794c0
- update README.md 9f8f2b2
- fix total amount calculation 3a648be
- fix lint aff42b0
- sidenav on mobile 8ac647b
- update docker-compose and Makefile for improved configuration 45d8531
- add coding guidelines to CLAUDE.md and remove from README.md ec5e9e7
- add deployment badge to README.md 9131328
- adjust parsing for in-out transfers and more fixes 6d09537
- fix in/out calculation 4376764
- minor formatting effcf53
