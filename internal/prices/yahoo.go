package prices

import (
	"context"
	"sync"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

// fetchCrumbGuarded runs fetch for symbols[0] synchronously to force the shared
// client's Yahoo auth "crumb" to populate, then fans the remaining symbols out
// in parallel. The go-finance client fetches its crumb lazily on first use with
// no internal locking, so calling it from many goroutines at once makes them
// all race to populate c.crumb simultaneously — Yahoo answers that pile-up with
// 401/429 for the whole batch. Serializing just the first call avoids the race
// entirely since every later call sees a already-populated crumb.
func fetchCrumbGuarded[T any](ctx context.Context, symbols []string, fetch func(context.Context, *yahoo.Client, string) (T, bool)) (map[string]T, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}

	out := make(map[string]T, len(symbols))
	if len(symbols) == 0 {
		return out, nil
	}

	first, rest := symbols[0], symbols[1:]
	if v, ok := fetch(ctx, client, first); ok {
		out[first] = v
	}

	type result struct {
		sym string
		val T
		ok  bool
	}
	ch := make(chan result, len(rest))
	var wg sync.WaitGroup
	for _, sym := range rest {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			v, ok := fetch(ctx, client, sym)
			ch <- result{sym: sym, val: v, ok: ok}
		}(sym)
	}
	wg.Wait()
	close(ch)

	for r := range ch {
		if r.ok {
			out[r.sym] = r.val
		}
	}
	return out, nil
}

// FetchQuotes fetches current prices for a list of ticker symbols in parallel.
// Returns a map of both original and normalized symbol → price.
func FetchQuotes(ctx context.Context, symbols []string) (map[string]float64, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}
	return client.FetchQuotes(ctx, symbols)
}

// FetchFXRates fetches FX spot rates relative to a base currency (e.g. "USD")
// in parallel. The base currency always gets rate 1.0.
func FetchFXRates(ctx context.Context, currencies []string, base string) (map[string]float64, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}
	return client.FetchFXRates(ctx, currencies, base)
}

// FiftyTwoWeekRange holds the 52-week trading range for a symbol.
type FiftyTwoWeekRange struct {
	Low  float64
	High float64
}

// FetchFiftyTwoWeekRanges fetches 52-week high/low ranges for a list of ticker
// symbols in parallel. Symbols that fail to resolve are omitted from the result.
func FetchFiftyTwoWeekRanges(ctx context.Context, symbols []string) (map[string]FiftyTwoWeekRange, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}

	type result struct {
		sym string
		rng FiftyTwoWeekRange
		ok  bool
	}

	ch := make(chan result, len(symbols))
	var wg sync.WaitGroup

	for _, sym := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			rng, err := client.FetchFiftyTwoWeekRange(ctx, sym)
			if err != nil {
				ch <- result{sym: sym}
				return
			}
			ch <- result{sym: sym, rng: FiftyTwoWeekRange{Low: rng.Low, High: rng.High}, ok: true}
		}(sym)
	}

	wg.Wait()
	close(ch)

	out := make(map[string]FiftyTwoWeekRange, len(symbols))
	for r := range ch {
		if r.ok {
			out[r.sym] = r.rng
		}
	}
	return out, nil
}

// PERatio holds the trailing and forward price/earnings ratios for a symbol.
type PERatio struct {
	PE        float64
	ForwardPE float64
}

// FetchPERatios fetches trailing and forward P/E ratios for a list of ticker
// symbols in parallel. Symbols that fail to resolve are omitted from the result.
func FetchPERatios(ctx context.Context, symbols []string) (map[string]PERatio, error) {
	return fetchCrumbGuarded(ctx, symbols, func(ctx context.Context, c *yahoo.Client, sym string) (PERatio, bool) {
		pe, err := c.GetPE(ctx, sym)
		if err != nil {
			return PERatio{}, false
		}
		return PERatio{PE: pe.PE, ForwardPE: pe.ForwardPE}, true
	})
}

// Performance holds trailing YTD, 3-year, and 5-year percentage price change for a symbol.
type Performance struct {
	YTD       float64
	ThreeYear float64
	FiveYear  float64
}

// FetchPerformances fetches YTD, 3-year, and 5-year performance for a list of ticker
// symbols in parallel. Symbols that fail to resolve are omitted from the result.
func FetchPerformances(ctx context.Context, symbols []string) (map[string]Performance, error) {
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}

	type result struct {
		sym  string
		perf Performance
		ok   bool
	}

	ch := make(chan result, len(symbols))
	var wg sync.WaitGroup

	for _, sym := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			perf, err := client.FetchPerformance(ctx, sym)
			if err != nil {
				ch <- result{sym: sym}
				return
			}
			ch <- result{sym: sym, perf: Performance{YTD: perf.YTD, ThreeYear: perf.ThreeYear, FiveYear: perf.FiveYear}, ok: true}
		}(sym)
	}

	wg.Wait()
	close(ch)

	out := make(map[string]Performance, len(symbols))
	for r := range ch {
		if r.ok {
			out[r.sym] = r.perf
		}
	}
	return out, nil
}

// FreeCashFlow holds the trailing twelve-month free cash flow for a symbol,
// in the symbol's reporting currency, plus a plain-language interpretation.
type FreeCashFlow struct {
	FCF            float64
	Interpretation string
}

// FetchFreeCashFlows fetches trailing twelve-month free cash flow for a list of
// ticker symbols in parallel. Symbols that fail to resolve are omitted from the result.
func FetchFreeCashFlows(ctx context.Context, symbols []string) (map[string]FreeCashFlow, error) {
	return fetchCrumbGuarded(ctx, symbols, func(ctx context.Context, c *yahoo.Client, sym string) (FreeCashFlow, bool) {
		fcf, err := c.GetFreeCashFlow(ctx, sym)
		if err != nil {
			return FreeCashFlow{}, false
		}
		return FreeCashFlow{FCF: fcf.FCF, Interpretation: fcf.Interpretation}, true
	})
}

