package recommendations

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
)

var now = time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)

// pagedClient serves total months, a page at a time, newest first.
type pagedClient struct {
	total    int
	requests []finimpulse.RecommendationsRequest
}

func (c *pagedClient) Recommendations(_ context.Context, r finimpulse.RecommendationsRequest) (
	finimpulse.Response[finimpulse.RecommendationsResult], error) {
	c.requests = append(c.requests, r)
	result := finimpulse.RecommendationsResult{TotalCount: c.total}
	for i := r.Offset; i < min(r.Offset+r.Limit, c.total); i++ {
		month := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).AddDate(0, -i, 0)
		result.Items = append(result.Items, finimpulse.Recommendation{
			Date: month.Format(time.DateOnly), Buy: i + 1,
		})
	}
	return finimpulse.Response[finimpulse.RecommendationsResult]{Result: result}, nil
}

func newFetcher(t *testing.T, client Client) *Fetcher {
	t.Helper()

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	fetcher := NewFetcher(conn, client, Options{Lookback: 365 * 24 * time.Hour, PageSize: 4})
	fetcher.now = func() time.Time { return now }
	return fetcher
}

func TestFetchPagesUntilTotalAndStoresEverything(t *testing.T) {
	client := &pagedClient{total: 10}
	fetcher := newFetcher(t, client)
	var pages []Page

	from := time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := fetcher.Fetch(context.Background(), "XTB.PL", from, now,
		func(p Page) { pages = append(pages, p) })

	if err != nil || result.Pages != 3 || result.Items != 10 {
		t.Fatalf("result = %+v, %v; want 3 pages of 10 months", result, err)
	}
	if got := client.requests[2]; got.Offset != 8 || got.Symbol != "XTB.WA" || got.StartDate != "2016-01-01" {
		t.Errorf("last request = %+v", got)
	}
	if last := pages[len(pages)-1]; last.Items != 10 || last.Total != 10 {
		t.Errorf("last page = %+v", last)
	}
	recs, _, _ := db.RecommendationsOf(fetcher.conn, "XTB.PL")
	if len(recs.Months) != 10 || !recs.FetchedAt.Equal(now) || recs.Months[9].Buy != 1 {
		t.Errorf("stored %+v, want all 10 months, newest last", recs)
	}
}

func TestFetchRecentReachesBackTheLookback(t *testing.T) {
	client := &pagedClient{total: 0}
	fetcher := newFetcher(t, client)

	if _, err := fetcher.FetchRecent(context.Background(), "XTB.PL", func(Page) {}); err != nil {
		t.Fatalf("FetchRecent returned error: %v", err)
	}
	if got := client.requests; len(got) != 1 || got[0].StartDate != "2025-10-08" ||
		got[0].EndDate != "2026-10-08" {
		t.Errorf("requests = %+v, want one for the last year", got)
	}
}
