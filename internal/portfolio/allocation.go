package portfolio

import "sort"

// Grouping says what an allocation is broken down by.
type Grouping int

const (
	BySymbol Grouping = iota
	ByAccount
	ByCategory
	ByCurrency
)

// Groupings lists every grouping, in the order a toggle cycles through them.
var Groupings = []Grouping{BySymbol, ByAccount, ByCategory, ByCurrency}

// String names the grouping for display.
func (g Grouping) String() string {
	switch g {
	case ByAccount:
		return "account"
	case ByCategory:
		return "category"
	case ByCurrency:
		return "currency"
	default:
		return "symbol"
	}
}

// cashLabel is the slice uninvested cash is reported under.
const cashLabel = "cash"

// Slice is one group's share of the portfolio.
type Slice struct {
	Label  string
	Value  float64
	Weight float64
}

// Allocation splits the portfolio total, cash included, by grouping, largest
// first. Cash is its own slice except when grouping by account or currency,
// where it belongs to the account or currency it sits in.
func Allocation(report Report, grouping Grouping) []Slice {
	values := map[string]float64{}
	for _, h := range report.Holdings {
		values[groupLabel(h, grouping)] += h.ValueBase
	}
	for _, account := range report.Accounts {
		cash := account.CashBase
		switch grouping {
		case ByAccount:
			values[account.Label()] += cash
		case ByCurrency:
			values[account.Currency] += cash
		default:
			values[cashLabel] += cash
		}
	}

	slices := make([]Slice, 0, len(values))
	for label, value := range values {
		if value == 0 {
			continue
		}
		slices = append(slices, Slice{Label: label, Value: value, Weight: pct(value, report.Totals.Total)})
	}
	sort.SliceStable(slices, func(i, j int) bool {
		if slices[i].Value != slices[j].Value {
			return slices[i].Value > slices[j].Value
		}
		return slices[i].Label < slices[j].Label
	})
	return slices
}

// TopSlices keeps the n largest slices and folds the rest into "other".
func TopSlices(slices []Slice, n int) []Slice {
	if n <= 0 || len(slices) <= n {
		return slices
	}
	top := append([]Slice{}, slices[:n-1]...)
	other := Slice{Label: "other"}
	for _, s := range slices[n-1:] {
		other.Value += s.Value
		other.Weight += s.Weight
	}
	return append(top, other)
}

func groupLabel(h Holding, grouping Grouping) string {
	switch grouping {
	case ByAccount:
		return h.Account.Label()
	case ByCategory:
		return h.Category
	case ByCurrency:
		return h.Account.Currency
	default:
		return h.Symbol
	}
}

// TopSymbols lists the n symbols worth the most, biggest first, a symbol held
// in several accounts counting their value together.
func TopSymbols(holdings []Holding, n int) []string {
	if n <= 0 {
		return nil
	}
	value := map[string]float64{}
	var symbols []string
	for _, h := range holdings {
		if _, seen := value[h.Symbol]; !seen {
			symbols = append(symbols, h.Symbol)
		}
		value[h.Symbol] += h.ValueBase
	}
	sort.SliceStable(symbols, func(i, j int) bool { return value[symbols[i]] > value[symbols[j]] })
	return symbols[:min(n, len(symbols))]
}
