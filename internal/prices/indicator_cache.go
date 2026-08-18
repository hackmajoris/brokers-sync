package prices

import (
	"context"
	"sync"
	"time"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

// indicatorTTL is how long a fetched set of indicators stays usable. Yahoo's
// figures move slowly compared to how often a watchlist is reloaded, so five
// minutes turns a repeat load into zero upstream calls while staying fresh
// enough for a price column.
const indicatorTTL = 5 * time.Minute

// The cache lives in the Lambda process, so it survives only as long as the
// execution environment. That is enough for the case it targets — the same
// symbols requested again minutes later — and it needs no infrastructure.
// Entries are treated as immutable once stored; callers only read them.
var indicatorCache sync.Map // cacheKey -> cacheEntry

type cacheKey struct {
	symbol string
	full   bool
}

type cacheEntry struct {
	ti      *TickerIndicators
	expires time.Time
}

func cachedIndicators(symbol string, full bool) (*TickerIndicators, bool) {
	v, ok := indicatorCache.Load(cacheKey{symbol: symbol, full: full})
	if !ok {
		return nil, false
	}
	e := v.(cacheEntry)
	if time.Now().After(e.expires) {
		indicatorCache.Delete(cacheKey{symbol: symbol, full: full})
		return nil, false
	}
	return e.ti, true
}

func storeIndicators(symbol string, full bool, ti *TickerIndicators) {
	indicatorCache.Store(cacheKey{symbol: symbol, full: full}, cacheEntry{ti: ti, expires: time.Now().Add(indicatorTTL)})
}

// FetchListIndicators fetches the table-rendered indicators for symbols, served
// from cache where possible. One shared Yahoo client covers the whole batch, so
// the auth crumb and its cookie handshake are paid once instead of per symbol,
// and connections are reused. concurrency bounds parallel symbol fetches.
//
// Symbols with no upstream data are absent from the result rather than being an
// error: one delisted ticker must not fail the whole list.
func FetchListIndicators(ctx context.Context, symbols []string, concurrency int) map[string]*TickerIndicators {
	out := make(map[string]*TickerIndicators, len(symbols))
	var misses []string
	for _, sym := range symbols {
		if ti, ok := cachedIndicators(sym, false); ok {
			out[sym] = ti
			continue
		}
		misses = append(misses, sym)
	}
	if len(misses) == 0 {
		return out
	}

	client, err := yahoo.New()
	if err != nil {
		return out
	}

	// symbols[0] runs alone to populate the client's crumb before the rest fan
	// out. go-finance fetches the crumb lazily with no internal locking, so a
	// cold parallel start makes every goroutine race to fetch it and Yahoo
	// answers the pile-up with 401/429. See fetchCrumbGuarded.
	first, rest := misses[0], misses[1:]
	if ti, ok := fetchIndicators(ctx, client, first, false); ok {
		storeIndicators(first, false, ti)
		out[first] = ti
	}

	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, sym := range rest {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ti, ok := fetchIndicators(ctx, client, sym, false)
			if !ok {
				return
			}
			storeIndicators(sym, false, ti)
			mu.Lock()
			out[sym] = ti
			mu.Unlock()
		}(sym)
	}
	wg.Wait()
	return out
}
