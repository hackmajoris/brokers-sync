package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/model"
	"brokers-sync/internal/stats"
)

// Report is the full serializable report, suitable for JSON export or chart consumption.
type Report struct {
	GeneratedAt       time.Time                `json:"generated_at"`
	BaseCurrency      string                   `json:"base_currency"`
	OpenPositions     []stats.PositionSummary  `json:"open_positions"`
	RealizedBySymbol  []RealizedRow            `json:"realized_pnl_by_symbol"`
	AllTime           stats.PeriodSummary      `json:"all_time"`
	YTD               stats.PeriodSummary      `json:"ytd"`
	MTD               stats.PeriodSummary      `json:"mtd"`
	ByYear            []stats.PeriodSummary    `json:"by_year"`
	DividendsBySymbol []stats.DividendBySymbol `json:"dividends_by_symbol"`
	Transactions      []model.Transaction      `json:"transactions"`
}

type RealizedRow struct {
	Symbol string  `json:"symbol"`
	PnL    float64 `json:"pnl"`
}

func Build(s stats.Summary, realized []ledger.RealizedTx, allTxs []model.Transaction) Report {
	bySymbol := stats.RealizedBySymbol(realized)
	type kv struct {
		sym string
		pnl float64
	}
	var rows []kv
	for sym, pnl := range bySymbol {
		rows = append(rows, kv{sym, pnl})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pnl > rows[j].pnl })

	realizedRows := make([]RealizedRow, len(rows))
	for i, r := range rows {
		realizedRows[i] = RealizedRow{Symbol: r.sym, PnL: r.pnl}
	}

	return Report{
		GeneratedAt:       time.Now().UTC(),
		BaseCurrency:      s.BaseCurrency,
		OpenPositions:     s.OpenPositions,
		RealizedBySymbol:  realizedRows,
		AllTime:           s.AllTime,
		YTD:               s.YTD,
		MTD:               s.MTD,
		ByYear:            s.ByYear,
		DividendsBySymbol: s.BySymbol,
		Transactions:      allTxs,
	}
}

func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// WriteCSV writes one CSV file per section into dir.
func WriteCSV(dir string, r Report) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if err := writePositionsCSV(filepath.Join(dir, "positions.csv"), r.OpenPositions); err != nil {
		return fmt.Errorf("positions.csv: %w", err)
	}
	if err := writeRealizedCSV(filepath.Join(dir, "realized_by_symbol.csv"), r.RealizedBySymbol); err != nil {
		return fmt.Errorf("realized_by_symbol.csv: %w", err)
	}
	if err := writeDividendsCSV(filepath.Join(dir, "dividends_by_symbol.csv"), r.DividendsBySymbol); err != nil {
		return fmt.Errorf("dividends_by_symbol.csv: %w", err)
	}
	if err := writeSummaryByYearCSV(filepath.Join(dir, "summary_by_year.csv"), r.ByYear); err != nil {
		return fmt.Errorf("summary_by_year.csv: %w", err)
	}
	if err := writeTransactionsCSV(filepath.Join(dir, "transactions.csv"), r.Transactions); err != nil {
		return fmt.Errorf("transactions.csv: %w", err)
	}
	return nil
}

func writePositionsCSV(path string, positions []stats.PositionSummary) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"symbol", "currency", "quantity", "avg_cost", "total_cost", "current_price", "market_value", "unrealized_pnl", "unrealized_pct"})
	for _, p := range positions {
		_ = w.Write([]string{
			p.Symbol,
			p.Currency,
			ff(p.Quantity),
			ff(p.AvgCost),
			ff(p.TotalCost),
			ff(p.CurrentPrice),
			ff(p.MarketValue),
			ff(p.UnrealizedPnL),
			ff(p.UnrealizedPct),
		})
	}
	w.Flush()
	return w.Error()
}

func writeRealizedCSV(path string, rows []RealizedRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"symbol", "realized_pnl"})
	for _, r := range rows {
		_ = w.Write([]string{r.Symbol, ff(r.PnL)})
	}
	w.Flush()
	return w.Error()
}

func writeDividendsCSV(path string, divs []stats.DividendBySymbol) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"symbol", "gross", "tax_withheld", "net"})
	for _, d := range divs {
		_ = w.Write([]string{d.Symbol, ff(d.Gross), ff(d.TaxWithheld), ff(d.Net)})
	}
	w.Flush()
	return w.Error()
}

func writeSummaryByYearCSV(path string, years []stats.PeriodSummary) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"year", "realized_pnl", "dividends_net", "tax_withheld", "commissions", "fees", "deposits", "withdrawals", "buy_volume", "sell_volume", "gain_pct"})
	for _, p := range years {
		_ = w.Write([]string{
			p.Label,
			ff(p.Realized),
			ff(p.Dividends),
			ff(-p.TaxWithheld),
			ff(p.Commissions),
			ff(p.Fees),
			ff(p.Deposits),
			ff(p.Withdrawals),
			ff(p.BuyVolume),
			ff(p.SellVolume),
			ff(p.GainPct),
		})
	}
	w.Flush()
	return w.Error()
}

func writeTransactionsCSV(path string, txs []model.Transaction) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"id", "date", "broker", "account", "type", "symbol", "isin", "name", "quantity", "price", "currency", "gross", "commission", "net", "fx_rate", "notes"})
	for _, tx := range txs {
		_ = w.Write([]string{
			tx.ID,
			tx.Date.Format(time.RFC3339),
			tx.Broker,
			tx.Account,
			string(tx.Type),
			tx.Symbol,
			tx.ISIN,
			tx.Name,
			ff(tx.Quantity),
			ff(tx.Price),
			tx.Currency,
			ff(tx.Gross),
			ff(tx.Commission),
			ff(tx.Net),
			ff(tx.FXRate),
			tx.Notes,
		})
	}
	w.Flush()
	return w.Error()
}

func ff(v float64) string {
	return strconv.FormatFloat(v, 'f', 6, 64)
}
