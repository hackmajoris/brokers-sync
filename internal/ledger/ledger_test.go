package ledger

import (
	"testing"
	"time"

	"brokers-sync/internal/model"
)

// Transfer out moves shares to another broker: it must remove them FIFO WITHOUT
// booking realized P&L or proceeds — otherwise a transfer would masquerade as a
// sale and distort realized gains.
func TestTransferOutRemovesSharesWithoutPnL(t *testing.T) {
	d := func(s string) time.Time {
		tm, _ := time.Parse("2006-01-02", s)
		return tm
	}
	txs := []model.Transaction{
		{Date: d("2026-01-02"), Type: model.TxBuy, Symbol: "VICI", Currency: "USD", Quantity: 10, Net: -300},
		{Date: d("2026-05-27"), Type: model.TxTransferOut, Symbol: "VICI", Currency: "USD", Quantity: 6, Net: 180},
	}

	l := New()
	l.Process(txs)

	p := l.Positions["VICI"]
	if p == nil {
		t.Fatal("VICI position missing")
	}
	if p.Quantity != 4 {
		t.Errorf("Quantity: got %.4f, want 4 (10 bought − 6 transferred out)", p.Quantity)
	}
	// Cost basis drops proportionally: 300 × 4/10 = 120.
	if p.TotalCost < 119.99 || p.TotalCost > 120.01 {
		t.Errorf("TotalCost: got %.4f, want 120", p.TotalCost)
	}
	if len(l.Realized) != 0 {
		t.Errorf("Realized: got %d events, want 0 (transfer is not a sale)", len(l.Realized))
	}
}

// Transfer in moves shares in from another broker: it must establish a lot at the
// carried cost basis WITHOUT any cash flow or realized P&L, so later sells draw
// these shares FIFO with the correct basis instead of a phantom zero cost.
func TestTransferInEstablishesLotAtCostBasis(t *testing.T) {
	d := func(s string) time.Time {
		tm, _ := time.Parse("2006-01-02", s)
		return tm
	}
	txs := []model.Transaction{
		{Date: d("2026-05-27"), Type: model.TxTransferIn, Symbol: "AMZN", Currency: "USD", Quantity: 15, Net: 3979.35},
		{Date: d("2026-06-01"), Type: model.TxSell, Symbol: "AMZN", Currency: "USD", Quantity: 5, Net: 1500},
	}

	l := New()
	l.Process(txs)

	p := l.Positions["AMZN"]
	if p == nil {
		t.Fatal("AMZN position missing")
	}
	if p.Quantity != 10 {
		t.Errorf("Quantity: got %.4f, want 10 (15 in − 5 sold)", p.Quantity)
	}
	// Remaining basis: 3979.35 × 10/15 = 2652.90.
	if p.TotalCost < 2652.89 || p.TotalCost > 2652.91 {
		t.Errorf("TotalCost: got %.4f, want 2652.90", p.TotalCost)
	}
	// Sold 5 with basis 3979.35 × 5/15 = 1326.45, proceeds 1500 → P&L 173.55.
	if len(l.Realized) != 1 {
		t.Fatalf("Realized: got %d events, want 1", len(l.Realized))
	}
	if pnl := l.Realized[0].PnL; pnl < 173.54 || pnl > 173.56 {
		t.Errorf("PnL: got %.4f, want 173.55 (basis carried from transfer, not zero)", pnl)
	}
}

// A delisting write-off (Liquidate) must close the ENTIRE remaining position even
// when the exported sell quantity is smaller than the holding (e.g. an unrecorded
// reverse split) — otherwise phantom shares linger forever.
func TestLiquidateClosesWholePosition(t *testing.T) {
	d := func(s string) time.Time {
		tm, _ := time.Parse("2006-01-02", s)
		return tm
	}
	txs := []model.Transaction{
		{Date: d("2021-01-20"), Type: model.TxBuy, Symbol: "GTCH", Currency: "USD", Quantity: 6474.255, Net: -591.91},
		{Date: d("2026-05-13"), Type: model.TxSell, Symbol: "GTCH", Currency: "USD", Quantity: 129.4851, Price: 0.0001, Net: 0.01, Liquidate: true},
	}

	l := New()
	l.Process(txs)

	if p := l.Positions["GTCH"]; p != nil && p.Quantity > 1e-6 {
		t.Errorf("GTCH still open: qty %.4f, want 0 (full liquidation)", p.Quantity)
	}
	if len(l.Realized) != 1 {
		t.Fatalf("Realized: got %d events, want 1", len(l.Realized))
	}
}
