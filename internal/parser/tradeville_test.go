package parser

import (
	"strings"
	"testing"
	"time"

	"brokers-sync/internal/model"
)

// csvWithHeader wraps rows in a valid Tradeville CSV (SEP hint + header + data).
func csvWithHeader(rows ...string) string {
	header := "SEP=\t\nid\tdata\top\tdescr\tsimbol\tcant\tpret\tcomis\tsuma\tsoldlei\tsoldact\tprofit\tnrtranz\tvaluta\tobs\tcostm\telei\ttxcnlei\tidord\tcont\tmarket\tytm\timpozlei\tsimbolb\tdirty\tpiata\tcant1an\tcost\tnrzecDynamic\tnrzecFractionar\tdataConf\n"
	return header + strings.Join(rows, "\n")
}

// trade row (24 fields) - cump or vanz
const buyRow = "23432347\t2026-04-24 11:18:08.637\tcump\tSNP\tSNP\t970\t1.02\t5.78214\t-995.77578\t14.256\t20818\t\t29812317\tRON\t\t0.87\t0\t0.59\tA17038616\tE3FD39\tREGS\t\t\tSNP\t\t\t\t1.02\t[object Object]\t<5\t<img>"
const sellRow = "20835631\t2025-11-11 13:39:14.297\tvanz\tSNP\tSNP\t3055\t0.957\t12.57163\t2897.41667\t7567.26\t13643\t454.89\t28333959\tRON\t\t0.80\t0\t0\tA15662135\tE3FD39\tREGS\t\t13.65\tSNP\t\t\t\t0.94\t[object Object]\t<5\t<img>"

// non-trade rows (18 fields — pret column absent, cols shift left)
const depositRow = "23431470\t2026-04-24 10:56:27.033\tin\tDEPUNERE CONT CLIENT\tRON\t1000\t0\t1000\t1010.03268\t\t\t\t\tRON\tDEPUNERE CONT CLIENT\t\t1\t\tA2650519\tE3FD39\t\t\t\t\t\t\t\t\t[object Object]\t2\t<img>"
const dividendRow = "21282648\t2025-12-11 12:05:06.730\tdiv\tDIVIDEND TLV\tRON\t105.16\t1.0516\t104.1084\t117.39\t\t\tTLV\tRON\tdividend TLV\t\t1\t\t\tE3FD39\t\t\t\tTLV\t\t\t\t\t[object Object]\t2\t<img>"
const feeRow = "20861264\t2025-11-13 08:44:59.903\tcomis\tREFACTURARE CMS\tEUR\t0\t0.98\t-0.98\t1472.73\t\t\t\t\tEUR\tcomis\t\t1\t\tA2229193\tE3FD39-RE\t\t\t\t\t\t\t\t\t[object Object]\t2\t<img>"
const freeSharesRow = "19355597\t2025-07-21 09:04:16.717\tin\tTLV - MCS GRATUITE\tTLV\t23\t0\t0\t21.82\t\t\t\t7/21/2025\tRON\tmcs gratuite\t25.03\t0\t0\t\tE3FD39\t\t\t\tTLV\t\t\t\t0\t[object Object]\t<5\t<img>"

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
				Type:       model.TxBuy,
				Symbol:     "SNP",
				Currency:   "RON",
				Quantity:   970,
				Price:      1.02,
				Commission: -5.78214,
				Net:        -995.77578,
				Broker:     "tradeville",
			},
		},
		{
			name: "SELL trade row",
			row:  sellRow,
			want: model.Transaction{
				Type:       model.TxSell,
				Symbol:     "SNP",
				Currency:   "RON",
				Quantity:   3055,
				Price:      0.957,
				Commission: -12.57163,
				Net:        2897.41667,
				Broker:     "tradeville",
			},
		},
		{
			name: "DEPOSIT (in RON)",
			row:  depositRow,
			want: model.Transaction{
				Type:     model.TxDeposit,
				Symbol:   "",
				Currency: "RON",
				Quantity: 1000,
				Net:      1000,
				Broker:   "tradeville",
			},
		},
		{
			name: "DIVIDEND with TLV symbol extracted from descr",
			row:  dividendRow,
			want: model.Transaction{
				Type:       model.TxDividend,
				Symbol:     "TLV",
				Currency:   "RON",
				Quantity:   105.16,
				Net:        104.1084,
				Commission: -1.0516,
				Broker:     "tradeville",
			},
		},
		{
			name: "FEE (comis op)",
			row:  feeRow,
			want: model.Transaction{
				Type:       model.TxFee,
				Currency:   "EUR",
				Net:        -0.98,
				Commission: -0.98,
				Broker:     "tradeville",
			},
		},
		{
			name: "Free shares (in + stock symbol = zero-cost BUY)",
			row:  freeSharesRow,
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
			if !approxEq(got.Net, tc.want.Net, 1e-4) {
				t.Errorf("Net: got %.6f, want %.6f", got.Net, tc.want.Net)
			}
			if !approxEq(got.Commission, tc.want.Commission, 1e-4) {
				t.Errorf("Commission: got %.6f, want %.6f", got.Commission, tc.want.Commission)
			}
			if got.Broker != tc.want.Broker {
				t.Errorf("Broker: got %q, want %q", got.Broker, tc.want.Broker)
			}
		})
	}
}

func TestParseTradevilleDate(t *testing.T) {
	tests := []struct {
		name    string
		row     string
		wantDay time.Time
	}{
		{
			name:    "millisecond precision date",
			row:     buyRow,
			wantDay: time.Date(2026, 4, 24, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			txs, err := ParseTradeville(strings.NewReader(csvWithHeader(tc.row)))
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			got := txs[0].Date
			if got.Year() != tc.wantDay.Year() || got.Month() != tc.wantDay.Month() || got.Day() != tc.wantDay.Day() {
				t.Errorf("Date: got %v, want %v", got, tc.wantDay)
			}
		})
	}
}

func approxEq(a, b, epsilon float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= epsilon
}
