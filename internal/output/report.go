package output

import (
	"encoding/json"
	"io"
	"sort"
	"time"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/model"
	"brokers-sync/internal/stats"
)

// Report is the full serializable report, suitable for JSON export or chart consumption.
type Report struct {
	GeneratedAt       time.Time                `json:"generated_at"`
	BaseCurrency      string                   `json:"base_currency"`
	CashBalance       float64                  `json:"cash_balance"`
	Brokers           []BrokerReport           `json:"brokers"`
	OpenPositions     []stats.PositionSummary  `json:"open_positions"`
	RealizedBySymbol  []RealizedRow            `json:"realized_pnl_by_symbol"`
	AllTime           stats.PeriodSummary      `json:"all_time"`
	YTD               stats.PeriodSummary      `json:"ytd"`
	MTD               stats.PeriodSummary      `json:"mtd"`
	ByYear            []stats.PeriodSummary    `json:"by_year"`
	DividendsBySymbol []stats.DividendBySymbol `json:"dividends_by_symbol"`
	Transactions      []TxRow                  `json:"transactions"`
}

// TxRow is a single ledger transaction in its broker's native currency (no FX conversion).
type TxRow struct {
	ID         string    `json:"id"`
	Date       time.Time `json:"date"`
	Broker     string    `json:"broker"`
	Type       string    `json:"type"`
	Symbol     string    `json:"symbol"`
	Name       string    `json:"name"`
	Quantity   float64   `json:"quantity"`
	Price      float64   `json:"price"`
	Currency   string    `json:"currency"`
	Gross      float64   `json:"gross"`
	Commission float64   `json:"commission"`
	Net        float64   `json:"net"`
	Notes      string    `json:"notes,omitempty"`
}

func txRows(txs []model.Transaction) []TxRow {
	sorted := make([]model.Transaction, len(txs))
	copy(sorted, txs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date.After(sorted[j].Date) })
	out := make([]TxRow, len(sorted))
	for i, t := range sorted {
		out[i] = TxRow{
			ID:         t.ID,
			Date:       t.Date,
			Broker:     t.Broker,
			Type:       string(t.Type),
			Symbol:     t.Symbol,
			Name:       t.Name,
			Quantity:   t.Quantity,
			Price:      t.Price,
			Currency:   t.Currency,
			Gross:      t.Gross,
			Commission: t.Commission,
			Net:        t.Net,
			Notes:      t.Notes,
		}
	}
	return out
}

// BrokerReport mirrors Report but scoped to a single broker's native currency.
type BrokerReport struct {
	Name              string                   `json:"name"`
	BaseCurrency      string                   `json:"base_currency"`
	CashBalance       float64                  `json:"cash_balance"`
	OpenPositions     []stats.PositionSummary  `json:"open_positions"`
	RealizedBySymbol  []RealizedRow            `json:"realized_pnl_by_symbol"`
	AllTime           stats.PeriodSummary      `json:"all_time"`
	YTD               stats.PeriodSummary      `json:"ytd"`
	MTD               stats.PeriodSummary      `json:"mtd"`
	ByYear            []stats.PeriodSummary    `json:"by_year"`
	DividendsBySymbol []stats.DividendBySymbol `json:"dividends_by_symbol"`
}

type RealizedRow struct {
	Symbol string  `json:"symbol"`
	PnL    float64 `json:"pnl"`
}

func realizedRows(realized []ledger.RealizedTx) []RealizedRow {
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
	out := make([]RealizedRow, len(rows))
	for i, r := range rows {
		out[i] = RealizedRow{Symbol: r.sym, PnL: r.pnl}
	}
	return out
}

func BuildBrokerReport(name string, s stats.Summary, realized []ledger.RealizedTx) BrokerReport {
	return BrokerReport{
		Name:              name,
		BaseCurrency:      s.BaseCurrency,
		CashBalance:       s.CashBalance,
		OpenPositions:     s.OpenPositions,
		RealizedBySymbol:  realizedRows(realized),
		AllTime:           s.AllTime,
		YTD:               s.YTD,
		MTD:               s.MTD,
		ByYear:            s.ByYear,
		DividendsBySymbol: s.BySymbol,
	}
}

func Build(s stats.Summary, realized []ledger.RealizedTx, brokers []BrokerReport, txs []model.Transaction) Report {
	return Report{
		GeneratedAt:       time.Now().UTC(),
		BaseCurrency:      s.BaseCurrency,
		CashBalance:       s.CashBalance,
		Brokers:           brokers,
		OpenPositions:     s.OpenPositions,
		RealizedBySymbol:  realizedRows(realized),
		AllTime:           s.AllTime,
		YTD:               s.YTD,
		MTD:               s.MTD,
		ByYear:            s.ByYear,
		DividendsBySymbol: s.BySymbol,
		Transactions:      txRows(txs),
	}
}

func WriteJSON(w io.Writer, r Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
