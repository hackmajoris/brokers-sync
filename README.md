# brokers-sync

Parses and normalizes transaction exports from multiple brokers into a unified ledger, then computes portfolio statistics: open positions with unrealized P&L, realized P&L (FIFO), dividends, year-by-year breakdowns, and more.

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
web/                      React + Vite frontend (served by cmd/server)
infra/                    AWS CDK stack (Lambda + API Gateway deployment)
```
