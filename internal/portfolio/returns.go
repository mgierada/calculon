package portfolio

import (
	"math"
	"time"
)

// ReturnMetric is a way of annualizing the portfolio's return.
type ReturnMetric int

const (
	// TWR is the time-weighted return since the start, not annualized: daily
	// returns with the day's deposits and withdrawals taken out, chained. It
	// measures the portfolio's performance independent of when money moved.
	TWR ReturnMetric = iota
	// XIRR is the money-weighted return: the one rate that turns every
	// deposit and withdrawal into today's value. It rewards adding money
	// before good periods, so it answers "what did my money earn".
	XIRR
	// CAGR is the time-weighted return, annualized: daily returns with the
	// day's deposits and withdrawals taken out, chained and compounded. It
	// ignores when money was added, so it answers "how did the portfolio
	// perform". Plain start-to-end CAGR would mostly measure the deposits.
	CAGR
)

// ReturnMetrics lists the metrics in the order a toggle cycles through them,
// the default first.
var ReturnMetrics = []ReturnMetric{TWR, XIRR, CAGR}

// String names the metric for display.
func (m ReturnMetric) String() string {
	switch m {
	case XIRR:
		return "XIRR"
	case CAGR:
		return "CAGR"
	default:
		return "TWR"
	}
}

// Annualized reports whether the metric is a yearly rate.
func (m ReturnMetric) Annualized() bool {
	return m != TWR
}

// MinReturnDays is how much history an annualized return needs before it is
// shown. Annualizing a few weeks turns noise into absurd yearly rates.
const MinReturnDays = 90

const daysPerYear = 365.0

// ReturnPoint is every metric as of the end of one day, in percent, each
// computed over everything up to that day. TWR is cumulative; XIRR and CAGR
// are annual rates and only set once MinReturnDays have passed.
type ReturnPoint struct {
	Time time.Time
	TWR  float64
	XIRR float64
	CAGR float64
	// HasCAGR is false until there is enough history to annualize.
	HasCAGR bool
	// HasXIRR is also false when no rate solves the day's cash flows.
	HasXIRR bool
}

// Value is the metric at this point, and whether it is known.
func (p ReturnPoint) Value(metric ReturnMetric) (float64, bool) {
	switch metric {
	case XIRR:
		return p.XIRR, p.HasXIRR
	case CAGR:
		return p.CAGR, p.HasCAGR
	default:
		return p.TWR, true
	}
}

// Returns are the portfolio's returns now and over time.
type Returns struct {
	TWR    Delta
	XIRR   Delta
	CAGR   Delta
	Series []ReturnPoint
}

// Latest is the metric's current value.
func (r Returns) Latest(metric ReturnMetric) Delta {
	switch metric {
	case XIRR:
		return r.XIRR
	case CAGR:
		return r.CAGR
	default:
		return r.TWR
	}
}

// flow is money moved into (positive) or out of the portfolio on one day.
type flow struct {
	day    int
	amount float64
}

// computeReturns derives both metrics from the daily value history. The day
// to day change in contributions is that day's external cash flow, so the
// history alone is enough and any account scope works the same way.
func computeReturns(history []ValuePoint) Returns {
	if len(history) == 0 {
		return Returns{}
	}

	var (
		returns Returns
		flows   []flow
		growth  = 1.0
		guess   = 0.1
	)
	for i, point := range history {
		prevContrib, prevValue := 0.0, 0.0
		if i > 0 {
			prevContrib, prevValue = history[i-1].Contributions, history[i-1].Value
		}
		dayFlow := point.Contributions - prevContrib
		if dayFlow != 0 {
			flows = append(flows, flow{day: i, amount: dayFlow})
		}
		// Flows land during the day, so the day's return is measured on what
		// the portfolio held going in. A day starting empty has no return.
		if prevValue > 0 {
			growth *= (point.Value - dayFlow) / prevValue
		}

		if i == 0 {
			continue
		}
		p := ReturnPoint{Time: point.Time, TWR: (growth - 1) * 100}
		if i >= MinReturnDays {
			years := float64(i) / daysPerYear
			p.CAGR, p.HasCAGR = annualize(growth, years)*100, true
			if rate, ok := xirr(flows, i, point.Value, guess); ok {
				p.XIRR, p.HasXIRR, guess = rate*100, true, rate
			}
		}
		returns.Series = append(returns.Series, p)
	}

	if n := len(returns.Series); n > 0 {
		last := returns.Series[n-1]
		returns.TWR = Delta{Amount: last.TWR, Pct: last.TWR, Known: true}
		returns.CAGR = Delta{Amount: last.CAGR, Pct: last.CAGR, Known: last.HasCAGR}
		returns.XIRR = Delta{Amount: last.XIRR, Pct: last.XIRR, Known: last.HasXIRR}
	}
	return returns
}

