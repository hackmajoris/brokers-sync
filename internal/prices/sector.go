package prices

import (
	"context"
	"sync"
	"time"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

// SectorValuation is a sector's aggregate valuation: the market-cap-weighted
// P/E and EV/EBITDA across the sector's largest companies, as computed by
// go-finance's sector benchmark.
//
// It is deliberately keyed by sector rather than by symbol. go-finance's
// GetSectorBenchmark answers for one ticker and re-aggregates the whole peer
// set every time it is called — roughly a dozen upstream requests. The
// aggregate itself depends only on the sector, so fetching it once per distinct
// sector and comparing each symbol's own P/E against it gives the same answer
// for a fraction of the traffic: a thirty-symbol watchlist spanning five
// sectors costs five aggregates, not thirty.
type SectorValuation struct {
	Sector     string
	PE         float64
	EVToEBITDA float64
	PeerCount  int
}

// sectorTTL is far longer than indicatorTTL. A sector aggregate is built from
// the largest companies in a sector and barely moves within a day, unlike the
// price-derived figures it is compared against.
const sectorTTL = 12 * time.Hour

// minSectorPeers is the smallest peer count worth publishing a comparison
// against. Below it the "aggregate" is a handful of companies, and a percentage
// against it reads as authoritative while meaning very little.
const minSectorPeers = 4

var (
	sectorCache sync.Map // sector name -> sectorEntry
	// One in-flight fetch per sector. Without this, a cold list of ten
	// technology symbols would start ten identical aggregate fetches at once,
	// each costing a dozen upstream calls.
	sectorFetchMu sync.Map // sector name -> *sync.Mutex
)

type sectorEntry struct {
	val     *SectorValuation
	expires time.Time
}

func cachedSector(sector string) (*SectorValuation, bool) {
	v, ok := sectorCache.Load(sector)
	if !ok {
		return nil, false
	}
	e := v.(sectorEntry)
	if time.Now().After(e.expires) {
		sectorCache.Delete(sector)
		return nil, false
	}
	return e.val, true
}

func storeSector(sector string, val *SectorValuation) {
	sectorCache.Store(sector, sectorEntry{val: val, expires: time.Now().Add(sectorTTL)})
}

// symbolSectorCache maps symbol -> sector. A company's sector is static, so it
// is cached far longer than the price-derived indicators it travels with;
// without this every five-minute list refresh would re-ask Yahoo which sector
// each symbol is in.
var symbolSectorCache sync.Map // symbol -> sectorNameEntry

type sectorNameEntry struct {
	sector  string
	expires time.Time
}

// SectorOf returns the Yahoo sector classification for a symbol. The name is
// taken verbatim from Yahoo's own taxonomy, which is not GICS.
func SectorOf(ctx context.Context, client *yahoo.Client, symbol string) (string, bool) {
	if v, ok := symbolSectorCache.Load(symbol); ok {
		e := v.(sectorNameEntry)
		if time.Now().Before(e.expires) {
			return e.sector, e.sector != ""
		}
		symbolSectorCache.Delete(symbol)
	}
	s, err := client.GetSector(ctx, symbol)
	if err != nil {
		return "", false
	}
	symbolSectorCache.Store(symbol, sectorNameEntry{sector: s.Sector, expires: time.Now().Add(sectorTTL)})
	if s.Sector == "" {
		return "", false
	}
	return s.Sector, true
}

// SectorValuationFor returns the aggregate valuation for a sector, fetched via
// one representative symbol from that sector and cached for every other symbol
// that shares it. Returns nil when the sector is unknown, the aggregate cannot
// be built, or too few peers resolved to make the comparison meaningful.
func SectorValuationFor(ctx context.Context, client *yahoo.Client, sector, representative string) *SectorValuation {
	if sector == "" {
		return nil
	}
	if v, ok := cachedSector(sector); ok {
		return v
	}

	muAny, _ := sectorFetchMu.LoadOrStore(sector, &sync.Mutex{})
	mu := muAny.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Another goroutine may have filled it while this one waited.
	if v, ok := cachedSector(sector); ok {
		return v
	}

	b, err := client.GetSectorBenchmark(ctx, representative)
	if err != nil || b.PeerCount < minSectorPeers {
		// Cache the negative too: a sector Yahoo has no peer data for would
		// otherwise be retried by every symbol in it, on every request.
		storeSector(sector, nil)
		return nil
	}
	val := &SectorValuation{
		Sector:     b.Sector,
		PE:         b.SectorPE,
		EVToEBITDA: b.SectorEVToEBITDA,
		PeerCount:  b.PeerCount,
	}
	storeSector(sector, val)
	return val
}

// vsSector expresses a company figure as a percentage above or below its sector
// aggregate. Negative is cheaper than the sector, positive is more expensive.
// Returns nil when either side is missing or the sector figure is zero.
func vsSector(own *float64, sectorValue float64) *float64 {
	if own == nil || *own <= 0 || sectorValue <= 0 {
		return nil
	}
	pct := (*own - sectorValue) / sectorValue * 100
	return &pct
}
