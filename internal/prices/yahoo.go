package prices

import (
	"context"
	"sync"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

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
	client, err := yahoo.New()
	if err != nil {
		return nil, err
	}

	type result struct {
		sym string
		pe  PERatio
		ok  bool
	}

	ch := make(chan result, len(symbols))
	var wg sync.WaitGroup

	for _, sym := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			pe, err := client.GetPE(ctx, sym)
			if err != nil {
				ch <- result{sym: sym}
				return
			}
			ch <- result{sym: sym, pe: PERatio{PE: pe.PE, ForwardPE: pe.ForwardPE}, ok: true}
		}(sym)
	}

	wg.Wait()
	close(ch)

	out := make(map[string]PERatio, len(symbols))
	for r := range ch {
		if r.ok {
			out[r.sym] = r.pe
		}
	}
	return out, nil
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
