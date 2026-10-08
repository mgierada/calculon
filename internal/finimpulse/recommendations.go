package finimpulse

import (
	"context"
	"time"
)

// RecommendationsRequest searches a symbol's analyst recommendations.
type RecommendationsRequest struct {
	Symbol string `json:"symbol"`
	// StartDate and EndDate bound the months, as YYYY-MM-DD.
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

// NewRecommendationsRequest asks for one page of a symbol's recommendations
// between from and to.
func NewRecommendationsRequest(symbol string, from, to time.Time, limit, offset int) RecommendationsRequest {
	return RecommendationsRequest{
		Symbol:    symbol,
		StartDate: from.Format(time.DateOnly), EndDate: to.Format(time.DateOnly),
		Limit: limit, Offset: offset,
	}
}

// RecommendationsResult is one page of a symbol's recommendations, newest
// month first.
type RecommendationsResult struct {
	TotalCount int              `json:"total_count"`
	ItemsCount int              `json:"items_count"`
	Items      []Recommendation `json:"items"`
}

// Recommendation counts the analysts rating a symbol each way in a month;
// Date is the month's first day as YYYY-MM-DD.
type Recommendation struct {
	Date       string `json:"date"`
	StrongBuy  int    `json:"strong_buy"`
	Buy        int    `json:"buy"`
	Hold       int    `json:"hold"`
	Sell       int    `json:"sell"`
	StrongSell int    `json:"strong_sell"`
}

// Recommendations searches a symbol's analyst recommendations.
func (c *Client) Recommendations(ctx context.Context, request RecommendationsRequest) (
	Response[RecommendationsResult], error) {
	return post[RecommendationsResult](ctx, c, "/v1/analysis/recommendations", request)
}
