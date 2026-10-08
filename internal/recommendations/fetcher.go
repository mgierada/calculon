// Package recommendations fetches a symbol's analyst recommendations from
// finimpulse and stores them, page by page, for any period, so the same call
// serves the dashboard's recent refresh and a backfill.
package recommendations

import (
	"context"
	"database/sql"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/symbolmap"
)

// maxPages bounds one fetch should the API keep reporting more items.
const maxPages = 100

// Client searches recommendations.
type Client interface {
	Recommendations(ctx context.Context, request finimpulse.RecommendationsRequest) (
		finimpulse.Response[finimpulse.RecommendationsResult], error)
}

// Options shape every request: how far back a recent fetch reaches and how
// many months a page asks for.
type Options struct {
	Lookback time.Duration
	PageSize int
}

// Page reports a fetch's progress after each page.
type Page struct {
	Number int
	// Items is how many months the fetch has stored so far, of Total the API
	// reports for the period.
	Items, Total int
}

// Result totals a fetch.
type Result struct {
	Pages, Items int
}

// Fetcher fetches and stores recommendations.
type Fetcher struct {
	conn   *sql.DB
	client Client
	opts   Options
	now    func() time.Time
}

// NewFetcher fetches with opts.
func NewFetcher(conn *sql.DB, client Client, opts Options) *Fetcher {
	return &Fetcher{conn: conn, client: client, opts: opts, now: time.Now}
}

// FetchRecent fetches a symbol's recommendations over the lookback up to today.
func (f *Fetcher) FetchRecent(ctx context.Context, symbol string, progress func(Page)) (Result, error) {
	now := f.now().UTC()
	return f.Fetch(ctx, symbol, now.Add(-f.opts.Lookback), now, progress)
}

// Fetch stores a symbol's recommendations of months between from and to,
// asking page after page until the API's total is reached or a page comes
// back empty. Stored months are overwritten, so repeated fetches are harmless.
func (f *Fetcher) Fetch(ctx context.Context, symbol string, from, to time.Time,
	progress func(Page)) (Result, error) {
	providerSymbol, err := symbolmap.Lookup(f.conn, symbol)
	if err != nil {
		return Result{}, err
	}
	fetchedAt := f.now().UTC()
	var result Result
	for offset := 0; result.Pages < maxPages; offset += f.opts.PageSize {
		request := finimpulse.NewRecommendationsRequest(providerSymbol, from, to, f.opts.PageSize, offset)
		resp, err := f.client.Recommendations(ctx, request)
		if err != nil {
			return result, err
		}
		page := resp.Result
		if err := db.StoreRecommendations(f.conn, symbol, fetchedAt, page.Items); err != nil {
			return result, err
		}
		result.Pages++
		result.Items += len(page.Items)
		progress(Page{Number: result.Pages, Items: result.Items, Total: page.TotalCount})
		if len(page.Items) == 0 || offset+f.opts.PageSize >= page.TotalCount {
			break
		}
	}
	return result, nil
}
