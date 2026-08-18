package prices

import (
	"context"
	"testing"
	"time"
)

func TestFetchListIndicatorsServesCacheWithoutUpstream(t *testing.T) {
	indicatorCache.Clear()
	price := 123.45
	storeIndicators("CACHED", false, &TickerIndicators{Price: &price})

	// A cancelled context makes any upstream call fail, so a result here can
	// only have come from the cache. This is the whole point of the cache: a
	// reload minutes later must cost zero Yahoo requests.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := FetchListIndicators(ctx, []string{"CACHED"}, 4)
	ti, ok := got["CACHED"]
	if !ok {
		t.Fatal("cached symbol was not returned; the fetch went upstream instead of using the cache")
	}
	if ti.Price == nil || *ti.Price != price {
		t.Fatalf("cached indicators not returned intact: got %+v", ti)
	}
}

func TestCachedIndicatorsExpire(t *testing.T) {
	indicatorCache.Clear()
	price := 10.0
	// Stale entries must not be served: a price column that never refreshes is
	// worse than a slow one.
	indicatorCache.Store(cacheKey{symbol: "STALE", full: false}, cacheEntry{
		ti:      &TickerIndicators{Price: &price},
		expires: time.Now().Add(-time.Second),
	})

	if _, ok := cachedIndicators("STALE", false); ok {
		t.Fatal("expired entry was served from cache")
	}
}

func TestCacheScopesAreSeparate(t *testing.T) {
	indicatorCache.Clear()
	price := 50.0
	storeIndicators("SCOPED", false, &TickerIndicators{Price: &price})

	// The list scope omits the modal-only indicators, so serving it to the
	// modal would silently blank out half that panel.
	if _, ok := cachedIndicators("SCOPED", true); ok {
		t.Fatal("list-scope entry was served to a full-scope lookup")
	}
}
