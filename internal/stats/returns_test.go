package stats_test

import (
	"testing"
	"time"

	"brokers-sync/internal/stats"
)

// ---- TWR (Time-Weighted Return) ----

// TestTWR_SubPeriodReturn verifies the sub-period formula using the first data
// point from the IBKR white paper:
//
//	BMV = 4,549,863.44  EMV = 4,629,129.14  CF = 0  → R = 1.74%
func TestTWR_SubPeriodReturn(t *testing.T) {
	r := stats.SubPeriodReturn(4_549_863.44, 4_629_129.14, 0)
	near(t, "sub-period return", r*100, 1.74, 0.01)
}

// TestTWR_SubPeriodReturn_WithCashFlow verifies that a cash flow shifts the
// denominator.  If BMV = 1000, CF = +200, and EMV = 1260, the adjusted base
// is 1200 → R = (1260−1200)/1200 = 5%.
func TestTWR_SubPeriodReturn_WithCashFlow(t *testing.T) {
	r := stats.SubPeriodReturn(1000, 1260, 200)
	near(t, "sub-period with inflow", r*100, 5.0, 0.001)
}

// TestTWR_WhitePaperChain reproduces the IBKR white-paper worked example.
// Five consecutive daily sub-period returns are geometrically linked:
//
//	R₁=+1.74%  R₂=−4.68%  R₃=+1.92%  R₄=−0.69%  R₅=+2.02%
//
// TWR = (1.0174)(0.9532)(1.0192)(0.9931)(1.0202) − 1 ≈ +0.14%
func TestTWR_WhitePaperChain(t *testing.T) {
	returns := []float64{0.0174, -0.0468, 0.0192, -0.0069, 0.0202}
	twr := stats.TWR(returns)
	near(t, "IBKR white-paper TWR", twr*100, 0.14, 0.02)
}

// TestTWR_SinglePeriodNoFlow confirms that with one sub-period and no cash
// flow the result collapses to a plain percentage gain.
func TestTWR_SinglePeriodNoFlow(t *testing.T) {
	// 10% gain: BMV=1000, EMV=1100, CF=0
	r := stats.SubPeriodReturn(1000, 1100, 0)
	twr := stats.TWR([]float64{r})
	near(t, "single-period TWR", twr*100, 10.0, 0.001)
}

// TestTWR_FromSubPeriods exercises the end-to-end helper that accepts
// SubPeriod structs and chains them.
func TestTWR_FromSubPeriods(t *testing.T) {
	// Reproduce white-paper first sub-period then a flat second period.
	periods := []stats.SubPeriod{
		{BMV: 4_549_863.44, EMV: 4_629_129.14, CF: 0}, // +1.74%
		{BMV: 4_629_129.14, EMV: 4_629_129.14, CF: 0}, // 0% (flat)
	}
	twr := stats.TWRFromSubPeriods(periods)
	near(t, "two-period TWR (flat second)", twr*100, 1.74, 0.02)
}

// TestTWR_NeutralPeriod ensures that a 0% sub-period has no effect on the chain.
func TestTWR_NeutralPeriod(t *testing.T) {
	returns := []float64{0.10, 0.0, -0.05}
	twr := stats.TWR(returns)
	// (1.10)(1.00)(0.95) − 1 = 0.045 = +4.5%
	near(t, "neutral period passthrough", twr*100, 4.5, 0.001)
}

// ---- MWR (Modified Dietz) ----

// TestMWR_NoFlows verifies that with no cash flows Modified Dietz reduces to
// the simple return (EMV−BMV)/BMV.
func TestMWR_NoFlows(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	mwr := stats.MWRModifiedDietz(1000, 1100, nil, start, end)
	near(t, "MWR no flows", mwr*100, 10.0, 0.001)
}