// EVToEBITDA holds the enterprise-value-to-EBITDA ratio for a symbol,
// plus a plain-language interpretation.
type EVToEBITDA struct {
	Ratio          float64
	Interpretation string
}

// FetchEVToEBITDAs fetches EV/EBITDA ratios for a list of ticker symbols in
// parallel. Symbols that fail to resolve are omitted from the result.
func FetchEVToEBITDAs(ctx context.Context, symbols []string) (map[string]EVToEBITDA, error) {
	return fetchCrumbGuarded(ctx, symbols, func(ctx context.Context, c *yahoo.Client, sym string) (EVToEBITDA, bool) {
		ev, err := c.GetEVToEBITDA(ctx, sym)
		if err != nil {
			return EVToEBITDA{}, false
		}
		return EVToEBITDA{Ratio: ev.Ratio, Interpretation: ev.Interpretation}, true
	})
}

// DebtToEquity holds the debt-to-equity ratio for a symbol (Yahoo reports it as
// a percentage — 150.5 means total debt is 1.5x total equity), plus a
// plain-language interpretation.
type DebtToEquity struct {
	Ratio          float64
	Interpretation string
}

// FetchDebtToEquities fetches debt-to-equity ratios for a list of ticker symbols
// in parallel. Symbols that fail to resolve are omitted from the result.
func FetchDebtToEquities(ctx context.Context, symbols []string) (map[string]DebtToEquity, error) {
	return fetchCrumbGuarded(ctx, symbols, func(ctx context.Context, c *yahoo.Client, sym string) (DebtToEquity, bool) {
		de, err := c.GetDebtToEquity(ctx, sym)
		if err != nil {
			return DebtToEquity{}, false
		}
		return DebtToEquity{Ratio: de.Ratio, Interpretation: de.Interpretation}, true
	})
}

// CashFlowQuality holds trailing twelve-month operating cash flow vs net income
// (the "earnings quality" ratio) for a symbol, plus a plain-language interpretation.
type CashFlowQuality struct {
	Ratio          float64
	NetIncome      float64
	Interpretation string
}

// FetchCashFlowQualities fetches operating-cash-flow-vs-net-income ratios for a
// list of ticker symbols in parallel. Symbols that fail to resolve are omitted
// from the result.
func FetchCashFlowQualities(ctx context.Context, symbols []string) (map[string]CashFlowQuality, error) {
	return fetchCrumbGuarded(ctx, symbols, func(ctx context.Context, c *yahoo.Client, sym string) (CashFlowQuality, bool) {
		cfq, err := c.GetOperatingCashFlowVsNetIncome(ctx, sym)
		if err != nil {
			return CashFlowQuality{}, false
		}
		return CashFlowQuality{Ratio: cfq.Ratio, NetIncome: cfq.NetIncome, Interpretation: cfq.Interpretation}, true
	})
}

// ClassifyRatings runs go-finance's health and valuation classifiers over
// already-fetched FCF, cash-flow-quality, debt-to-equity, P/E, and EV/EBITDA
// results for a list of ticker symbols. Symbols missing all inputs are omitted.
func ClassifyRatings(
	symbols []string,
	fcf map[string]FreeCashFlow,
	cfq map[string]CashFlowQuality,
	d2e map[string]DebtToEquity,
	pe map[string]PERatio,
	ev map[string]EVToEBITDA,
) (health map[string]string, healthReason map[string]string, valuation map[string]string, valuationReason map[string]string) {
	health = make(map[string]string, len(symbols))
	healthReason = make(map[string]string, len(symbols))
	valuation = make(map[string]string, len(symbols))
	valuationReason = make(map[string]string, len(symbols))

	for _, sym := range symbols {
		var fcfPtr *yahoo.FreeCashFlow
		if v, ok := fcf[sym]; ok {
			fcfPtr = &yahoo.FreeCashFlow{FCF: v.FCF}
		}
		var cfqPtr *yahoo.CashFlowQuality
		if v, ok := cfq[sym]; ok {
			cfqPtr = &yahoo.CashFlowQuality{Ratio: v.Ratio, NetIncome: v.NetIncome}
		}
		var d2ePtr *yahoo.DebtToEquity
		if v, ok := d2e[sym]; ok {
			d2ePtr = &yahoo.DebtToEquity{Ratio: v.Ratio}
		}
		var pePtr *yahoo.PERatio
		if v, ok := pe[sym]; ok {
			pePtr = &yahoo.PERatio{PE: v.PE, ForwardPE: v.ForwardPE}
		}
		var evPtr *yahoo.EVToEBITDA
		if v, ok := ev[sym]; ok {
			evPtr = &yahoo.EVToEBITDA{Ratio: v.Ratio}
		}

		if fcfPtr == nil && cfqPtr == nil && d2ePtr == nil {
			// no health inputs for this symbol
		} else {
			rating, reason := yahoo.ClassifyHealth(fcfPtr, cfqPtr, d2ePtr)
			health[sym] = string(rating)
			healthReason[sym] = reason
		}

		if pePtr == nil && evPtr == nil {
			// no valuation inputs for this symbol
		} else {
			rating, reason := yahoo.ClassifyValuation(pePtr, evPtr)
			valuation[sym] = string(rating)
			valuationReason[sym] = reason
		}
	}
	return health, healthReason, valuation, valuationReason
}
