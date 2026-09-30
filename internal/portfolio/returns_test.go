package portfolio

import (
	"math"
	"testing"
	"time"
)

var epoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// dayOffset is days from epoch, as the history's day index.
func dayOffset(t *testing.T, date string) int {
	t.Helper()
	d, err := time.Parse(time.DateOnly, date)
	if err != nil {
		t.Fatalf("bad date %q: %v", date, err)
	}
	return int(d.Sub(time.Date(2008, 1, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24)
}

// The example from Microsoft's XIRR documentation, which gives 0.373362535.
func TestXIRRMatchesSpreadsheetExample(t *testing.T) {
	// Flows are money put into the portfolio: the 10000 bought in is a
	// deposit, the later payouts are withdrawals.
	flows := []flow{
		{day: dayOffset(t, "2008-01-01"), amount: 10000},
		{day: dayOffset(t, "2008-03-01"), amount: -2750},
		{day: dayOffset(t, "2008-10-30"), amount: -4250},
		{day: dayOffset(t, "2009-02-15"), amount: -3250},
	}
	rate, ok := xirr(flows, dayOffset(t, "2009-04-01"), 2750, 0.1)
	if !ok || math.Abs(rate-0.373362535) > 1e-6 {
		t.Errorf("xirr = %v (ok %v), want 0.373362535", rate, ok)
	}
}

func TestXIRRFallsBackToBisection(t *testing.T) {
	flows := []flow{{day: 0, amount: 100}}
	// A wild guess sends Newton astray; bisection still finds 10%.
	rate, ok := xirr(flows, 365, 110, 500)
	if !ok || math.Abs(rate-0.10) > 1e-6 {
		t.Errorf("xirr = %v (ok %v), want 0.10", rate, ok)
	}
}

func TestXIRRNeedsMoneyInAndOut(t *testing.T) {
	if _, ok := xirr([]flow{{day: 0, amount: 100}}, 365, 0, 0.1); ok {
		t.Error("xirr solved flows that never came back")
	}
}

// history builds daily points: value per day, deposits keyed by day index.
func history(values []float64, deposits map[int]float64) []ValuePoint {
	points := make([]ValuePoint, len(values))
	var contributed float64
	for i, v := range values {
		contributed += deposits[i]
		points[i] = ValuePoint{Time: epoch.AddDate(0, 0, i), Value: v, Contributions: contributed}
	}
	return points
}

// growing is a portfolio compounding at an annual rate, with deposits added on
// top of the value as they arrive.
func growing(days int, rate float64, deposits map[int]float64) []float64 {
	daily := math.Pow(1+rate, 1/daysPerYear)
	values := make([]float64, days)
	var value float64
	for i := range values {
		value = value*daily + deposits[i]
		values[i] = value
	}
	return values
}

func TestReturnsOfSteadyGrowth(t *testing.T) {
	deposits := map[int]float64{0: 1000}
	returns := computeReturns(history(growing(731, 0.08, deposits), deposits))

	if !returns.CAGR.Known || math.Abs(returns.CAGR.Pct-8) > 0.01 {
		t.Errorf("CAGR = %+v, want 8%%", returns.CAGR)
	}
	// Two years at 8% compound to 16.64% in total.
	if !returns.TWR.Known || math.Abs(returns.TWR.Pct-16.64) > 0.01 {
		t.Errorf("TWR = %+v, want 16.64%% cumulative", returns.TWR)
	}
	if !returns.XIRR.Known || math.Abs(returns.XIRR.Pct-8) > 0.01 {
		t.Errorf("XIRR = %+v, want 8%% when there is a single deposit", returns.XIRR)
	}
}

// Time-weighted return must not care when money arrived; XIRR does.
func TestCAGRIgnoresDepositTiming(t *testing.T) {
	early := map[int]float64{0: 1000, 30: 10000}
	late := map[int]float64{0: 1000, 600: 10000}
	earlyReturns := computeReturns(history(growing(731, 0.08, early), early))
	lateReturns := computeReturns(history(growing(731, 0.08, late), late))

	if math.Abs(earlyReturns.TWR.Pct-lateReturns.TWR.Pct) > 0.01 {
		t.Errorf("TWR differs with deposit timing: %v vs %v",
			earlyReturns.TWR.Pct, lateReturns.TWR.Pct)
	}
	if math.Abs(earlyReturns.CAGR.Pct-lateReturns.CAGR.Pct) > 0.01 {
		t.Errorf("CAGR differs with deposit timing: %v vs %v",
			earlyReturns.CAGR.Pct, lateReturns.CAGR.Pct)
	}
	for _, r := range []Returns{earlyReturns, lateReturns} {
		if math.Abs(r.XIRR.Pct-8) > 0.05 {
			t.Errorf("XIRR = %v, want 8%% while every deposit earns 8%%", r.XIRR.Pct)
		}
	}
}

func TestReturnsSurviveWithdrawals(t *testing.T) {
	flows := map[int]float64{0: 10000, 200: -6000}
	returns := computeReturns(history(growing(500, 0.05, flows), flows))

	if math.Abs(returns.CAGR.Pct-5) > 0.01 || math.Abs(returns.XIRR.Pct-5) > 0.05 {
		t.Errorf("returns = CAGR %v, XIRR %v, want both 5%%", returns.CAGR.Pct, returns.XIRR.Pct)
	}
}

// Annual rates wait for MinReturnDays; the cumulative TWR is shown at once.
func TestAnnualReturnsNeedEnoughHistory(t *testing.T) {
	deposits := map[int]float64{0: 1000}
	returns := computeReturns(history(growing(MinReturnDays, 0.5, deposits), deposits))

	if returns.CAGR.Known || returns.XIRR.Known {
		t.Errorf("annual returns over %d days = %+v, want none yet", MinReturnDays, returns)
	}
	if !returns.TWR.Known || len(returns.Series) != MinReturnDays-1 {
		t.Errorf("TWR = %+v over %d points, want it from the second day", returns.TWR, len(returns.Series))
	}

	returns = computeReturns(history(growing(MinReturnDays+10, 0.5, deposits), deposits))
	first := 0
	for first < len(returns.Series) && !returns.Series[first].HasCAGR {
		first++
	}
	if !returns.Series[first].Time.Equal(epoch.AddDate(0, 0, MinReturnDays)) {
		t.Errorf("first annual point on %v, want day %d", returns.Series[first].Time, MinReturnDays)
	}
}

func TestReturnsLatestPicksMetric(t *testing.T) {
	r := Returns{TWR: Delta{Pct: 3}, XIRR: Delta{Pct: 1, Known: true}, CAGR: Delta{Pct: 2, Known: true}}
	if r.Latest(XIRR).Pct != 1 || r.Latest(CAGR).Pct != 2 || r.Latest(TWR).Pct != 3 {
		t.Errorf("Latest picked the wrong metric")
	}
}

func TestTWRIsTheDefaultMetric(t *testing.T) {
	if ReturnMetrics[0] != TWR || TWR.Annualized() || !XIRR.Annualized() {
		t.Errorf("metrics = %v, want TWR first and the only cumulative one", ReturnMetrics)
	}
}
