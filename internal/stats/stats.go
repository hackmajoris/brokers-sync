package stats

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"brokers-sync/internal/ledger"
	"brokers-sync/internal/model"
)

// PeriodSummary aggregates key metrics over a time range.
type PeriodSummary struct {
	Label       string    `json:"label"`
	Start       time.Time `json:"start,omitempty"`
	End         time.Time `json:"end,omitempty"`
	Realized    float64   `json:"realized_pnl"`
	Dividends   float64   `json:"dividends_net"`
	TaxWithheld float64   `json:"tax_withheld"`
	Fees        float64   `json:"fees"`        // custody / platform fees (TxFee)
	Commissions float64   `json:"commissions"` // per-trade commissions on buy/sell
	Deposits    float64   `json:"deposits"`
	Withdrawals float64   `json:"withdrawals"`
	BuyVolume   float64   `json:"buy_volume"`
	SellVolume  float64   `json:"sell_volume"`
	GainPct     float64   `json:"gain_pct"`          // (Realized + Dividends) / Deposits * 100
	MWRPct      float64   `json:"mwr_pct,omitempty"` // Modified Dietz money-weighted return
}

// DividendBySymbol totals net dividends (after tax) per ticker.
type DividendBySymbol struct {
	Symbol      string  `json:"symbol"`
	Gross       float64 `json:"gross"`
	TaxWithheld float64 `json:"tax_withheld"`
	Net         float64 `json:"net"`
}

// PositionSummary is a position enriched with optional live price data.
type PositionSummary struct {
	Symbol        string  `json:"symbol"`
	Currency      string  `json:"currency"` // native currency of the position's cost basis
	Quantity      float64 `json:"quantity"`
	AvgCost       float64 `json:"avg_cost"`   // in base currency
	TotalCost     float64 `json:"total_cost"` // in base currency
	CurrentPrice  float64 `json:"current_price,omitempty"`
	MarketValue   float64 `json:"market_value,omitempty"`
	UnrealizedPnL float64 `json:"unrealized_pnl,omitempty"`
	UnrealizedPct float64 `json:"unrealized_pct_omitempty,omitempty"`
}

// Summary aggregates stats from a fully-processed ledger + raw transactions.
type Summary struct {
	BaseCurrency  string
	OpenPositions []PositionSummary
	Realized      []ledger.RealizedTx
	AllTime       PeriodSummary
	YTD           PeriodSummary
	MTD           PeriodSummary
	ByYear        []PeriodSummary
	BySymbol      []DividendBySymbol
	CashBalance   float64 // uninvested cash: deposits - withdrawals - buys + sells + dividends + fees

	// Per-symbol qty/cost of lots opened within the YTD/MTD window (still open).
	// Used by RecalcGainPct to compute period-specific unrealized gains.
	ytdLotQty  map[string]float64
	ytdLotCost map[string]float64
	mtdLotQty  map[string]float64
	mtdLotCost map[string]float64

	// MWR (Modified Dietz) inputs pre-computed during Compute.
	// OpenCostBasis is the accumulated portfolio cost going INTO each period start
	// (includes lots later sold within the period, so it's the true BMV proxy).
	// WeightedCF is Σ(Wᵢ×CFᵢ) for cash flows within each period,
	// where Wᵢ = (periodEnd − cfDate) / periodLength.
	ytdOpenCostBasis  float64
	ytdWeightedCF     float64
	mtdBMV            float64
	mtdWeightedCF     float64
	allTimeWeightedCF float64
}

// accumulateWeightedCF adds amount to the Σ(Wᵢ×CFᵢ) denominators for each period.
// Wᵢ = (periodEnd − cfDate) / periodLength  (Modified Dietz time-weight).
func accumulateWeightedCF(s *Summary, amount float64, cfDate, now, ytdStart, mtdStart, allTimeStart time.Time) {
	daysWeight := func(periodStart time.Time) float64 {
		total := now.Sub(periodStart).Hours() / 24
		if total <= 0 {
			return 0
		}
		remaining := now.Sub(cfDate).Hours() / 24
		if remaining < 0 {
			remaining = 0
		}
		return remaining / total
	}

	// AllTime: every deposit/withdrawal contributes, weighted from account start.
	s.allTimeWeightedCF += daysWeight(allTimeStart) * amount

	// YTD: only cash flows that occurred on or after Jan 1.
	if !cfDate.Before(ytdStart) {
		s.ytdWeightedCF += daysWeight(ytdStart) * amount
	}

	// MTD: only cash flows on or after the 1st of the current month.
	if !cfDate.Before(mtdStart) {
		s.mtdWeightedCF += daysWeight(mtdStart) * amount
	}
}

