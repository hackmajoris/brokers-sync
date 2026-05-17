package parser

import (
	"strings"
	"testing"

	"brokers-sync/internal/model"
)

// IBKR Flex reports carry date only (no time or trade ID), so a multi-fill order
// shows as byte-identical rows. These are REAL separate fills — Dedup must keep
// them. Regression for BAC showing 32 instead of 22 when one of two same-day
// 10-share sells was silently dropped as a "duplicate".
func TestIBKRDistinctIDsForIdenticalSameDayFills(t *testing.T) {
	const csv = `Statement,Header,Field Name,Field Value
Transaction History,Header,Date,Account,Description,Transaction Type,Symbol,Quantity,Price,Price Currency,Gross Amount,Commission,Net Amount
Transaction History,Data,2025-11-14,Main,BANK OF AMERICA CORP,Sell,BAC,-10.0,52.615,USD,526.15,-0.35,525.80
Transaction History,Data,2025-11-14,Main,BANK OF AMERICA CORP,Sell,BAC,-10.0,52.615,USD,526.15,-0.35,525.80
`
	txs, err := ParseIBKR(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 2 {
		t.Fatalf("got %d transactions, want 2", len(txs))
	}
	if txs[0].ID == txs[1].ID {
		t.Fatalf("both fills share ID %q — Dedup would drop one, losing 10 shares", txs[0].ID)
	}

	// After Dedup both must survive (they are real fills, not a cross-file dup).
	kept, dropped := Dedup(txs)
	if len(kept) != 2 || len(dropped) != 0 {
		t.Errorf("Dedup: kept %d dropped %d, want kept 2 dropped 0", len(kept), len(dropped))
	}

	var sold float64
	for _, tx := range kept {
		if tx.Type == model.TxSell {
			sold += tx.Quantity
		}
	}
	if sold != 20 {
		t.Errorf("total sold = %.1f, want 20 (both fills counted)", sold)
	}
}

// A truly duplicated transaction across two overlapping statement files (same
// occurrence index in each) must still collapse to one.
func TestIBKRCrossFileDuplicateStillDedups(t *testing.T) {
	const csv = `Transaction History,Header,Date,Account,Description,Transaction Type,Symbol,Quantity,Price,Price Currency,Gross Amount,Commission,Net Amount
Transaction History,Data,2025-07-18,Main,BANK OF AMERICA CORP,Buy,BAC,7.0,47.13,USD,-329.91,-0.34,-330.25
`
	fileA, _ := ParseIBKR(strings.NewReader(csv))
	fileB, _ := ParseIBKR(strings.NewReader(csv))
	kept, dropped := Dedup(append(fileA, fileB...))
	if len(kept) != 1 || len(dropped) != 1 {
		t.Errorf("Dedup: kept %d dropped %d, want kept 1 dropped 1", len(kept), len(dropped))
	}
}

// Interest, ADR/other fees, and FX-translation adjustments are real cash
// movements IBKR reflects in the cash balance; before, they mapped to UNKNOWN
// and vanished from cash. They must map to cash-affecting types.
func TestIBKRInterestFeesAdjustmentMapToCash(t *testing.T) {
	const csv = `Transaction History,Header,Date,Account,Description,Transaction Type,Symbol,Quantity,Price,Price Currency,Gross Amount,Commission,Net Amount,Exchange Rate
Transaction History,Data,2026-01-31,Main,USD Credit Interest,Credit Interest,-,-,-,-,30.79,-,30.79,1.0
Transaction History,Data,2026-02-15,Main,USD Debit Interest,Debit Interest,-,-,-,-,-0.24,-,-0.24,1.0
Transaction History,Data,2026-03-01,Main,BABA ADR Fee,Other Fee,-,-,-,-,-0.32,-,-0.32,1.0
Transaction History,Data,2026-07-17,Main,FX Translations P&L,Adjustment,-,-,-,-,-5.01,-,-5.01,1.0
`
	txs, err := ParseIBKR(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	want := map[model.TxType]bool{
		model.TxInterest: false, // credit + debit interest
		model.TxFee:      false, // other fee
		model.TxForex:    false, // FX translation adjustment
	}
	for _, tx := range txs {
		if tx.Type == model.TxUnknown {
			t.Errorf("%q mapped to UNKNOWN — would be dropped from cash", tx.Notes)
		}
		want[tx.Type] = true
	}
	for typ, seen := range want {
		if !seen {
			t.Errorf("no transaction mapped to %s", typ)
		}
	}
}
