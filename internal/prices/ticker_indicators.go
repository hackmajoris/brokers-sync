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

	YTD       *float64
	ThreeYear *float64
	FiveYear  *float64

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
	client, err := yahoo.New()
	if err != nil {
		return nil, false
	}

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
		ti.YTD = &p.YTD
		ti.ThreeYear = &p.ThreeYear
		ti.FiveYear = &p.FiveYear
		return true
	})
	run(func() bool {
		v, err := client.GetFreeCashFlow(ctx, symbol)
		if err != nil {
			return false
		}
		ti.FCF = &v.FCF
		ti.FCFInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetEVToEBITDA(ctx, symbol)
		if err != nil {
			return false
		}
		ti.EVToEBITDA = &v.Ratio
		ti.EVInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetDebtToEquity(ctx, symbol)
		if err != nil {
			return false
		}
		ti.DebtToEquity = &v.Ratio
		ti.DebtEqInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetOperatingCashFlowVsNetIncome(ctx, symbol)
		if err != nil {
			return false
		}
		ti.CashFlowQuality = &v.Ratio
		ti.cfqNetIncome = v.NetIncome
		ti.CFQInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetMarketCap(ctx, symbol)
		if err != nil {
			return false
		}
		ti.MarketCap = &v.MarketCap
		ti.MarketCapInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetPriceToSales(ctx, symbol)
		if err != nil {
			return false
		}
		ti.PriceToSales = &v.Ratio
		ti.PriceToSalesInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetPriceToBook(ctx, symbol)
		if err != nil {
			return false
		}
		ti.PriceToBook = &v.Ratio
		ti.PriceToBookInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetFreeCashFlowYield(ctx, symbol)
		if err != nil {
			return false
		}
		ti.FCFYield = &v.Yield
		ti.FCFYieldInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetProfitMargin(ctx, symbol)
		if err != nil {
			return false
		}
		ti.ProfitMargin = &v.Margin
		ti.ProfitMarginInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetOperatingMargin(ctx, symbol)
		if err != nil {
			return false
		}
		ti.OperatingMargin = &v.Margin
		ti.OperatingMarginInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetQuarterlyEarningsGrowth(ctx, symbol)
		if err != nil {
			return false
		}
		ti.QuarterlyEarningsGrowth = &v.Growth
		ti.QuarterlyEarningsGrowthInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetQuarterlyRevenueGrowth(ctx, symbol)
		if err != nil {
			return false
		}
		ti.QuarterlyRevenueGrowth = &v.Growth
		ti.QuarterlyRevenueGrowthInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetCash(ctx, symbol)
		if err != nil {
			return false
		}
		ti.Cash = &v.Cash
		ti.CashInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetDebt(ctx, symbol)
		if err != nil {
			return false
		}
		ti.Debt = &v.Debt
		ti.DebtInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetDividendYield(ctx, symbol)
		if err != nil {
			return false
		}
		ti.DividendYield = &v.Yield
		ti.DividendYieldInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetPayoutRatio(ctx, symbol)
		if err != nil {
			return false
		}
		ti.PayoutRatio = &v.Ratio
		ti.PayoutRatioInterp = v.Interpretation
		return true
	})
	run(func() bool {
		v, err := client.GetPayoutDate(ctx, symbol)
		if err != nil {
			return false
		}
		ti.PayoutDate = &v.Date
		ti.PayoutDateInterp = v.Interpretation
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
