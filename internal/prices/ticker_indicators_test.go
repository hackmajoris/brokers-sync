package prices

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

// fakeYahoo answers the v7 quote with the given quote type and records every
// quoteSummary module set requested, so a test can see which upstream calls a
// fetch actually paid for. Every other endpoint 404s.
func fakeYahoo(t *testing.T, quoteType string) (*yahoo.Client, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var modules []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v7/finance/quote":
			fmt.Fprintf(w, `{"quoteResponse":{"result":[{"symbol":%q,"regularMarketPrice":100,"currency":"EUR","quoteType":%q}]}}`, r.URL.Query().Get("symbols"), quoteType)
		case strings.HasPrefix(r.URL.Path, "/v10/finance/quoteSummary/"):
			m := r.URL.Query().Get("modules")
			mu.Lock()
			modules = append(modules, m)
			mu.Unlock()
			switch {
			case strings.Contains(m, "fundProfile"):
				fmt.Fprint(w, `{"quoteSummary":{"result":[{"fundProfile":{"feesExpensesInvestment":{"annualReportExpenseRatio":{"raw":0.0007}}}}]}}`)
			case strings.Contains(m, "topHoldings"):
				fmt.Fprint(w, `{"quoteSummary":{"result":[{"topHoldings":{"holdings":[{"symbol":"NVDA","holdingName":"NVIDIA","holdingPercent":{"raw":0.08}}]}}]}}`)
			default:
				fmt.Fprint(w, `{"quoteSummary":{"result":[{}]}}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := yahoo.New(yahoo.WithBaseURL(srv.URL), yahoo.WithV8BaseURL(srv.URL), yahoo.WithCrumb("test"))
	if err != nil {
		t.Fatal(err)
	}
	return client, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), modules...)
	}
}

func requested(modules []string, name string) bool {
	for _, m := range modules {
		if strings.Contains(m, name) {
			return true
		}
	}
	return false
}

// A watchlist is mostly stocks. Fund data is an extra request per symbol, so a
// stock must never pay for it — otherwise every list load grows by a call per
// row for data that is always empty.
func TestStockSkipsFundRequests(t *testing.T) {
	client, modules := fakeYahoo(t, "EQUITY")

	ti, ok := fetchIndicators(context.Background(), client, "TSTSTOCK", true)
	if !ok {
		t.Fatal("expected indicators to resolve")
	}
	if ti.Currency != "EUR" {
		t.Errorf("currency must come from the quote so prices are not shown as USD, got %q", ti.Currency)
	}
	if ti.TER != nil || len(ti.TopHoldings) > 0 {
		t.Errorf("a stock has no TER or holdings, got TER=%v holdings=%v", ti.TER, ti.TopHoldings)
	}
	if got := modules(); requested(got, "fundProfile") || requested(got, "topHoldings") {
		t.Errorf("stock fetch requested fund modules: %v", got)
	}
}

// The table shows TER, so a list fetch needs it; holdings only appear in the
// lookup modal, so a list fetch must not spend a request per ETF on them.
func TestETFListFetchHasTERButNoHoldings(t *testing.T) {
	client, modules := fakeYahoo(t, "ETF")

	ti, ok := fetchIndicators(context.Background(), client, "TSTETFLIST", false)
	if !ok {
		t.Fatal("expected indicators to resolve")
	}
	if ti.TER == nil || *ti.TER < 0.069 || *ti.TER > 0.071 {
		t.Errorf("want TER 0.07%% (raw 0.0007 as a percentage), got %v", ti.TER)
	}
	if requested(modules(), "topHoldings") {
		t.Errorf("list fetch requested holdings: %v", modules())
	}
}

func TestETFFullFetchHasHoldings(t *testing.T) {
	client, _ := fakeYahoo(t, "ETF")

	ti, ok := fetchIndicators(context.Background(), client, "TSTETFFULL", true)
	if !ok {
		t.Fatal("expected indicators to resolve")
	}
	if len(ti.TopHoldings) != 1 || ti.TopHoldings[0].Symbol != "NVDA" {
		t.Fatalf("want NVDA holding for the modal, got %v", ti.TopHoldings)
	}
}