// toBase converts an amount from the given currency to the base currency using fxRates.
// If the currency is missing from fxRates (unknown), the amount is returned unchanged.
func toBase(amount float64, currency string, fxRates map[string]float64) float64 {
	if len(fxRates) == 0 {
		return amount
	}
	if rate, ok := fxRates[currency]; ok {
		return amount * rate
	}
	return amount
}

// Compute builds the Summary from the ledger state and all transactions.
// fxRates maps currency codes to spot rates relative to baseCurrency (e.g. {"EUR":1.09,"RON":0.22}).
// Pass nil fxRates to skip normalization (amounts stay in their original currencies).
func Compute(l *ledger.Ledger, allTxs []model.Transaction, now time.Time, fxRates map[string]float64, baseCurrency string) Summary {
	ytdStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	mtdStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Determine year range from transaction data
	minYear, maxYear := now.Year(), now.Year()
	// allTimeStart is set after minYear is known (below); declare here for use in the tx loop.
	var allTimeStart time.Time
	for _, tx := range allTxs {
		y := tx.Date.Year()
		if y < minYear {
			minYear = y
		}
		if y > maxYear {
			maxYear = y
		}
	}

	allTimeStart = time.Date(minYear, 1, 1, 0, 0, 0, 0, now.Location())

	yearMap := make(map[int]*PeriodSummary, maxYear-minYear+1)
	for y := minYear; y <= maxYear; y++ {
		start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(y, 12, 31, 23, 59, 59, 0, time.UTC)
		yearMap[y] = &PeriodSummary{Label: fmt.Sprintf("%d", y), Start: start, End: end}
	}

	s := Summary{
		BaseCurrency: baseCurrency,
		Realized:     l.Realized,
		YTD:          PeriodSummary{Label: "YTD", Start: ytdStart, End: now},
		MTD:          PeriodSummary{Label: "MTD", Start: mtdStart, End: now},
		AllTime:      PeriodSummary{Label: "All Time"},
	}

	// Open positions — normalize each lot's cost basis to the base currency.
	type posRow struct {
		sym string
		pos *ledger.Position
	}
	var posRows []posRow
	for sym, pos := range l.Positions {
		if pos.Quantity > 1e-4 {
			posRows = append(posRows, posRow{sym, pos})
		}
	}
	sort.Slice(posRows, func(i, j int) bool { return posRows[i].sym < posRows[j].sym })
	for _, r := range posRows {
		var totalCostBase float64
		var posCurrency string
		for _, lot := range r.pos.Lots {
			totalCostBase += toBase(lot.CostBasis, lot.Currency, fxRates)
			if posCurrency == "" {
				posCurrency = lot.Currency
			}
		}
		avgCost := 0.0
		if r.pos.Quantity > 0 {
			avgCost = totalCostBase / r.pos.Quantity
		}
		s.OpenPositions = append(s.OpenPositions, PositionSummary{
			Symbol:    r.sym,
			Currency:  posCurrency,
			Quantity:  r.pos.Quantity,
			AvgCost:   avgCost,
			TotalCost: totalCostBase,
		})
	}

	// Collect per-symbol qty/cost for lots opened within the YTD and MTD windows.
	// We iterate the ledger positions (not posRows) so we see all symbols, and we
	// only look at lots that are still open (lot.Quantity > 0 after FIFO sells).
	s.ytdLotQty = make(map[string]float64)
	s.ytdLotCost = make(map[string]float64)
	s.mtdLotQty = make(map[string]float64)
	s.mtdLotCost = make(map[string]float64)
	for sym, pos := range l.Positions {
		if pos.Quantity <= 1e-4 {
			continue
		}
		for _, lot := range pos.Lots {
			if lot.Quantity <= 1e-9 {
				continue
			}
			lotCost := toBase(lot.CostBasis, lot.Currency, fxRates)
			if !lot.Date.Before(ytdStart) {
				s.ytdLotQty[sym] += lot.Quantity
				s.ytdLotCost[sym] += lotCost
			}
			if !lot.Date.Before(mtdStart) {
				s.mtdLotQty[sym] += lot.Quantity
				s.mtdLotCost[sym] += lotCost
			} else {
				s.mtdBMV += lotCost
			}
		}
	}

	// Realized P&L
	for _, r := range l.Realized {
		y := r.Date.Year()
		pnl := toBase(r.PnL, r.Currency, fxRates)
		s.AllTime.Realized += pnl
		if !r.Date.Before(ytdStart) {
			s.YTD.Realized += pnl
		}
		if !r.Date.Before(mtdStart) {
			s.MTD.Realized += pnl
		}
		if p, ok := yearMap[y]; ok {
			p.Realized += pnl
		}
	}

	// Dividends + tax withholding
	divMap := make(map[string]*DividendBySymbol)
	for _, tx := range l.Dividends {
		sym := tx.Symbol
		if sym == "" {
			sym = "(no symbol)"
		}
		d := divMap[sym]
		if d == nil {
			d = &DividendBySymbol{Symbol: sym}
			divMap[sym] = d
		}
		net := toBase(tx.Net, tx.Currency, fxRates)
		if tx.Type == model.TxTaxWithholding {
			d.TaxWithheld += -net
		} else {
			d.Gross += net
		}
		d.Net = d.Gross - d.TaxWithheld
	}

	for _, tx := range l.Dividends {
		y := tx.Date.Year()
		net := toBase(tx.Net, tx.Currency, fxRates)
		s.AllTime.Dividends += net
		if tx.Type == model.TxTaxWithholding {
			s.AllTime.TaxWithheld += net
		}
		if !tx.Date.Before(ytdStart) {
			s.YTD.Dividends += net
			if tx.Type == model.TxTaxWithholding {
				s.YTD.TaxWithheld += net
			}
		}
		if !tx.Date.Before(mtdStart) {
			s.MTD.Dividends += net
		}
		if p, ok := yearMap[y]; ok {
			p.Dividends += net
			if tx.Type == model.TxTaxWithholding {
				p.TaxWithheld += net
			}
		}
	}

	// Explicit fees (custody, platform)
	for _, tx := range l.Fees {
		y := tx.Date.Year()
		fee := toBase(tx.Net, tx.Currency, fxRates)
		s.AllTime.Fees += fee
		if !tx.Date.Before(ytdStart) {
			s.YTD.Fees += fee
		}
		if !tx.Date.Before(mtdStart) {
			s.MTD.Fees += fee
		}
		if p, ok := yearMap[y]; ok {
			p.Fees += fee
		}
	}

	// Per-trade commissions (IBKR, T212 — already baked into Net/cost basis, reported separately)
	for _, tx := range l.Commissions {
		y := tx.Date.Year()
		comm := toBase(tx.Commission, tx.Currency, fxRates)
		s.AllTime.Commissions += comm
		if !tx.Date.Before(ytdStart) {
			s.YTD.Commissions += comm
		}
		if !tx.Date.Before(mtdStart) {
			s.MTD.Commissions += comm
		}
		if p, ok := yearMap[y]; ok {
			p.Commissions += comm
		}
	}

	// Deposits, withdrawals, trade volume
	for _, tx := range allTxs {
		y := tx.Date.Year()
		yp := yearMap[y]

		switch tx.Type {
		case model.TxDeposit:
			dep := toBase(tx.Net, tx.Currency, fxRates)
			s.AllTime.Deposits += dep
			if !tx.Date.Before(ytdStart) {
				s.YTD.Deposits += dep
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.Deposits += dep
			}
			if yp != nil {
				yp.Deposits += dep
			}
			accumulateWeightedCF(&s, dep, tx.Date, now, ytdStart, mtdStart, allTimeStart)
		case model.TxWithdrawal:
			wd := toBase(tx.Net, tx.Currency, fxRates)
			s.AllTime.Withdrawals += wd
			if !tx.Date.Before(ytdStart) {
				s.YTD.Withdrawals += wd
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.Withdrawals += wd
			}
			if yp != nil {
				yp.Withdrawals += wd
			}
			accumulateWeightedCF(&s, wd, tx.Date, now, ytdStart, mtdStart, allTimeStart)
		case model.TxBuy:
			vol := toBase(tx.Quantity*tx.Price, tx.Currency, fxRates)
			s.AllTime.BuyVolume += vol
			if !tx.Date.Before(ytdStart) {
				s.YTD.BuyVolume += vol
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.BuyVolume += vol
			}
			if yp != nil {
				yp.BuyVolume += vol
			}
		case model.TxSell:
			vol := toBase(tx.Quantity*tx.Price, tx.Currency, fxRates)
			s.AllTime.SellVolume += vol
			if !tx.Date.Before(ytdStart) {
				s.YTD.SellVolume += vol
			}
			if !tx.Date.Before(mtdStart) {
				s.MTD.SellVolume += vol
			}
			if yp != nil {
				yp.SellVolume += vol
			}
		}
	}

	// Compute the actual cost basis added/removed per year so we can derive
	// what the portfolio's cost basis was at the start of each year.
	yearBuyCost := make(map[int]float64)
	for _, tx := range allTxs {
		if tx.Type != model.TxBuy {
			continue
		}
		var cost float64
		switch {
		case tx.Net < 0:
			cost = -tx.Net
		case tx.Net > 0:
			cost = tx.Net
		default:
			cost = tx.Quantity * tx.Price
		}
		yearBuyCost[tx.Date.Year()] += toBase(cost, tx.Currency, fxRates)
	}
	yearSoldCost := make(map[int]float64)
	for _, r := range l.Realized {
		yearSoldCost[r.Date.Year()] += toBase(r.CostBasis, r.Currency, fxRates)
	}

	// Roll the open cost basis forward year by year.
	// GainPct denominator = portfolio cost basis at start of year + new deposits that year.
	var openCostBasis float64
	for y := minYear; y <= maxYear; y++ {
		if y == now.Year() {
			// Capture the portfolio cost basis carried INTO this year (before any
			// buys/sells/deposits in the current year).  Used as BMV in RecalcGainPct.
			s.ytdOpenCostBasis = openCostBasis
		}
		yp := yearMap[y]
		base := openCostBasis + yp.Deposits
		if base > 0.01 {
			yp.GainPct = (yp.Realized + yp.Dividends) / base * 100
		}
		openCostBasis += yearBuyCost[y] - yearSoldCost[y]
	}

	// All-time: net capital deployed = deposits + withdrawals.
	// Withdrawals are stored as negative (tx.Net for 'out' rows is negative),
	// so addition correctly reduces the base.
	totalBase := s.AllTime.Deposits + s.AllTime.Withdrawals
	if totalBase > 0.01 {
		s.AllTime.GainPct = (s.AllTime.Realized + s.AllTime.Dividends) / totalBase * 100
	}
	// YTD: reuse the current year's base (portfolio cost at year-start + YTD deposits),
	// which is already computed in the per-year loop above and is more meaningful than
	// dividing by all-time deposits.
	if yp, ok := yearMap[now.Year()]; ok {
		s.YTD.GainPct = yp.GainPct
	}
	// MTD: use YTD deposits; fall back to all-time if no new deposits this year.
	mtdBase := s.YTD.Deposits
	if mtdBase < 0.01 {
		mtdBase = totalBase
	}
	if mtdBase > 0.01 {
		s.MTD.GainPct = (s.MTD.Realized + s.MTD.Dividends) / mtdBase * 100
	}

	// Flatten yearMap in ascending order
	for y := minYear; y <= maxYear; y++ {
		s.ByYear = append(s.ByYear, *yearMap[y])
	}

	// Sort dividend summaries by gross descending
	for _, d := range divMap {
		s.BySymbol = append(s.BySymbol, *d)
	}
	sort.Slice(s.BySymbol, func(i, j int) bool {
		return s.BySymbol[i].Gross > s.BySymbol[j].Gross
	})

	// Cash balance: uninvested cash remaining in the account.
	for _, tx := range allTxs {
		switch tx.Type {
		case model.TxDeposit:
			s.CashBalance += toBase(tx.Net, tx.Currency, fxRates)
		case model.TxWithdrawal:
			s.CashBalance += toBase(tx.Net, tx.Currency, fxRates)
		case model.TxBuy:
			var cost float64
			switch {
			case tx.Net < 0:
				cost = -tx.Net
			case tx.Net > 0:
				cost = tx.Net
			default:
				cost = tx.Quantity * tx.Price
			}
			s.CashBalance -= toBase(cost, tx.Currency, fxRates)
		case model.TxSell:
			proceeds := tx.Net
			if proceeds <= 0 {
				proceeds = tx.Quantity * tx.Price
			}
			s.CashBalance += toBase(proceeds, tx.Currency, fxRates)
		case model.TxDividend, model.TxTaxWithholding:
			s.CashBalance += toBase(tx.Net, tx.Currency, fxRates)
		case model.TxFee:
			s.CashBalance += toBase(tx.Net, tx.Currency, fxRates)
		}
	}

	return s
}