// TestMWR_MidPeriodDeposit checks a deposit made exactly halfway through a
// 365-day year.  The time-weight Wᵢ = days_remaining / total_days.
//
//	BMV=1000, EMV=1150, CF=+100 on day 182 of 365
//	W = (365−182)/365 = 183/365 ≈ 0.5014
//	MWR = (1150−1000−100) / (1000 + 0.5014×100) = 50/1050.14 ≈ 4.76%
func TestMWR_MidPeriodDeposit(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	cfDate := start.AddDate(0, 0, 182) // day 182

	flows := []stats.CashFlow{{Date: cfDate, Amount: 100}}
	mwr := stats.MWRModifiedDietz(1000, 1150, flows, start, end)

	totalDays := end.Sub(start).Hours() / 24
	daysRemaining := end.Sub(cfDate).Hours() / 24
	w := daysRemaining / totalDays
	expected := 50.0 / (1000 + w*100) * 100

	near(t, "MWR mid-period deposit", mwr*100, expected, 0.01)
}

// TestMWR_EarlyDepositHighWeight confirms that a deposit at the very start
// of the period receives full weight (W≈1), so the denominator is maximally
// inflated by that cash flow.
func TestMWR_EarlyDepositHighWeight(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	cfDate := start.AddDate(0, 0, 1) // day 1: almost full weight

	flows := []stats.CashFlow{{Date: cfDate, Amount: 500}}
	// BMV=1000, deposit 500 on day 1; total invested ≈ 1500
	// If EMV=1600 → gain=100; MWR ≈ 100 / (1000 + ~1*500) ≈ 6.67%
	mwr := stats.MWRModifiedDietz(1000, 1600, flows, start, end)

	totalDays := end.Sub(start).Hours() / 24
	daysRemaining := end.Sub(cfDate).Hours() / 24
	w := daysRemaining / totalDays
	expected := (1600 - 1000 - 500) / (1000 + w*500) * 100

	near(t, "MWR early deposit", mwr*100, expected, 0.01)
}

// TestMWR_Withdrawal confirms that a withdrawal reduces the weighted denominator.
//
//	BMV=1000, withdraw −200 at mid-year, EMV=700
//	Net gain = 700 − 1000 − (−200) = −100
//	W ≈ 0.5, denominator = 1000 + 0.5*(−200) = 900
//	MWR ≈ −100/900 = −11.11%
func TestMWR_Withdrawal(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	cfDate := start.AddDate(0, 6, 0) // ~mid-year

	flows := []stats.CashFlow{{Date: cfDate, Amount: -200}}
	mwr := stats.MWRModifiedDietz(1000, 700, flows, start, end)

	totalDays := end.Sub(start).Hours() / 24
	daysRemaining := end.Sub(cfDate).Hours() / 24
	w := daysRemaining / totalDays
	expected := (700 - 1000 - (-200)) / (1000 + w*(-200)) * 100

	near(t, "MWR withdrawal", mwr*100, expected, 0.01)
}

// TestMWR_MultipleFlows checks that multiple cash flows are each weighted
// independently and summed in the denominator.
func TestMWR_MultipleFlows(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	// Two deposits: +200 at day 91, +300 at day 274
	d1 := start.AddDate(0, 0, 91)
	d2 := start.AddDate(0, 0, 274)
	flows := []stats.CashFlow{
		{Date: d1, Amount: 200},
		{Date: d2, Amount: 300},
	}

	totalDays := end.Sub(start).Hours() / 24
	w1 := end.Sub(d1).Hours() / 24 / totalDays
	w2 := end.Sub(d2).Hours() / 24 / totalDays

	// BMV=1000, EMV=1620, net CF=500
	// gain = 1620 − 1000 − 500 = 120
	// denom = 1000 + w1*200 + w2*300
	denom := 1000 + w1*200 + w2*300
	expected := 120 / denom * 100

	mwr := stats.MWRModifiedDietz(1000, 1620, flows, start, end)
	near(t, "MWR multiple flows", mwr*100, expected, 0.01)
}
