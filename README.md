# brokers-sync

Parses and normalizes transaction exports from multiple brokers into a unified ledger, then computes portfolio statistics: open positions with unrealized P&L, realized P&L (FIFO), dividends, year-by-year breakdowns, and more.

## Supported brokers

| Broker                     | Format                                                                                 |
|----------------------------|----------------------------------------------------------------------------------------|
| Revolut                    | CSV (`Date, Ticker, Type, Quantity, Price per share, Total Amount, Currency, FX Rate`) |
| Interactive Brokers (IBKR) | Multi-section CSV (Transaction History export)                                         |
| Trading 212                | CSV (`Action, Time, ISIN, Ticker, Name, ...`)                                          |

## Requirements

- Go 1.21+

```bash
go build ./...
```

## Usage

### Basic — text output with live prices

```bash
go run ./cmd/brokers-sync \
  -revolut  data/revolut-all-tmp.csv \
  -ibkr     data/ibrk-all-tmp.csv \
  -t212     data/from_2020-06-15_to_2020-12-31-tmp.csv
```

### Skip live price fetch (no unrealized P&L)

```bash
go run ./cmd/brokers-sync \
  -revolut  data/revolut-all-tmp.csv \
  -ibkr     data/ibrk-all-tmp.csv \
  -t212     data/from_2020-06-15_to_2020-12-31-tmp.csv \
  -no-prices
```

### JSON output (for charting / external tools)

Writes a single JSON file with all positions, transactions, dividends, and period summaries.

```bash
go run ./cmd/brokers-sync \
  -revolut  data/revolut-all-tmp.csv \
  -ibkr     data/ibrk-all-tmp.csv \
  -t212     data/from_2020-06-15_to_2020-12-31-tmp.csv \
  -format json \
  -out report.json
```

Omit `-out` to print JSON to stdout:

```bash
go run ./cmd/brokers-sync \
  -revolut  data/revolut-all-tmp.csv \
  -ibkr     data/ibrk-all-tmp.csv \
  -t212     data/from_2020-06-15_to_2020-12-31-tmp.csv \
  -format json
```

### CSV output (one file per section)

Writes five CSV files into the specified directory.

```bash
go run ./cmd/brokers-sync \
  -revolut  data/revolut-all-tmp.csv \
  -ibkr     data/ibrk-all-tmp.csv \
  -t212     data/from_2020-06-15_to_2020-12-31-tmp.csv \
  -format csv \
  -out ./out/
```

Files produced:

| File                      | Contents                                                           |
|---------------------------|--------------------------------------------------------------------|
| `positions.csv`           | Open positions with cost basis and live unrealized P&L             |
| `realized_by_symbol.csv`  | Realized P&L per ticker (all time)                                 |
| `dividends_by_symbol.csv` | Gross dividends, tax withheld, net per ticker                      |
| `summary_by_year.csv`     | Realized, dividends, fees, deposits, withdrawals per calendar year |
| `transactions.csv`        | All normalized transactions from all brokers                       |

### Use a single broker file

All broker flags are optional — pass only the ones you have:

```bash
go run ./cmd/brokers-sync -ibkr data/ibrk-all-tmp.csv
go run ./cmd/brokers-sync -revolut data/revolut-all-tmp.csv -no-prices
```

## Flags

| Flag                        | Default      | Description                           |
|-----------------------------|--------------|---------------------------------------|
| `-revolut <file>`           | —            | Revolut CSV export                    |
| `-ibkr <file>`              | —            | IBKR Transaction History CSV export   |
| `-t212 <file>`              | —            | Trading 212 CSV export                |
| `-format <text\|json\|csv>` | `text`       | Output format                         |
| `-out <path>`               | stdout / `.` | Output file (json) or directory (csv) |
| `-no-prices`                | false        | Skip Yahoo Finance price fetch        |

## Output sections (text mode)

- **Open Positions** — quantity, average cost, cost basis, market value, unrealized P&L and % (when prices available)
- **Realized P&L by Symbol** — FIFO-based realized gain/loss per ticker, all time
- **All Time / YTD / MTD** — realized P&L, net dividends, tax withheld, fees, deposits, withdrawals, buy/sell volume
- **Year-by-Year Breakdown** — same metrics bucketed per calendar year
- **Dividends by Symbol** — gross, tax withheld, net per ticker, all time

## Project structure

```
cmd/brokers-sync/main.go    CLI entry point and text output
internal/
  model/transaction.go      Normalized Transaction type and TxType enum
  parser/
    revolut.go              Revolut CSV parser
    ibkr.go                 IBKR multi-section CSV parser
    trading212.go           Trading 212 CSV parser
    util.go                 Shared helpers (column mapping, date parsing, ID hashing)
  ledger/ledger.go          FIFO lot tracking, realized P&L, stock split adjustment
  stats/stats.go            Period aggregations, unrealized P&L enrichment
  prices/yahoo.go           Yahoo Finance chart API, parallel price fetch
  output/report.go          JSON and CSV report writers
```
