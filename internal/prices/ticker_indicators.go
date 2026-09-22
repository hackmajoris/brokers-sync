package prices

import (
	"context"
	"sync"
	"time"

	"github.com/hackmajoris/go-finance/pkg/yahoo"
)

// TickerIndicators holds every indicator shown in the stock-lookup modal for a
// single symbol. Value fields are pointers so "not available" (nil) is distinct
// from a real zero. Interpretation strings are go-finance's plain-language notes.
type TickerIndicators struct {
	Price      *float64
	Week52Low  *float64
	Week52High *float64

	PE        *float64
	ForwardPE *float64

	Today     *float64
	OneWeek   *float64
	OneMonth  *float64
	YTD       *float64
	ThreeYear *float64
	FiveYear  *float64
	TenYear   *float64

	FCF       *float64
	FCFInterp string

	EVToEBITDA *float64
	EVInterp   string

	DebtToEquity *float64
	DebtEqInterp string

	CashFlowQuality *float64
	CFQInterp       string

	MarketCap       *float64
	MarketCapInterp string

	PriceToSales       *float64
	PriceToSalesInterp string

	PriceToBook       *float64
	PriceToBookInterp string

	FCFYield       *float64
	FCFYieldInterp string

	ProfitMargin       *float64
	ProfitMarginInterp string

	OperatingMargin       *float64
	OperatingMarginInterp string

	QuarterlyEarningsGrowth       *float64
	QuarterlyEarningsGrowthInterp string

	QuarterlyRevenueGrowth       *float64
	QuarterlyRevenueGrowthInterp string

	Cash       *float64
	CashInterp string

	Debt       *float64
	DebtInterp string

	Net *float64

	DividendYield       *float64
	DividendYieldInterp string

	PayoutRatio       *float64
	PayoutRatioInterp string

	PayoutDate       *time.Time
	PayoutDateInterp string

	AnalystScore  *float64
	AnalystRating string

	Sector           string
	SectorPE         *float64
	PEVsSector       *float64
	SectorEVToEBITDA *float64
	EVVsSector       *float64
	SectorPeerCount  int

	HealthRating    string
	HealthReason    string
	ValuationRating string
	ValuationReason string

	cfqNetIncome float64 // retained for the health classifier, not serialized
}

// FetchTickerIndicators fetches all modal indicators for a single symbol using
// one shared Yahoo client. The client's auth crumb is primed by a single
// synchronous call before the remaining indicators fan out concurrently — the
// go-finance client populates its crumb lazily with no internal locking, so
// firing every call cold at once would make them race and draw 401/429. Priming
// once means all fan-out calls see a populated crumb and share it, so the whole
// modal costs one crumb fetch instead of one per indicator. Returns ok=false
// when nothing at all resolved (unknown or invalid symbol).
func FetchTickerIndicators(ctx context.Context, symbol string) (*TickerIndicators, bool) {
	if ti, ok := cachedIndicators(symbol, true); ok {
		return ti, true
	}
	client, err := yahoo.New()
	if err != nil {
		return nil, false
	}
	ti, ok := fetchIndicators(ctx, client, symbol, true)
	if ok {
		storeIndicators(symbol, true, ti)
	}
	return ti, ok
}

