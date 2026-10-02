package finimpulse

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// newsTimeLayout is how the news endpoint writes times: no zone, in UTC.
const newsTimeLayout = time.DateTime

// NewsTypeNews asks the news endpoint for articles.
const NewsTypeNews = "news"

// NewsRequest searches the news about one symbol.
type NewsRequest struct {
	Symbol string   `json:"symbol"`
	Types  []string `json:"types"`
	// StartDate and EndDate bound the publication dates, as YYYY-MM-DD.
	StartDate string   `json:"start_date"`
	EndDate   string   `json:"end_date"`
	Limit     int      `json:"limit"`
	Offset    int      `json:"offset"`
	SortBy    []SortBy `json:"sort_by"`
}

// SortBy orders a search by one field.
type SortBy struct {
	Selector string `json:"selector"`
	Desc     bool   `json:"desc"`
}

// NewsResult is one page of a news search.
type NewsResult struct {
	TotalCount       int        `json:"total_count"`
	ItemsCount       int        `json:"items_count"`
	SearchAfterToken string     `json:"search_after_token"`
	Items            []NewsItem `json:"items"`
}

// NewsItem is one article.
type NewsItem struct {
	ID                  string   `json:"id"`
	Type                string   `json:"type"`
	Title               string   `json:"title"`
	Description         string   `json:"description"`
	PubDate             NewsTime `json:"pub_date"`
	DisplayTime         NewsTime `json:"display_time"`
	CanonicalURL        string   `json:"canonical_url"`
	ContentType         string   `json:"content_type"`
	RelatedTickers      []string `json:"related_tickers"`
	ProviderDisplayName string   `json:"provider_display_name"`
	ProviderURL         string   `json:"provider_url"`
	IsHosted            bool     `json:"is_hosted"`
	IsPremiumNews       bool     `json:"is_premium_news"`
}

// NewsTime is a news timestamp, sent without a zone and read as UTC.
type NewsTime struct{ time.Time }

// UnmarshalJSON implements json.Unmarshaler; null and "" leave the zero time.
func (t *NewsTime) UnmarshalJSON(data []byte) error {
	var text *string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == nil || *text == "" {
		return nil
	}
	parsed, err := time.ParseInLocation(newsTimeLayout, *text, time.UTC)
	if err != nil {
		return fmt.Errorf("news time %q: %w", *text, err)
	}
	t.Time = parsed
	return nil
}

// LatestNews asks for up to limit of the newest articles about a symbol in
// finimpulse notation published between from and to, newest first.
func LatestNews(symbol string, from, to time.Time, limit int) NewsRequest {
	return NewsRequest{
		Symbol: symbol, Types: []string{NewsTypeNews},
		StartDate: from.Format(time.DateOnly), EndDate: to.Format(time.DateOnly),
		Limit: limit, SortBy: []SortBy{{Selector: "pub_date", Desc: true}},
	}
}

// News searches the news.
func (c *Client) News(ctx context.Context, request NewsRequest) (Response[NewsResult], error) {
	return post[NewsResult](ctx, c, "/v1/news", request)
}
