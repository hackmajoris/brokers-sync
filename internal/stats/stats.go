package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/model"
)

// PeriodSummary aggregates key metrics over a time range.
type PeriodSummary struct {
	Label       string    `json:"label"`
	Start       time.Time `json:"start,omitempty"`
	End         time.Time `json:"end,omitempty"`
	Realized    float64   `json:"realized_pnl"`
	Dividends   float64   `json:"dividends_net"`
	TaxWithheld float64   `json:"tax_withheld"`
	Fees        float64   `json:"fees"`
	Deposits    float64   `json:"deposits"`
	Withdrawals float64   `json:"withdrawals"`
	BuyVolume   float64   `json:"buy_volume"`
	SellVolume  float64   `json:"sell_volume"`
}

// DividendBySymbol totals net dividends (after tax) per ticker.
type DividendBySymbol struct {
	Symbol      string  `json:"symbol"`
	Gross       float64 `json:"gross"`
	TaxWithheld float64 `json:"tax_withheld"`
	Net         float64 `json:"net"`
}

// PositionSummary is a position enriched with optional live price data.
type PositionSummary struct {
	Symbol        string  `json:"symbol"`
	Quantity      float64 `json:"quantity"`
	AvgCost       float64 `json:"avg_cost"`
	TotalCost     float64 `json:"total_cost"`
	CurrentPrice  float64 `json:"current_price,omitempty"`
	MarketValue   float64 `json:"market_value,omitempty"`
	UnrealizedPnL float64 `json:"unrealized_pnl,omitempty"`
	UnrealizedPct float64 `json:"unrealized_pct_omitempty,omitempty"`
}

// Summary aggregates stats from a fully-processed ledger + raw transactions.
type Summary struct {
	OpenPositions []PositionSummary
	Realized      []ledger.RealizedTx
	AllTime       PeriodSummary
	YTD           PeriodSummary
	MTD           PeriodSummary
	ByYear        []PeriodSummary
	BySymbol      []DividendBySymbol
}

