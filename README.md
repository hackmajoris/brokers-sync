# brokers-sync


[![Deploy](https://github.com/hackmajoris/brokers-sync/actions/workflows/deploy.yml/badge.svg)](https://github.com/hackmajoris/brokers-sync/actions/workflows/deploy.yml)

Parses and normalizes transaction exports from multiple brokers into a unified ledger, then computes portfolio statistics: open positions with unrealized P&L, realized P&L (FIFO), dividends, year-by-year breakdowns, and more.


## Features

- **No account required** — upload a ZIP of your broker exports and get results instantly. No login, no data stored server-side; the backend only parses the CSVs and calls Yahoo Finance for live prices, then discards everything.
- **ZIP saved in your browser** — the uploaded file is cached in IndexedDB so you can re-process without re-uploading. All data is transient and stays in your browser session.
- **Automatic broker detection** — drop any mix of exports into one ZIP; the format is inferred from the file headers automatically.
- **Per-broker statistics** — all-time return, all-time realized P&L, YTD realized P&L, total dividends received, annualized return, top realized gainers, top realized losers, and open positions summary — broken down per broker and aggregated across all of them.
- **Open positions** — quantity, average cost, cost basis, current market value, unrealized P&L (amount and %), portfolio allocation weight, and return since purchase.
- **Live market data per position** — 52-week high/low range, trailing and forward P/E, and YTD/3-year/5-year price performance, fetched from Yahoo Finance.
- **Fundamental indicators per position** — free cash flow, EV/EBITDA, debt-to-equity, and operating cash flow vs. net income (cash flow quality), each with a plain-language interpretation shown via an info popup on the column header.
- **Annual breakdown** — realized P&L, dividends, deposits, withdrawals, fees, buy volume, and sell volume — bucketed per calendar year.
- **P&L statistics** — all-time and YTD realized gain/loss, FIFO lot matching, largest single winners and losers.
- **Dividend statistics** — aggregated dividend income (all-time and YTD), annual dividend totals, year-over-year dividend progress, and per-symbol breakdown with gross amount, tax withheld, and net received.
- **Wishlist** — track companies you do not own yet, at `/wishlist`. Still no account: a generated portfolio code identifies the list, and it is the only credential. See [Wishlist](#wishlist) below.
- **Runs locally via Docker Compose** — single command (`docker compose up`) to spin up the full stack; no cloud account needed.

**Live site:** [brokersync.dot-core.com](http://brokersync.dot-core.com)

## Supported brokers

| Broker | Format |
|---|---|
| Revolut | CSV (`Date, Ticker, Type, Quantity, Price per share, Total Amount, Currency, FX Rate`) |
| Interactive Brokers (IBKR) | Multi-section CSV (Transaction History export) |
| Trading 212 | CSV (`Action, Time, ISIN, Ticker, Name, ...`) |
| Tradeville | Tab-separated CSV with leading `SEP=\t` hint line (`id, data, op, simbol, cant, pret, comis, suma, valuta`) |
| XTB | XLSX export with `Cash Operations` and `Closed Positions` sheets |

Broker format is detected automatically from the file header — no need to label files.

## Requirements

- Go 1.21+

```bash
go build ./...
```

## Running with Docker Compose

The easiest way to run the full stack (server + web UI) is with Docker Compose:

```bash
docker compose up
```

The web UI will be available at [http://localhost:8080](http://localhost:8080). Use the Settings page to upload your broker CSV files.

**Rebuild the image after code changes:**

```bash
docker compose up --build
```

**Run in the background:**

```bash
docker compose up -d
docker compose logs -f   # stream logs
docker compose down      # stop and remove the container
```

## Usage

### Drop files and run — zero configuration

Put any number of broker export CSV files into `data/` and run with no flags:

```bash
go run ./cmd/brokers-sync
```

The tool detects each file's broker automatically, merges all transactions, and removes any duplicates (useful when export date ranges overlap).

Example output when scanning `data/`:
```
Scanning data/
  ibrk-2024.csv          → ibkr         (215 transactions)
  revolut-all.csv        → revolut       (55 transactions)
  t212-2020.csv          → trading212   (141 transactions)
  dedup: removed 12 duplicate transaction(s)
Total: 399 transactions
```

### Skip live price fetch (faster, no unrealized P&L)

```bash
go run ./cmd/brokers-sync -no-prices
```

### Scan a different directory

```bash
go run ./cmd/brokers-sync -data ~/Downloads/statements/
```

### JSON output (for charting / external tools)

Writes a single JSON file with all positions, transactions, dividends, and period summaries.

```bash
go run ./cmd/brokers-sync -format json -out report.json
```

Omit `-out` to print JSON to stdout:

```bash
go run ./cmd/brokers-sync -format json
```

### CSV output (one file per section)

Writes five CSV files into the specified directory.

```bash
go run ./cmd/brokers-sync -format csv -out ./out/
```

Files produced:

| File | Contents |
|---|---|
| `positions.csv` | Open positions with cost basis and live unrealized P&L |
| `realized_by_symbol.csv` | Realized P&L per ticker (all time) |
| `dividends_by_symbol.csv` | Gross dividends, tax withheld, net per ticker |
| `summary_by_year.csv` | Realized, dividends, fees, deposits, withdrawals per calendar year |
| `transactions.csv` | All normalized transactions from all brokers |

### Explicit file flags (optional override)

If you need to point at files outside `data/`, use the explicit flags.
These are merged with (and deduplicated against) whatever the directory scan finds.

```bash
go run ./cmd/brokers-sync \
  -revolut ~/Downloads/revolut-new.csv \
  -ibkr    ~/Downloads/ibkr-new.csv \
  -t212    ~/Downloads/t212-new.csv
```

Mix explicit files with a directory scan:

```bash
go run ./cmd/brokers-sync -data ./data -revolut ~/Downloads/revolut-latest.csv
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-data <dir>` | `data` | Directory to scan for broker CSV files (auto-detected) |
| `-revolut <file>` | — | Explicit Revolut CSV (merged with `-data` scan) |
| `-ibkr <file>` | — | Explicit IBKR CSV (merged with `-data` scan) |
| `-t212 <file>` | — | Explicit Trading 212 CSV (merged with `-data` scan) |
| `-format <text\|json\|csv>` | `text` | Output format |
| `-out <path>` | stdout / `.` | Output file (json) or directory (csv) |
| `-no-prices` | false | Skip Yahoo Finance price fetch |

## Deduplication

Transactions are keyed by a hash of `broker + date + symbol + type + quantity + net amount`.
Any transaction appearing more than once — across files or from overlapping export ranges — is dropped silently after the first occurrence. The count of removed duplicates is printed to stderr.

## Output sections (text mode)

- **Open Positions** — quantity, average cost, cost basis, market value, unrealized P&L and % (when prices available)
- **Realized P&L by Symbol** — FIFO-based realized gain/loss per ticker, all time
- **All Time / YTD / MTD** — realized P&L, net dividends, tax withheld, fees, deposits, withdrawals, buy/sell volume
- **Year-by-Year Breakdown** — same metrics bucketed per calendar year
- **Dividends by Symbol** — gross, tax withheld, net per ticker, all time

## Project structure

```
cmd/
  brokers-sync/main.go    CLI entry point and text output
  server/main.go          HTTP server serving the web UI and JSON API
internal/
  model/transaction.go    Normalized Transaction type and TxType enum
  parser/
    detect.go             Auto-detection of broker format + directory loader + dedup
    revolut.go            Revolut CSV parser
    ibkr.go               IBKR multi-section CSV parser
    trading212.go         Trading 212 CSV parser
    tradeville.go         Tradeville tab-separated CSV parser
    xtb.go                XTB XLSX parser
    util.go               Shared helpers (column mapping, date parsing, ID hashing)
  ledger/ledger.go        FIFO lot tracking, realized P&L, stock split adjustment
  stats/stats.go          Period aggregations, unrealized P&L enrichment
  prices/yahoo.go         Yahoo Finance chart API, parallel price fetch
  output/report.go        JSON and CSV report writers
  wishlist/               Portfolio code generation and DynamoDB wishlist store
web/                      React + Vite frontend (served by cmd/server)
infra/                    AWS CDK stack (Lambda + API Gateway deployment)
```

## Wishlist

Tracks companies you do not own yet. Reached at `/wishlist`.

There are still no accounts. The app generates a **portfolio code** such as
`K7M2-9QRF-3XVB-8TDW`, stores it in your browser's `localStorage`, and sends it
with each request. Possession of the code *is* access — anyone holding it can
read and edit that wishlist.

**The code cannot be recovered or reset. Lose it and the list is gone.** It is
shown once, at creation.

Design notes:

- The server stores only `sha256(code)`, so a dump of the table yields no usable
  codes.
- The code travels in an `X-Portfolio-Code` header, never in a URL. URLs end up
  in CloudFront and API Gateway access logs, browser history, and `Referer`
  headers on outbound links.
- Missing, malformed and unknown codes all return an identical bare `404`, so
  responses cannot be used to discover which codes exist.
- Server-side caps: 50 symbols per wishlist, 500-character notes, 12-character
  symbols.
- Wishlists with no activity for 12 months are removed automatically by a
  DynamoDB TTL.

### Endpoints

All require the `X-Portfolio-Code` header except `POST /api/wishlist/new`.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/wishlist/new` | Generate a code, returns `{"code": "..."}` |
| `GET` | `/api/wishlist` | List entries |
| `PUT` | `/api/wishlist` | Add or update `{"symbol","note","targetPrice"}` |
| `DELETE` | `/api/wishlist?symbol=X` | Remove an entry |

The routes are registered only when `WISHLIST_TABLE` is set, so local runs
without AWS credentials work unchanged — the tab simply reports the wishlist as
unavailable.

## Cost guardrails

CloudFront has no spend cap, and cache hits never reach API Gateway, so the
10 rps API throttle does nothing to bound egress. Three ceilings are deployed:

| Guardrail | Effect |
| --- | --- |
| Lambda reserved concurrency (5) | Caps worst-case compute |
| DynamoDB provisioned 1 RCU / 1 WCU, no autoscaling | Excess traffic is throttled, not billed (~$0.47/month) |
| CloudWatch alarm on CloudFront `BytesDownloaded` | **Disables the distribution** and emails on an egress spike |
| Monthly AWS Budget | Emails at 80% actual and 100% forecast — slow backup only, billing data lags 8–24h |

Deploy-time context keys (all optional, defaults shown):

```
--context alertEmail=brokersync@dot-core.com
--context budgetLimitUsd=5
--context bytesAlarmGb=5
```

The alarm stack (`BrokersSyncCostGuardStack`) deploys to **us-east-1** because
CloudFront publishes its CloudWatch metrics nowhere else. The main stack stays
in eu-central-1.

**After the first deploy, confirm the SNS subscription email.** Until that link
is clicked, the alarm still fires but no mail is delivered.

### When the kill switch fires

The site goes down — that is the intended behaviour, not a bug. The distribution
is left disabled until you re-enable it deliberately:

```sh
aws cloudfront get-distribution-config --id <DIST_ID> > cfg.json
# set .DistributionConfig.Enabled = true, keep the ETag
aws cloudfront update-distribution --id <DIST_ID> \
  --distribution-config file://config-only.json --if-match <ETag>
```

Re-enabling takes roughly 10 minutes to propagate. Check CloudFront logs for the
source of the traffic before turning it back on.
