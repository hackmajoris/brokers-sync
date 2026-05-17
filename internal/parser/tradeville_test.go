package parser

import (
	"strings"
	"testing"
	"time"

	"brokers-sync/internal/model"
)

// csvWithHeader wraps rows in a valid Tradeville CSV (SEP hint + header + data).
func csvWithHeader(rows ...string) string {
	header := "SEP=\t\nTip\tData\tSimbol\tSuma\tCantitate\tPret\n"
	return header + strings.Join(rows, "\n")
}

const (
	buyRow       = "Cumparare\t10/07/2026\tTVBETETF\t-1,176.54 RON\t20\t58.52"
	sellRow      = "Vanzare\t11/11/2025\tSNP\t2,897.42 RON\t3055\t0.96"
	depositRow   = "Alimentare\t10/07/2026\tRON\t1,000 RON\t1,000\t-"
	withdrawRow  = "Retragere\t01/06/2026\tEUR\t-1,472.02 EUR\t1,472.02\t-"
	dividendRow  = "Dividend\t30/06/2026\tTLV\t194.34 RON\t196.3\t-"
	feeRow       = "Comision\t13/11/2025\tEUR\t-0.98 EUR\t0\t-"
	fxRow        = "Schimb valutar\t01/06/2026\tRON\t-7,499.98 RON\t7,499.98\t-"
	transferRow  = "Transferuri interne\t01/06/2026\tEUR\t1,473.56 EUR\t1,473.56\t-"
	freeShareRow = "Transferuri interne\t21/07/2025\tTLV\t0 RON\t23\t0"
)

func TestParseTradevilleRows(t *testing.T) {
	tests := []struct {
		name string
		row  string
		want model.Transaction
	}{
		{
			name: "BUY trade row",
			row:  buyRow,
			want: model.Transaction{
				Type:     model.TxBuy,
				Symbol:   "TVBETETF",
				Currency: "RON",
				Quantity: 20,
				Price:    58.52,
				Net:      -1176.54,
				Broker:   "tradeville",
			},
		},
		{
			name: "SELL trade row",
			row:  sellRow,
			want: model.Transaction{
				Type:     model.TxSell,
				Symbol:   "SNP",
				Currency: "RON",
				Quantity: 3055,
				Price:    0.96,
				Net:      2897.42,
				Broker:   "tradeville",
			},
		},
		{
			// Deposit currency comes from the Suma suffix, not the Simbol column;
			// no ticker symbol is attached to cash rows.
			name: "DEPOSIT (Alimentare RON)",
			row:  depositRow,
			want: model.Transaction{
				Type:     model.TxDeposit,
				Symbol:   "",
				Currency: "RON",
				Net:      1000,
				Broker:   "tradeville",
			},
		},
		{
			// Withdrawals keep Suma's negative sign so cash-flow stats subtract them.
			name: "WITHDRAWAL (Retragere EUR)",
			row:  withdrawRow,
			want: model.Transaction{
				Type:     model.TxWithdrawal,
				Symbol:   "",
				Currency: "EUR",
				Net:      -1472.02,
				Broker:   "tradeville",
			},
		},
		{
			// Suma is the net dividend actually received; this export carries no tax line.
			name: "DIVIDEND with ticker from Simbol",
			row:  dividendRow,
			want: model.Transaction{
				Type:     model.TxDividend,
				Symbol:   "TLV",
				Currency: "RON",
				Net:      194.34,
				Broker:   "tradeville",
			},
		},
		{
			name: "FEE (Comision)",
			row:  feeRow,
			want: model.Transaction{
				Type:     model.TxFee,
				Currency: "EUR",
				Net:      -0.98,
				Broker:   "tradeville",
			},
		},
		{
			// FX conversion is internal plumbing → TxForex, ignored by ledger/cash-flow.
			name: "FX conversion (Schimb valutar)",
			row:  fxRow,
			want: model.Transaction{
				Type:     model.TxForex,
				Symbol:   "",
				Currency: "RON",
				Net:      -7499.98,
				Broker:   "tradeville",
			},
		},
		{
			// Internal transfer is not external capital → TxForex, so it never counts
			// as a deposit (which would inflate invested capital and distort gain%).
			name: "Internal transfer (Transferuri interne)",
			row:  transferRow,
			want: model.Transaction{
				Type:     model.TxForex,
				Symbol:   "",
				Currency: "EUR",
				Net:      1473.56,
				Broker:   "tradeville",
			},
		},
		{
			// Same "Transferuri interne" label but Simbol is a ticker with zero Suma:
			// a free-share distribution, recorded as a zero-cost BUY so the shares
			// count toward quantity without adding cost basis.
			name: "Free shares (Transferuri interne + ticker)",
			row:  freeShareRow,
			want: model.Transaction{
				Type:     model.TxBuy,
				Symbol:   "TLV",
				Currency: "RON",
				Quantity: 23,
				Price:    0,
				Net:      0,
				Broker:   "tradeville",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txs, err := ParseTradeville(strings.NewReader(csvWithHeader(tc.row)))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if len(txs) != 1 {
				t.Fatalf("expected 1 transaction, got %d", len(txs))
			}
			got := txs[0]

			if got.Type != tc.want.Type {
				t.Errorf("Type: got %q, want %q", got.Type, tc.want.Type)
			}
			if got.Symbol != tc.want.Symbol {
				t.Errorf("Symbol: got %q, want %q", got.Symbol, tc.want.Symbol)
			}
			if got.Currency != tc.want.Currency {
				t.Errorf("Currency: got %q, want %q", got.Currency, tc.want.Currency)
			}
			if !approxEq(got.Quantity, tc.want.Quantity, 1e-6) {
				t.Errorf("Quantity: got %.6f, want %.6f", got.Quantity, tc.want.Quantity)
			}
			if !approxEq(got.Price, tc.want.Price, 1e-6) {
				t.Errorf("Price: got %.6f, want %.6f", got.Price, tc.want.Price)
			}
			if !approxEq(got.Net, tc.want.Net, 1e-4) {
				t.Errorf("Net: got %.6f, want %.6f", got.Net, tc.want.Net)
			}
			if got.Broker != tc.want.Broker {
				t.Errorf("Broker: got %q, want %q", got.Broker, tc.want.Broker)
			}
		})
	}
}

func TestParseTradevilleDate(t *testing.T) {
	// DD/MM/YYYY: day 10, month 07 — must not be read as month 10.
	txs, err := ParseTradeville(strings.NewReader(csvWithHeader(buyRow)))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	want := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	got := txs[0].Date
	if got.Year() != want.Year() || got.Month() != want.Month() || got.Day() != want.Day() {
		t.Errorf("Date: got %v, want %v", got, want)
	}
}

func approxEq(a, b, epsilon float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= epsilon
}