// Compute builds the Summary from the ledger state and all transactions.
func Compute(l *ledger.Ledger, allTxs []model.Transaction, now time.Time) Summary {
	ytdStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	mtdStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Determine year range from transaction data
	minYear, maxYear := now.Year(), now.Year()
	for _, tx := range allTxs {
		y := tx.Date.Year()
		if y < minYear {
			minYear = y
		}
		if y > maxYear {
			maxYear = y
		}
	}

	yearMap := make(map[int]*PeriodSummary, maxYear-minYear+1)
	for y := minYear; y <= maxYear; y++ {
		start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(y, 12, 31, 23, 59, 59, 0, time.UTC)
		yearMap[y] = &PeriodSummary{Label: fmt.Sprintf("%d", y), Start: start, End: end}
	}

	s := Summary{
		Realized: l.Realized,
		YTD:      PeriodSummary{Label: "YTD", Start: ytdStart, End: now},
		MTD:      PeriodSummary{Label: "MTD", Start: mtdStart, End: now},
		AllTime:  PeriodSummary{Label: "All Time"},
	}

	// Open positions
	type posRow struct {
		sym string
		pos *ledger.Position
	}
	var posRows []posRow
	for sym, pos := range l.Positions {
		if pos.Quantity > 1e-6 {
			posRows = append(posRows, posRow{sym, pos})
		}
	}
	sort.Slice(posRows, func(i, j int) bool { return posRows[i].sym < posRows[j].sym })
	for _, r := range posRows {
		s.OpenPositions = append(s.OpenPositions, PositionSummary{
			Symbol:    r.sym,
			Quantity:  r.pos.Quantity,
			AvgCost:   r.pos.AvgCost(),
			TotalCost: r.pos.TotalCost,
		})
	}

	// Realized P&L
	for _, r := range l.Realized {
		y := r.Date.Year()
		s.AllTime.Realized += r.PnL
		if !r.Date.Before(ytdStart) {
			s.YTD.Realized += r.PnL
		}
		if !r.Date.Before(mtdStart) {
			s.MTD.Realized += r.PnL
		}
		if p, ok := yearMap[y]; ok {
			p.Realized += r.PnL
		}
	}

	// Dividends + tax withholding
	divMap := make(map[string]*DividendBySymbol)
	for _, tx := range l.Dividends {
		sym := tx.Symbol
		if sym == "" {
			sym = "(no symbol)"
		}
		d := divMap[sym]
		if d == nil {
			d = &DividendBySymbol{Symbol: sym}
			divMap[sym] = d
		}
		if tx.Type == model.TxTaxWithholding {
			d.TaxWithheld += -tx.Net
		} else {
			d.Gross += tx.Net
		}
		d.Net = d.Gross - d.TaxWithheld
	}

	for _, tx := range l.Dividends {
		y := tx.Date.Year()
		net := tx.Net
		s.AllTime.Dividends += net
		if tx.Type == model.TxTaxWithholding {
			s.AllTime.TaxWithheld += net
		}
		if !tx.Date.Before(ytdStart) {
			s.YTD.Dividends += net
			if tx.Type == model.TxTaxWithholding {
				s.YTD.TaxWithheld += net
			}
		}
		if !tx.Date.Before(mtdStart) {
			s.MTD.Dividends += net
		}
		if p, ok := yearMap[y]; ok {
			p.Dividends += net
			if tx.Type == model.TxTaxWithholding {
				p.TaxWithheld += net
			}
		}
	}

	// Fees
	for _, tx := range l.Fees {
		y := tx.Date.Year()
		s.AllTime.Fees += tx.Net
		if !tx.Date.Before(ytdStart) {
			s.YTD.Fees += tx.Net
		}
		if !tx.Date.Before(mtdStart) {
			s.MTD.Fees += tx.Net
		}
		if p, ok := yearMap[y]; ok {
			p.Fees += tx.Net
		}
	}

	// Deposits, withdrawals, trade volume
	for _, tx := range allTxs {
		y := tx.Date.Year()
		yp := yearMap[y]

		switch tx.Type {
		case model.TxDeposit:
			s.AllTime.Deposits += tx.Net
			if !tx.Date.Before(ytdStart) {
				s.YTD.Deposits += tx.Net
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.Deposits += tx.Net
			}
			if yp != nil {
				yp.Deposits += tx.Net
			}
		case model.TxWithdrawal:
			s.AllTime.Withdrawals += tx.Net
			if !tx.Date.Before(ytdStart) {
				s.YTD.Withdrawals += tx.Net
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.Withdrawals += tx.Net
			}
			if yp != nil {
				yp.Withdrawals += tx.Net
			}
		case model.TxBuy:
			vol := tx.Quantity * tx.Price
			s.AllTime.BuyVolume += vol
			if !tx.Date.Before(ytdStart) {
				s.YTD.BuyVolume += vol
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.BuyVolume += vol
			}
			if yp != nil {
				yp.BuyVolume += vol
			}
		case model.TxSell:
			vol := tx.Quantity * tx.Price
			s.AllTime.SellVolume += vol
			if !tx.Date.Before(ytdStart) {
				s.YTD.SellVolume += vol
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.SellVolume += vol
			}
			if yp != nil {
				yp.SellVolume += vol
			}
		}
	}

	// Flatten yearMap in ascending order
	for y := minYear; y <= maxYear; y++ {
		s.ByYear = append(s.ByYear, *yearMap[y])
	}

	// Sort dividend summaries by gross descending
	for _, d := range divMap {
		s.BySymbol = append(s.BySymbol, *d)
	}
	sort.Slice(s.BySymbol, func(i, j int) bool {
		return s.BySymbol[i].Gross > s.BySymbol[j].Gross
	})

	return s
}

// EnrichWithPrices adds live price data to open positions.
// prices is a map of symbol (or Yahoo-normalized symbol) → current price.
func EnrichWithPrices(s *Summary, prices map[string]float64) {
	for i := range s.OpenPositions {
		p := &s.OpenPositions[i]
		price, ok := prices[p.Symbol]
		if !ok {
			// Try Yahoo-normalized form (e.g. "BRK-B" for "BRK B")
			norm := strings.ReplaceAll(p.Symbol, " ", "-")
			price, ok = prices[norm]
		}
		if !ok {
			continue
		}
		p.CurrentPrice = price
		p.MarketValue = price * p.Quantity
		p.UnrealizedPnL = p.MarketValue - p.TotalCost
		if p.TotalCost > 0.01 {
			p.UnrealizedPct = p.UnrealizedPnL / p.TotalCost * 100
		}
	}
}

// RealizedBySymbol returns realized P&L grouped by symbol.
func RealizedBySymbol(realized []ledger.RealizedTx) map[string]float64 {
	m := make(map[string]float64)
	for _, r := range realized {
		m[r.Symbol] += r.PnL
	}
	return m
}