// fetchIndicators does the work for both scopes against a caller-supplied
// client. full=false sets only the indicators the positions and watchlist
// tables render, leaving the dozen that feed the stock-lookup modal alone unset
// — the modal refetches them itself. All quoteSummary indicators come from one
// GetFundamentals request either way.
func fetchIndicators(ctx context.Context, client *yahoo.Client, symbol string, full bool) (*TickerIndicators, bool) {
	ti := &TickerIndicators{}
	var resolved bool

	// Prime the crumb synchronously with one quoteSummary-backed call.
	if pe, err := client.GetPE(ctx, symbol); err == nil {
		ti.PE = &pe.PE
		ti.ForwardPE = &pe.ForwardPE
		resolved = true
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // guards `resolved` only; each field is written by one goroutine
	markResolved := func() { mu.Lock(); resolved = true; mu.Unlock() }
	run := func(f func() bool) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if f() {
				markResolved()
			}
		}()
	}

	run(func() bool {
		// Rides the same v7 quote endpoint GetPE uses, so it is close to free.
		// An empty rating means no analyst covers the symbol — common for small
		// caps and foreign listings, and not an error.
		v, err := client.GetAnalystRating(ctx, symbol)
		if err != nil || v.Rating == "" {
			return false
		}
		ti.AnalystRating = v.Rating
		if v.Score > 0 {
			ti.AnalystScore = &v.Score
		}
		return true
	})
	run(func() bool {
		sector, ok := SectorOf(ctx, client, symbol)
		if !ok {
			return false
		}
		ti.Sector = sector
		return true
	})
	run(func() bool {
		q, err := client.GetQuote(ctx, symbol)
		if err != nil {
			return false
		}
		ti.Price = &q.Price
		return true
	})
	run(func() bool {
		r, err := client.FetchFiftyTwoWeekRange(ctx, symbol)
		if err != nil {
			return false
		}
		ti.Week52Low = &r.Low
		ti.Week52High = &r.High
		return true
	})
	run(func() bool {
		p, err := client.FetchPerformance(ctx, symbol)
		if err != nil {
			return false
		}
		ti.Today = &p.Today
		ti.OneWeek = &p.OneWeek
		ti.OneMonth = &p.OneMonth
		ti.YTD = &p.YTD
		ti.ThreeYear = &p.ThreeYear
		ti.FiveYear = &p.FiveYear
		ti.TenYear = &p.TenYear
		return true
	})
	run(func() bool {
		f, err := client.GetFundamentals(ctx, symbol)
		if err != nil {
			return false
		}
		ti.FCF = &f.FreeCashFlow.FCF
		ti.FCFInterp = f.FreeCashFlow.Interpretation
		ti.EVToEBITDA = &f.EVToEBITDA.Ratio
		ti.EVInterp = f.EVToEBITDA.Interpretation
		ti.DebtToEquity = &f.DebtToEquity.Ratio
		ti.DebtEqInterp = f.DebtToEquity.Interpretation
		ti.CashFlowQuality = &f.CashFlowQuality.Ratio
		ti.cfqNetIncome = f.CashFlowQuality.NetIncome
		ti.CFQInterp = f.CashFlowQuality.Interpretation
		// Modal-only indicators. The tables never render these, so a list fetch
		// leaves them unset even though the same request returned them.
		if full {
			ti.MarketCap = &f.MarketCap.MarketCap
			ti.MarketCapInterp = f.MarketCap.Interpretation
			ti.PriceToSales = &f.PriceToSales.Ratio
			ti.PriceToSalesInterp = f.PriceToSales.Interpretation
			ti.PriceToBook = &f.PriceToBook.Ratio
			ti.PriceToBookInterp = f.PriceToBook.Interpretation
			ti.FCFYield = &f.FreeCashFlowYield.Yield
			ti.FCFYieldInterp = f.FreeCashFlowYield.Interpretation
			ti.ProfitMargin = &f.ProfitMargin.Margin
			ti.ProfitMarginInterp = f.ProfitMargin.Interpretation
			ti.OperatingMargin = &f.OperatingMargin.Margin
			ti.OperatingMarginInterp = f.OperatingMargin.Interpretation
			ti.QuarterlyEarningsGrowth = &f.QuarterlyEarningsGrowth.Growth
			ti.QuarterlyEarningsGrowthInterp = f.QuarterlyEarningsGrowth.Interpretation
			ti.QuarterlyRevenueGrowth = &f.QuarterlyRevenueGrowth.Growth
			ti.QuarterlyRevenueGrowthInterp = f.QuarterlyRevenueGrowth.Interpretation
			ti.Cash = &f.Cash.Cash
			ti.CashInterp = f.Cash.Interpretation
			ti.Debt = &f.Debt.Debt
			ti.DebtInterp = f.Debt.Interpretation
			ti.DividendYield = &f.DividendYield.Yield
			ti.DividendYieldInterp = f.DividendYield.Interpretation
			ti.PayoutRatio = &f.PayoutRatio.Ratio
			ti.PayoutRatioInterp = f.PayoutRatio.Interpretation
			ti.PayoutDate = &f.PayoutDate.Date
			ti.PayoutDateInterp = f.PayoutDate.Interpretation
		}
		return true
	})

	wg.Wait()

	if !resolved {
		return nil, false
	}

	// Net cash = total cash − total debt, when both are known.
	if ti.Cash != nil && ti.Debt != nil {
		n := *ti.Cash - *ti.Debt
		ti.Net = &n
	}

	// Sector aggregate last: it needs this symbol's own P/E and EV/EBITDA to
	// compare against, and it is shared by every symbol in the same sector, so
	// it is fetched once and cached rather than per row.
	if sv := SectorValuationFor(ctx, client, ti.Sector, symbol); sv != nil {
		ti.SectorPeerCount = sv.PeerCount
		if sv.PE > 0 {
			pe := sv.PE
			ti.SectorPE = &pe
			ti.PEVsSector = vsSector(ti.PE, sv.PE)
		}
		if sv.EVToEBITDA > 0 {
			ev := sv.EVToEBITDA
			ti.SectorEVToEBITDA = &ev
			ti.EVVsSector = vsSector(ti.EVToEBITDA, sv.EVToEBITDA)
		}
	}

	ti.classify()
	return ti, true
}

// classify runs go-finance's health and valuation classifiers over the already
// fetched inputs and stores the ratings on the receiver.
func (ti *TickerIndicators) classify() {
	var fcfPtr *yahoo.FreeCashFlow
	if ti.FCF != nil {
		fcfPtr = &yahoo.FreeCashFlow{FCF: *ti.FCF}
	}
	var cfqPtr *yahoo.CashFlowQuality
	if ti.CashFlowQuality != nil {
		cfqPtr = &yahoo.CashFlowQuality{Ratio: *ti.CashFlowQuality, NetIncome: ti.cfqNetIncome}
	}
	var d2ePtr *yahoo.DebtToEquity
	if ti.DebtToEquity != nil {
		d2ePtr = &yahoo.DebtToEquity{Ratio: *ti.DebtToEquity}
	}
	var pePtr *yahoo.PERatio
	if ti.PE != nil {
		pePtr = &yahoo.PERatio{PE: *ti.PE, ForwardPE: derefOr(ti.ForwardPE)}
	}
	var evPtr *yahoo.EVToEBITDA
	if ti.EVToEBITDA != nil {
		evPtr = &yahoo.EVToEBITDA{Ratio: *ti.EVToEBITDA}
	}

	if fcfPtr != nil || cfqPtr != nil || d2ePtr != nil {
		rating, reason := yahoo.ClassifyHealth(fcfPtr, cfqPtr, d2ePtr)
		ti.HealthRating = string(rating)
		ti.HealthReason = reason
	}
	if pePtr != nil || evPtr != nil {
		rating, reason := yahoo.ClassifyValuation(pePtr, evPtr)
		ti.ValuationRating = string(rating)
		ti.ValuationReason = reason
	}
}

func derefOr(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