// RecalcGainPct recomputes GainPct for AllTime, YTD, and MTD after positions
// have been enriched with live prices, so that unrealized appreciation is
// reflected in the return figures (not just realized P&L and dividends).
func RecalcGainPct(s *Summary) {
	// Build a symbol→currentPrice map from enriched positions.
	priceBySymbol := make(map[string]float64, len(s.OpenPositions))
	var totalUnrealized float64
	for _, p := range s.OpenPositions {
		if p.CurrentPrice > 0 {
			priceBySymbol[p.Symbol] = p.CurrentPrice
		}
		totalUnrealized += p.UnrealizedPnL
	}

	// Compute unrealized P&L for lots opened within each period window.
	periodUnrealized := func(lotQty, lotCost map[string]float64) float64 {
		var u float64
		for sym, qty := range lotQty {
			if price, ok := priceBySymbol[sym]; ok {
				u += price*qty - lotCost[sym]
			}
		}
		return u
	}
	mtdUnrealized := periodUnrealized(s.mtdLotQty, s.mtdLotCost)

	// AllTime: net capital = deposits + withdrawals (withdrawals are stored as negative).
	totalDeposited := s.AllTime.Deposits + s.AllTime.Withdrawals
	if totalDeposited > 0.01 {
		s.AllTime.GainPct = (s.AllTime.Realized + s.AllTime.Dividends + totalUnrealized) / totalDeposited * 100
	}

	// YTD: denominator = cost basis carried into the year + new deposits this year.
	// This matches the by_year denominator and avoids the "divide by 381" bug when
	// most capital was deployed in prior years.
	ytdUnrealized := periodUnrealized(s.ytdLotQty, s.ytdLotCost)
	ytdBase := s.ytdOpenCostBasis + s.YTD.Deposits
	if ytdBase > 0.01 {
		s.YTD.GainPct = (s.YTD.Realized + s.YTD.Dividends + ytdUnrealized) / ytdBase * 100
	}

	// MTD: pre-month portfolio cost + new deposits this month.
	if mtdBase := s.mtdBMV + s.MTD.Deposits; mtdBase > 0.01 {
		s.MTD.GainPct = (s.MTD.Realized + s.MTD.Dividends + mtdUnrealized) / mtdBase * 100
	}

	// ---- MWR (Modified Dietz) ----
	// EMV = current market value of all open positions (with live prices where available,
	// falling back to cost basis for unpriced positions) + uninvested cash.
	var emv float64
	for _, p := range s.OpenPositions {
		if p.MarketValue > 0 {
			emv += p.MarketValue
		} else {
			emv += p.TotalCost
		}
	}
	emv += s.CashBalance

	// AllTime MWR: BMV = 0 (account started with nothing).
	// gain = EMV − net deposits; denominator = Σ(Wᵢ×CFᵢ) over all time.
	if s.allTimeWeightedCF > 0.01 {
		s.AllTime.MWRPct = (emv - totalDeposited) / s.allTimeWeightedCF * 100
	}

	// YTD MWR: BMV = portfolio cost basis at Jan 1 (includes lots sold during the year).
	ytdNetCF := s.YTD.Deposits + s.YTD.Withdrawals
	if ytdDenom := s.ytdOpenCostBasis + s.ytdWeightedCF; ytdDenom > 0.01 {
		s.YTD.MWRPct = (emv - s.ytdOpenCostBasis - ytdNetCF) / ytdDenom * 100
	}

	// MTD MWR: same logic scoped to the current month.
	mtdNetCF := s.MTD.Deposits + s.MTD.Withdrawals
	if mtdDenom := s.mtdBMV + s.mtdWeightedCF; mtdDenom > 0.01 {
		s.MTD.MWRPct = (emv - s.mtdBMV - mtdNetCF) / mtdDenom * 100
	}
}

