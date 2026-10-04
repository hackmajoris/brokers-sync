package parser

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/model"
)

// writeXTBFixture builds a minimal XTB export. The window starts 2025-12-31,
// like a real "custom period" export:
//   - AAA.DE position 100 was bought 2025-06-01 (before the window, so absent
//     from Cash Operations): 4 of its 10 shares were sold in-window, 6 are
//     still open.
//   - BBB.DE position 200 was bought and sold entirely inside the window.
func writeXTBFixture(t *testing.T) string {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheets := map[string][][]any{
		"Closed Positions": {
			{"Account number", "123"},
			{"Closed Positions"},
			{"Date from (UTC)", "2025-12-31 22:00:00"},
			{"Date to (UTC)", "2026-09-14 11:48:38"},
			{"Instrument", "Ticker", "Volume", "Open Time (UTC)", "Close Time (UTC)", "Profit/Loss", "Gross Profit", "Purchase Value", "Sale Value", "Commission", "Position ID"},
			{"Aaa", "AAA.DE", "4", "2025-06-01 10:00:00", "2026-02-01 10:00:00", "40", "40", "400", "416", "0", "100"},
			{"Bbb", "BBB.DE", "5", "2026-01-10 10:00:00", "2026-03-01 10:00:00", "-20", "-10", "100", "90", "-10", "200"},
			{"Profit/loss", "", "", "", "", "20", "30"},
		},
		"Cash Operations": {
			{"Account number", "123"},
			{"Cash Operations"},
			{"Date from (UTC)", "2025-12-31 22:00:00"},
			{"Date to (UTC)", "2026-09-14 11:48:38"},
			{"Type", "Instrument", "Ticker", "Category", "Time", "Amount", "ID", "Comment", "Product", "Position ID"},
			{"Stock purchase", "Bbb", "BBB.DE", "STOCK", "2026-01-10 10:00:00", "-100", "1", "OPEN BUY 5 @ 20.00", "My Trades", "200"},
			{"Stock sell", "Aaa", "AAA.DE", "STOCK", "2026-02-01 10:00:01", "416", "2", "CLOSE BUY 4/10 @ 104.00", "My Trades", "100"},
			{"Stock sell", "Bbb", "BBB.DE", "STOCK", "2026-03-01 10:00:01", "90", "3", "CLOSE BUY 5 @ 18.00", "My Trades", "200"},
			{"Commission", "Bbb", "BBB.DE", "STOCK", "2026-03-01 10:00:01", "-10", "4", "BUY 5 @ 20.00", "My Trades", "200"},
			{"Dividend", "Aaa", "AAA.DE", "STOCK", "2026-04-01 10:00:00", "3.22", "5", "AAA.DE USD 0.22/ SHR", "My Trades"},
			{"Withholding tax", "Aaa", "AAA.DE", "STOCK", "2026-04-01 10:00:00", "-0.32", "6", "AAA.DE USD WHT 10%", "My Trades"},
			{"Transfer", "", "", "", "2026-04-02 10:00:00", "-1", "7", "Currency conversion, EUR to RON", "My Trades"},
			{"RO tax", "", "", "", "2026-02-02 11:00:00", "-0.19", "8", "RO tax AAA.DE 2026-02-01 (1.00 RON) ///OMI/1555481617/1.00//", "My Trades"},
			{"Total", "", "", "", "", "402.9"},
		},
		"Open Positions": {
			{"Account number", "123"},
			{"Open Positions"},
			{"Product", "Metric", "Amount", "Currency"},
			{"My Trades", "Open position value", "660", "EUR"},
			{"Product", "Instrument/Position", "Ticker", "Category", "Type", "Volume", "Value", "Current price", "Open price", "Open time (UTC)", "Gross Profit"},
			{"My Trades", "100", "AAA.DE", "", "BUY", "6", "660", "110", "100", "2025-06-01 10:00:00", "60"},
			{"My Trades", "Aaa", "AAA.DE", "STOCK", "", "6", "660", "", "100", "", "60"},
		},
	}
	for name, rows := range sheets {
		if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for i, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			if err := f.SetSheetRow(name, cell, &row); err != nil {
				t.Fatal(err)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "xtb.xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// A position bought before the export window must not be realized at zero
// cost: its sale should book XTB's profit, and the unsold remainder must stay
// in holdings at its real purchase cost.
func TestParseXTBPositionOpenedBeforeWindow(t *testing.T) {
	txs, err := ParseXTB(writeXTBFixture(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	l := ledger.New()
	l.Process(txs)

	var realized float64
	for _, r := range l.Realized {
		realized += r.PnL
	}
	// 40 (AAA) + -10 (BBB gross; the 10 commission is a separate fee row).
	// Treating the AAA sale as zero-cost would give 416 + -10.
	if math.Abs(realized-30) > 1e-9 {
		t.Errorf("realized P&L = %.2f, want 30.00", realized)
	}

	aaa := l.Positions["AAA.DE"]
	if aaa == nil || math.Abs(aaa.Quantity-6) > 1e-9 || math.Abs(aaa.TotalCost-600) > 1e-9 {
		t.Errorf("AAA.DE position = %+v, want 6 shares at cost 600", aaa)
	}
	if bbb := l.Positions["BBB.DE"]; bbb != nil && bbb.Quantity > 1e-9 {
		t.Errorf("BBB.DE still holds %.4f shares, want fully sold", bbb.Quantity)
	}

	// The pre-window lot is carried in, not bought: its purchase cash predates
	// the window, so it must not drain the cash balance.
	var carried []model.Transaction
	for _, tx := range txs {
		if tx.Type == model.TxTransferIn {
			carried = append(carried, tx)
		}
		if tx.Type == model.TxBuy && tx.Symbol == "AAA.DE" {
			t.Errorf("pre-window AAA.DE lot emitted as BUY; want TRANSFER_IN")
		}
	}
	if len(carried) != 1 || carried[0].Quantity != 10 || carried[0].Net != 1000 {
		t.Errorf("transfer-ins = %+v, want one AAA.DE lot of 10 shares costing 1000", carried)
	}
}

// Every cash row type in the export moves money, so none may be dropped.
func TestParseXTBCashRowTypes(t *testing.T) {
	txs, err := ParseXTB(writeXTBFixture(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := map[string]model.TxType{}
	for _, tx := range txs {
		got[tx.ID] = tx.Type
	}
	want := map[string]model.TxType{
		"xtb|4": model.TxFee,
		"xtb|5": model.TxDividend,
		"xtb|6": model.TxTaxWithholding,
		"xtb|7": model.TxForex,
		// Tax on a sale, not on a dividend: as withholding it would push
		// total dividends negative.
		"xtb|8": model.TxFee,
	}
	for id, typ := range want {
		if got[id] != typ {
			t.Errorf("%s: type = %q, want %q", id, got[id], typ)
		}
	}
}
