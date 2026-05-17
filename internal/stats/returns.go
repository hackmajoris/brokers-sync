package stats

import "time"

// SubPeriod is one performance interval for a TWR calculation.
// A new sub-period begins at every cash flow event.
type SubPeriod struct {
	BMV float64 // beginning market value
	EMV float64 // ending market value
	CF  float64 // net cash flow during this sub-period (positive = inflow)
}

// CashFlow is a dated deposit or withdrawal.
// Amount is positive for deposits, negative for withdrawals.
type CashFlow struct {
	Date   time.Time
	Amount float64
}

// SubPeriodReturn computes the return for a single sub-period using the IBKR
// Time-Weighted Return formula:
//
//	Rₙ = (EMV − (BMV + CF)) / (BMV + CF)
//
// Returns 0 when the adjusted base is zero or negative.
func SubPeriodReturn(bmv, emv, cf float64) float64 {
	base := bmv + cf
	if base <= 0 {
		return 0
	}
	return (emv - base) / base
}

// TWR geometrically chains a slice of sub-period returns into a single
// period return (as a decimal, not a percentage):
//
//	TWR = (1+R₁)(1+R₂)…(1+Rₙ) − 1
func TWR(returns []float64) float64 {
	product := 1.0
	for _, r := range returns {
		product *= 1 + r
	}
	return product - 1
}

// TWRFromSubPeriods computes the TWR from a sequence of SubPeriod values.
// Each sub-period return is derived via SubPeriodReturn before chaining.
func TWRFromSubPeriods(periods []SubPeriod) float64 {
	returns := make([]float64, len(periods))
	for i, p := range periods {
		returns[i] = SubPeriodReturn(p.BMV, p.EMV, p.CF)
	}
	return TWR(returns)
}

// MWRModifiedDietz computes the Money-Weighted Return using the Modified Dietz
// method as specified by IBKR:
//
//	MWR = (EMV − BMV − CF) / (BMV + Σ(Wᵢ × CFᵢ))
//
// where Wᵢ = (T − tᵢ) / T is the fraction of the period remaining after
// cash flow i, T is the total period length in days, and tᵢ is the number
// of days elapsed from period start to the cash flow date.
//
// Returns 0 when the denominator is zero or negative.
func MWRModifiedDietz(bmv, emv float64, cashFlows []CashFlow, periodStart, periodEnd time.Time) float64 {
	totalDays := periodEnd.Sub(periodStart).Hours() / 24
	if totalDays <= 0 {
		return 0
	}

	var totalCF, weightedCF float64
	for _, cf := range cashFlows {
		totalCF += cf.Amount
		daysRemaining := periodEnd.Sub(cf.Date).Hours() / 24
		w := daysRemaining / totalDays
		weightedCF += w * cf.Amount
	}

	denom := bmv + weightedCF
	if denom <= 0 {
		return 0
	}
	return (emv - bmv - totalCF) / denom
}