// annualize turns total growth over some years into a yearly rate.
func annualize(growth, years float64) float64 {
	if growth <= 0 || years <= 0 {
		return -1
	}
	return math.Pow(growth, 1/years) - 1
}

// xirr solves for the annual rate at which the investor's cash flows (money
// put in is an outflow for them) and the final value, held on day end, net to
// zero. Newton's method from guess converges in a few steps when warm-started
// from the previous day; bisection takes over when it does not.
func xirr(flows []flow, end int, value, guess float64) (float64, bool) {
	cashflows := make([]flow, 0, len(flows)+1)
	for _, f := range flows {
		cashflows = append(cashflows, flow{day: f.day, amount: -f.amount})
	}
	cashflows = append(cashflows, flow{day: end, amount: value})
	if !hasBothSigns(cashflows) {
		return 0, false
	}

	if rate, ok := newton(cashflows, guess); ok {
		return rate, true
	}
	return bisect(cashflows)
}

// npv discounts cash flows to the first flow's day at an annual rate, and
// returns its derivative with respect to the rate as well.
func npv(cashflows []flow, rate float64) (float64, float64) {
	var value, derivative float64
	start := cashflows[0].day
	for _, cf := range cashflows {
		t := float64(cf.day-start) / daysPerYear
		discount := math.Pow(1+rate, -t)
		value += cf.amount * discount
		derivative -= t * cf.amount * discount / (1 + rate)
	}
	return value, derivative
}

const (
	xirrTolerance  = 1e-7
	xirrIterations = 50
	// The bracket bisection searches: nearly total loss up to 100000%/yr.
	xirrLow  = -0.9999
	xirrHigh = 1000.0
)

func newton(cashflows []flow, guess float64) (float64, bool) {
	rate := guess
	for range xirrIterations {
		value, derivative := npv(cashflows, rate)
		if derivative == 0 || math.IsNaN(value) {
			return 0, false
		}
		next := rate - value/derivative
		if next <= xirrLow || math.IsNaN(next) || math.IsInf(next, 0) {
			return 0, false
		}
		if math.Abs(next-rate) < xirrTolerance {
			return next, true
		}
		rate = next
	}
	return 0, false
}

func bisect(cashflows []flow) (float64, bool) {
	low, high := xirrLow, xirrHigh
	lowValue, _ := npv(cashflows, low)
	highValue, _ := npv(cashflows, high)
	if lowValue*highValue > 0 {
		return 0, false
	}
	for range 200 {
		mid := (low + high) / 2
		midValue, _ := npv(cashflows, mid)
		if math.Abs(midValue) < xirrTolerance || high-low < xirrTolerance {
			return mid, true
		}
		if midValue*lowValue < 0 {
			high = mid
		} else {
			low, lowValue = mid, midValue
		}
	}
	return (low + high) / 2, true
}

// hasBothSigns reports whether money went both in and out, without which no
// rate can balance the flows.
func hasBothSigns(cashflows []flow) bool {
	var in, out bool
	for _, cf := range cashflows {
		in = in || cf.amount > 0
		out = out || cf.amount < 0
	}
	return in && out
}
