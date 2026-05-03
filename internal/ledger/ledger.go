package ledger

import (
	"sort"
	"time"

	"brokers-sync/internal/model"
)

// Lot represents a single purchase lot.
type Lot struct {
	Date      time.Time
	Broker    string
	Quantity  float64
	CostBasis float64 // total cost for this lot (positive)
}

// Position holds the current state of a symbol.
type Position struct {
	Symbol    string
	Lots      []Lot
	Quantity  float64 // sum of remaining lot quantities
	TotalCost float64 // sum of remaining lot cost bases
}

// AvgCost returns average cost per share for the position.
func (p *Position) AvgCost() float64 {
	if p.Quantity == 0 {
		return 0
	}
	return p.TotalCost / p.Quantity
}

// RealizedTx captures a realized gain/loss event.
type RealizedTx struct {
	Date      time.Time
	Symbol    string
	Broker    string
	Quantity  float64
	Proceeds  float64 // positive
	CostBasis float64 // positive
	PnL       float64 // Proceeds - CostBasis
}

// Ledger processes transactions in chronological order and tracks:
// - open positions (FIFO lots)
// - realized P&L
// - dividends
type Ledger struct {
	Positions map[string]*Position
	Realized  []RealizedTx
	Dividends []model.Transaction
	Fees      []model.Transaction
}

func New() *Ledger {
	return &Ledger{
		Positions: make(map[string]*Position),
	}
}

// Process applies a slice of transactions (must be sorted by date ascending).
func (l *Ledger) Process(txs []model.Transaction) {
	sorted := make([]model.Transaction, len(txs))
	copy(sorted, txs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Date.Before(sorted[j].Date)
	})

	for _, tx := range sorted {
		l.apply(tx)
	}
}

func (l *Ledger) apply(tx model.Transaction) {
	switch tx.Type {
	case model.TxBuy:
		l.buy(tx)
	case model.TxSell:
		l.sell(tx)
	case model.TxStockSplit:
		l.split(tx)
	case model.TxDividend, model.TxTaxWithholding:
		l.Dividends = append(l.Dividends, tx)
	case model.TxFee:
		l.Fees = append(l.Fees, tx)
	}
}

func (l *Ledger) positionFor(symbol string) *Position {
	if p, ok := l.Positions[symbol]; ok {
		return p
	}
	p := &Position{Symbol: symbol}
	l.Positions[symbol] = p
	return p
}

func (l *Ledger) buy(tx model.Transaction) {
	if tx.Symbol == "" || tx.Quantity == 0 {
		return
	}
	p := l.positionFor(tx.Symbol)
	cost := -tx.Net // Net is negative for buys (cash outflow)
	if cost < 0 {
		cost = tx.Quantity * tx.Price
	}
	p.Lots = append(p.Lots, Lot{
		Date:      tx.Date,
		Broker:    tx.Broker,
		Quantity:  tx.Quantity,
		CostBasis: cost,
	})
	p.Quantity += tx.Quantity
	p.TotalCost += cost
}

// sell removes lots FIFO and records realized P&L.
func (l *Ledger) sell(tx model.Transaction) {
	if tx.Symbol == "" || tx.Quantity == 0 {
		return
	}
	p := l.positionFor(tx.Symbol)

	proceeds := tx.Net // positive for sells
	if proceeds <= 0 {
		proceeds = tx.Quantity * tx.Price
	}

	remaining := tx.Quantity
	var costBasis float64

	for i := 0; i < len(p.Lots) && remaining > 0; i++ {
		lot := &p.Lots[i]
		if lot.Quantity == 0 {
			continue
		}
		take := min(lot.Quantity, remaining)
		lotCostPerShare := lot.CostBasis / lot.Quantity
		costBasis += take * lotCostPerShare
		lot.Quantity -= take
		lot.CostBasis -= take * lotCostPerShare
		p.Quantity -= take
		p.TotalCost -= take * lotCostPerShare
		remaining -= take
	}

	// Remove exhausted lots
	active := p.Lots[:0]
	for _, lot := range p.Lots {
		if lot.Quantity > 1e-9 {
			active = append(active, lot)
		}
	}
	p.Lots = active

	l.Realized = append(l.Realized, RealizedTx{
		Date:      tx.Date,
		Symbol:    tx.Symbol,
		Broker:    tx.Broker,
		Quantity:  tx.Quantity,
		Proceeds:  proceeds,
		CostBasis: costBasis,
		PnL:       proceeds - costBasis,
	})
}

// split adjusts lot quantities and cost bases for a stock split.
// tx.Quantity is the additional shares received (delta), not the ratio.
func (l *Ledger) split(tx model.Transaction) {
	if tx.Symbol == "" {
		return
	}
	p, ok := l.Positions[tx.Symbol]
	if !ok || p.Quantity == 0 {
		return
	}
	ratio := (p.Quantity + tx.Quantity) / p.Quantity
	for i := range p.Lots {
		p.Lots[i].Quantity *= ratio
		// cost basis per share decreases proportionally; total basis unchanged
	}
	p.Quantity += tx.Quantity
	// TotalCost remains the same — cost basis per share has changed
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
