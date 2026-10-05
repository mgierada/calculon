package earnings

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
)

var now = time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)

// pagedClient serves total revenue items, a page at a time.
type pagedClient struct {
	total    int
	requests []finimpulse.EarningsRequest
}

func (c *pagedClient) Earnings(_ context.Context, r finimpulse.EarningsRequest) (finimpulse.Response[finimpulse.EarningsResult], error) {
	c.requests = append(c.requests, r)
	target := 1000.0 + float64(r.Offset)
	result := finimpulse.EarningsResult{TotalCount: c.total, TargetPrice: &target}
	for i := r.Offset; i < min(r.Offset+r.Limit, c.total); i++ {
		day := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 3*i, 0)
		result.Revenue = append(result.Revenue, finimpulse.EarningsRevenue{
			EarningsPeriod: finimpulse.EarningsPeriod{Date: day.Format(time.DateOnly), DateType: "quarter"},
			Methodology:    "gaap",
		})
	}
	return finimpulse.Response[finimpulse.EarningsResult]{Result: result}, nil
}

func newFetcher(t *testing.T, client Client) *Fetcher {
	t.Helper()

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	fetcher := NewFetcher(conn, client, Options{
		Types: []string{finimpulse.EarningsTypeRevenue}, Methodology: "gaap",
		Lookback: 365 * 24 * time.Hour, PageSize: 4,
	})
	fetcher.now = func() time.Time { return now }
	return fetcher
}

func TestFetchPagesUntilTotalAndStoresEverything(t *testing.T) {
	client := &pagedClient{total: 10}
	fetcher := newFetcher(t, client)
	var pages []Page

	from := time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := fetcher.Fetch(context.Background(), "EQIX.US", from, now, func(p Page) { pages = append(pages, p) })

	if err != nil || result.Pages != 3 || result.Items != 10 {
		t.Fatalf("result = %+v, %v; want 3 pages of 10 items", result, err)
	}
	if got := client.requests[2]; got.Offset != 8 || got.Symbol != "EQIX" || got.StartDate != "2016-01-01" ||
		len(got.Methodologies) != 1 {
		t.Errorf("last request = %+v", got)
	}
	if last := pages[len(pages)-1]; last.Items != 10 || last.Total != 10 {
		t.Errorf("last page = %+v", last)
	}
	e, _, _ := db.EarningsOf(fetcher.conn, "EQIX.US", "gaap")
	if len(e.Revenue) != 10 || *e.Target.Price != 1000 {
		t.Errorf("stored %d periods with target %v, want all 10 and the first page's target",
			len(e.Revenue), *e.Target.Price)
	}
}

func TestFetchRecentReachesBackTheLookback(t *testing.T) {
	client := &pagedClient{total: 0}
	fetcher := newFetcher(t, client)

	if _, err := fetcher.FetchRecent(context.Background(), "XTB.PL", func(Page) {}); err != nil {
		t.Fatalf("FetchRecent returned error: %v", err)
	}
	if got := client.requests; len(got) != 1 || got[0].StartDate != "2025-10-05" ||
		got[0].EndDate != "2026-10-05" || got[0].Symbol != "XTB.WA" {
		t.Errorf("requests = %+v, want one for the last year", got)
	}
}
