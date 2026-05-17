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
	Currency  string // currency the cost basis is denominated in
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
	Currency  string // currency the proceeds and P&L are denominated in
	Quantity  float64
	Proceeds  float64 // positive
	CostBasis float64 // positive
	PnL       float64 // Proceeds - CostBasis
}

// Ledger processes transactions in chronological order and tracks:
// - open positions (FIFO lots)
// - realized P&L
// - dividends
// - explicit fees (custody, platform)
// - per-trade commissions from buy/sell transactions
type Ledger struct {
	Positions   map[string]*Position
	Realized    []RealizedTx
	Dividends   []model.Transaction
	Fees        []model.Transaction // TxFee type (custody fees, platform charges)
	Commissions []model.Transaction // Commission field on buy/sell transactions
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
	case model.TxTransferOut:
		l.transferOut(tx)
	case model.TxTransferIn:
		l.transferIn(tx)
	case model.TxDividend, model.TxTaxWithholding:
		l.Dividends = append(l.Dividends, tx)
	case model.TxFee:
		l.Fees = append(l.Fees, tx)
	}
	// Track per-trade commission separately (IBKR, T212).
	// Note: commission is already included in tx.Net / cost basis — this is purely for reporting.
	if tx.Commission != 0 && (tx.Type == model.TxBuy || tx.Type == model.TxSell) {
		l.Commissions = append(l.Commissions, tx)
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

	// Three conventions across brokers:
	//   IBKR:           Net < 0  (cash outflow, e.g. -1167.95)
	//   Revolut/T212:   Net > 0  (absolute cost in account currency, e.g. 134.24)
	//   no Net at all:  Net == 0 → fall back to qty × price (same-currency only)
	var cost float64
	switch {
	case tx.Net < 0:
		cost = -tx.Net
	case tx.Net > 0:
		cost = tx.Net
	default:
		cost = tx.Quantity * tx.Price
	}
	p.Lots = append(p.Lots, Lot{
		Date:      tx.Date,
		Broker:    tx.Broker,
		Currency:  tx.Currency,
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

	// Delisting write-off: the broker's stated quantity is unreliable (e.g. an
	// unrecorded reverse split), so close the whole remaining position.
	remaining := tx.Quantity
	if tx.Liquidate {
		remaining = p.Quantity
	}
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

	pnl := proceeds - costBasis
	if tx.BrokerPnL != 0 {
		pnl = tx.BrokerPnL
	}
	l.Realized = append(l.Realized, RealizedTx{
		Date:      tx.Date,
		Symbol:    tx.Symbol,
		Broker:    tx.Broker,
		Currency:  tx.Currency,
		Quantity:  tx.Quantity,
		Proceeds:  proceeds,
		CostBasis: costBasis,
		PnL:       pnl,
	})
}

// transferOut removes shares moved to another broker. It drops lots FIFO like a
// sell but records NO realized P&L and NO proceeds — the position simply leaves.
func (l *Ledger) transferOut(tx model.Transaction) {
	if tx.Symbol == "" || tx.Quantity == 0 {
		return
	}
	p := l.positionFor(tx.Symbol)

	remaining := tx.Quantity
	for i := 0; i < len(p.Lots) && remaining > 0; i++ {
		lot := &p.Lots[i]
		if lot.Quantity == 0 {
			continue
		}
		take := min(lot.Quantity, remaining)
		lotCostPerShare := lot.CostBasis / lot.Quantity
		lot.Quantity -= take
		lot.CostBasis -= take * lotCostPerShare
		p.Quantity -= take
		p.TotalCost -= take * lotCostPerShare
		remaining -= take
	}

	active := p.Lots[:0]
	for _, lot := range p.Lots {
		if lot.Quantity > 1e-9 {
			active = append(active, lot)
		}
	}
	p.Lots = active
}

// transferIn adds shares moved in from another broker. It creates a lot at the
// carried cost basis (tx.Net, positive) with NO cash flow and NO commission —
// the position simply arrives. Mirrors buy but takes cost basis directly rather
// than deriving it from a cash amount.
func (l *Ledger) transferIn(tx model.Transaction) {
	if tx.Symbol == "" || tx.Quantity == 0 {
		return
	}
	p := l.positionFor(tx.Symbol)
	p.Lots = append(p.Lots, Lot{
		Date:      tx.Date,
		Broker:    tx.Broker,
		Currency:  tx.Currency,
		Quantity:  tx.Quantity,
		CostBasis: tx.Net,
	})
	p.Quantity += tx.Quantity
	p.TotalCost += tx.Net
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
