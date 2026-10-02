// Package news fetches articles about the biggest holdings from finimpulse and
// stores them, asking each time only for what was published since the newest
// article already stored.
package news

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/symbolmap"
)

// Client searches the news.
type Client interface {
	News(ctx context.Context, request finimpulse.NewsRequest) (finimpulse.Response[finimpulse.NewsResult], error)
}

// Step reports a fetch's progress: Current is the symbol being fetched, empty
// once all are done; Finished is the one just completed, if any.
type Step struct {
	Done, Total int
	Current     string
	Finished    *SymbolResult
}

// SymbolResult is how fetching one symbol went.
type SymbolResult struct {
	Symbol string
	Added  int
	Err    error
}

// Result totals a fetch.
type Result struct {
	Added  int
	Failed []string
}

// Fetcher fetches and stores news.
type Fetcher struct {
	conn      *sql.DB
	client    Client
	perSymbol int
	lookback  time.Duration
	now       func() time.Time
}

// NewFetcher asks for up to perSymbol articles per symbol, reaching lookback
// into the past for a symbol without stored news.
func NewFetcher(conn *sql.DB, client Client, perSymbol int, lookback time.Duration) *Fetcher {
	return &Fetcher{conn: conn, client: client, perSymbol: perSymbol, lookback: lookback, now: time.Now}
}

// Fetch gets new articles about each symbol in turn, reporting progress before
// and after each. One symbol failing does not stop the rest; a cancelled ctx
// does, and is returned.
func (f *Fetcher) Fetch(ctx context.Context, symbols []string, progress func(Step)) (Result, error) {
	var result Result
	total := len(symbols)
	for i, symbol := range symbols {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		progress(Step{Done: i, Total: total, Current: symbol})
		added, err := f.fetchSymbol(ctx, symbol)
		if err != nil {
			result.Failed = append(result.Failed, symbol)
		}
		result.Added += added
		finished := &SymbolResult{Symbol: symbol, Added: added, Err: err}
		progress(Step{Done: i + 1, Total: total, Finished: finished})
	}
	return result, ctx.Err()
}

// fetchSymbol stores the articles about symbol published since the newest
// one stored, or within the lookback when none is.
func (f *Fetcher) fetchSymbol(ctx context.Context, symbol string) (int, error) {
	providerSymbol, err := f.providerSymbol(symbol)
	if err != nil {
		return 0, err
	}
	now := f.now().UTC()
	from := now.Add(-f.lookback)
	latest, err := db.LatestNewsTime(f.conn, symbol)
	if err != nil {
		return 0, err
	}
	if latest.After(from) {
		from = latest
	}
	// The end date may be exclusive, so it reaches into tomorrow to keep
	// today's articles in.
	request := finimpulse.LatestNews(providerSymbol, from, now.AddDate(0, 0, 1), f.perSymbol)
	resp, err := f.client.News(ctx, request)
	if err != nil {
		return 0, err
	}
	return db.StoreNews(f.conn, symbol, now, resp.Result.Items)
}

// providerSymbol names symbol the way finimpulse does: the stored mapping, so
// a hand fix applies, or the suffix rules for a symbol not polled yet.
func (f *Fetcher) providerSymbol(symbol string) (string, error) {
	stored, ok, err := db.ProviderSymbol(f.conn, symbol, symbolmap.Finimpulse)
	if err != nil || ok {
		return stored, err
	}
	if mapped, ok := symbolmap.ToFinimpulse(symbol); ok {
		return mapped, nil
	}
	return "", fmt.Errorf("no finimpulse symbol for %s", symbol)
}
