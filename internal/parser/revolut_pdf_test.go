package parser

import (
	"testing"

	"brokers-sync/internal/model"
)

// Representative rows from a real Revolut PDF (pdftotext -layout output), chosen to
// pin down the parsing decisions that actually matter:
//   - the whole-share "Transfer" rows the CSV export OMITS (the bug that started this)
//   - the fractional "Transfer" row from Revolut's UK->EU entity migration, which
//     must be dropped (real external transfers only move whole shares)
//   - the symbol glued to the type ("GOOGLTrade") and thousands separators in amounts
//   - each type mapping to the correct model.TxType
const revolutPDFSample = `USD Transactions
Date                       SymbolType              Quantity    Price       SideValue        Fees      Commission

04 Mar 2020 19:20:51 GMT AAPL Trade - Market       0.296       US$298.72 Buy US$88.42       US$0           US$0
15 May 2020 06:44:07 GMT AAPL Dividend                                        US$0.22       US$0           US$0
31 Aug 2020 08:18:44 GMT AAPL Stock split          76.26614496                  US$0          US$0           US$0
14 Oct 2020 16:05:33 GMT AAPL Trade - Market       41.2982572 US$121.07     Sell US$4,999.88 US$0.12        US$0
30 Apr 2020 23:09:09 GMT         Custody fee                                  -US$0.04      US$0           US$0
27 Apr 2023 06:38:17 GMT AAPL Transfer                                                             -12                          US$0         US$0           US$0
25 Jun 2023 13:20:25 GMT AMZN Transfer from Revolut Trading Ltd to Revolut Securities Europe UAB 0.0971436                      US$0         US$0           US$0
16 Jun 2026 15:10:30 GMT GOOGLTrade - Market       4           US$374.52   Sell US$1,498.05 US$0.04        US$0
`

func TestParseRevolutPDFText(t *testing.T) {
	txs, err := parseRevolutPDFText(revolutPDFSample)
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 7 {
		t.Fatalf("got %d transactions, want 7", len(txs))
	}

	type want struct {
		typ  model.TxType
		sym  string
		qty  float64
		net  float64
		note string
	}
	wants := []want{
		{model.TxBuy, "AAPL", 0.296, 88.42, "buy: qty/price/value parsed"},
		{model.TxDividend, "AAPL", 0, 0.22, "dividend: value only, no qty"},
		{model.TxStockSplit, "AAPL", 76.26614496, 0, "split: delta qty, no value"},
		{model.TxSell, "AAPL", 41.2982572, 4999.88, "sell: thousands separator stripped"},
		{model.TxFee, "", 0, -0.04, "custody fee: negative, no symbol"},
		{model.TxTransferOut, "AAPL", 12, 0, "whole-share transfer (CSV omits this)"},
		{model.TxSell, "GOOGL", 4, 1498.05, "symbol glued to type must still parse"},
	}

	for i, w := range wants {
		got := txs[i]
		if got.Type != w.typ || got.Symbol != w.sym {
			t.Errorf("[%s] type/sym: got %s/%q want %s/%q", w.note, got.Type, got.Symbol, w.typ, w.sym)
		}
		if !approx(got.Quantity, w.qty) {
			t.Errorf("[%s] qty: got %v want %v", w.note, got.Quantity, w.qty)
		}
		if !approx(got.Net, w.net) {
			t.Errorf("[%s] net: got %v want %v", w.note, got.Net, w.net)
		}
	}

	// The transfer rows must carry a positive quantity so the ledger removes shares
	// (the whole point — the CSV lacked them and positions never closed).
	if txs[5].Quantity <= 0 {
		t.Errorf("whole transfer qty must be positive for removal, got %v", txs[5].Quantity)
	}

	// The fractional "Transfer" row (AMZN, entity-migration residue — real
	// external transfers only move whole shares) must be dropped, not parsed
	// as a TRANSFER_OUT.
	for _, tx := range txs {
		if tx.Symbol == "AMZN" {
			t.Errorf("fractional transfer row should have been dropped, got %+v", tx)
		}
	}
}

func approx(a, b float64) bool {
	d := a - b
	return d < 1e-9 && d > -1e-9
}
