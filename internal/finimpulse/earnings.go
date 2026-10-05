package finimpulse

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Item types the earnings endpoint returns.
const (
	EarningsTypeEPS     = "eps_actual"
	EarningsTypeRevenue = "earnings_revenue"
	EarningsTypeGrowth  = "growth"
)

// MethodologyGAAP asks for figures as reported under GAAP.
const MethodologyGAAP = "gaap"

// EarningsRequest searches a symbol's earnings history.
type EarningsRequest struct {
	Symbol        string   `json:"symbol"`
	Types         []string `json:"types"`
	Methodologies []string `json:"methodologies,omitempty"`
	// StartDate and EndDate bound the reporting periods, as YYYY-MM-DD.
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

// NewEarningsRequest asks for one page of a symbol's earnings of the given
// types and methodology between from and to; an empty methodology asks for all.
func NewEarningsRequest(symbol string, types []string, methodology string, from, to time.Time,
	limit, offset int) EarningsRequest {
	request := EarningsRequest{
		Symbol: symbol, Types: types,
		StartDate: from.Format(time.DateOnly), EndDate: to.Format(time.DateOnly),
		Limit: limit, Offset: offset,
	}
	if methodology != "" {
		request.Methodologies = []string{methodology}
	}
	return request
}

// EarningsResult is one page of a symbol's earnings, with the analysts' price
// targets. Items arrive as one list of mixed types and are split by type;
// types this client does not know are skipped.
type EarningsResult struct {
	Symbol             string            `json:"symbol"`
	TargetPrice        *float64          `json:"target_price"`
	TargetAveragePrice *float64          `json:"target_average_price"`
	TargetLowPrice     *float64          `json:"target_low_price"`
	TargetHighPrice    *float64          `json:"target_high_price"`
	TotalCount         int               `json:"total_count"`
	ItemsCount         int               `json:"items_count"`
	Growth             []EarningsGrowth  `json:"-"`
	EPS                []EarningsEPS     `json:"-"`
	Revenue            []EarningsRevenue `json:"-"`
}

// EarningsPeriod identifies a reporting period: Date is the period's first
// day as YYYY-MM-DD and DateType its length, e.g. quarter.
type EarningsPeriod struct {
	Date     string `json:"date"`
	DateType string `json:"date_type"`
}

// EarningsGrowth is a period's revenue growth next to a benchmark index's.
type EarningsGrowth struct {
	EarningsPeriod
	Growth          *float64 `json:"growth"`
	GrowthBenchmark *float64 `json:"growth_benchmark"`
	SymbolBenchmark string   `json:"symbol_benchmark"`
}

// EarningsEPS is a period's earnings per share against the estimate.
type EarningsEPS struct {
	EarningsPeriod
	Methodology string   `json:"methodology"`
	Actual      *float64 `json:"actual"`
	Estimate    *float64 `json:"estimate"`
	Surprise    *float64 `json:"surprise"`
	SurprisePct *float64 `json:"surprise_pct"`
}

// EarningsRevenue is a period's revenue and net earnings.
type EarningsRevenue struct {
	EarningsPeriod
	Methodology string   `json:"methodology"`
	Revenue     *float64 `json:"revenue"`
	Earnings    *float64 `json:"earnings"`
}

// UnmarshalJSON implements json.Unmarshaler, splitting items by type.
func (r *EarningsResult) UnmarshalJSON(data []byte) error {
	type plain EarningsResult
	var raw struct {
		plain
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = EarningsResult(raw.plain)
	for _, item := range raw.Items {
		if err := r.addItem(item); err != nil {
			return err
		}
	}
	return nil
}

// addItem decodes one item into the slice of its type.
func (r *EarningsResult) addItem(item json.RawMessage) error {
	var kind struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(item, &kind); err != nil {
		return err
	}
	var err error
	switch kind.Type {
	case EarningsTypeGrowth:
		r.Growth, err = appendDecoded(r.Growth, item)
	case EarningsTypeEPS:
		r.EPS, err = appendDecoded(r.EPS, item)
	case EarningsTypeRevenue:
		r.Revenue, err = appendDecoded(r.Revenue, item)
	}
	if err != nil {
		return fmt.Errorf("earnings %s item: %w", kind.Type, err)
	}
	return nil
}

func appendDecoded[T any](items []T, data json.RawMessage) ([]T, error) {
	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		return items, err
	}
	return append(items, item), nil
}

// Items is how many items the page held, of every type.
func (r EarningsResult) Items() int {
	return len(r.Growth) + len(r.EPS) + len(r.Revenue)
}

// Earnings searches a symbol's earnings history.
func (c *Client) Earnings(ctx context.Context, request EarningsRequest) (Response[EarningsResult], error) {
	return post[EarningsResult](ctx, c, "/v1/analysis/earnings", request)
}