// EnrichWithPrices adds live price data to open positions.
// prices is a map of symbol (or Yahoo-normalized symbol) → current price in the symbol's native currency.
// fxRates converts native prices to the summary's base currency (pass nil to skip conversion).
func EnrichWithPrices(s *Summary, prices map[string]float64, fxRates map[string]float64) {
	for i := range s.OpenPositions {
		p := &s.OpenPositions[i]
		price, ok := prices[p.Symbol]
		if !ok {
			// Try Yahoo-normalized form (e.g. "BRK-B" for "BRK B")
			norm := strings.ReplaceAll(p.Symbol, " ", "-")
			price, ok = prices[norm]
		}
		if !ok {
			continue
		}
		priceBase := toBase(price, p.Currency, fxRates)
		p.CurrentPrice = priceBase
		p.MarketValue = priceBase * p.Quantity
		p.UnrealizedPnL = p.MarketValue - p.TotalCost
		if p.TotalCost > 0.01 {
			p.UnrealizedPct = p.UnrealizedPnL / p.TotalCost * 100
		}
	}
}

// RealizedBySymbol returns realized P&L grouped by symbol.
func RealizedBySymbol(realized []ledger.RealizedTx) map[string]float64 {
	m := make(map[string]float64)
	for _, r := range realized {
		m[r.Symbol] += r.PnL
	}
	return m
}
