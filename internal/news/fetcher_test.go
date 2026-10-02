package news

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
)

var now = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)

// fakeClient answers from items, keyed by provider symbol, recording requests.
type fakeClient struct {
	items    map[string][]finimpulse.NewsItem
	requests []finimpulse.NewsRequest
}

func (f *fakeClient) News(_ context.Context, request finimpulse.NewsRequest) (finimpulse.Response[finimpulse.NewsResult], error) {
	f.requests = append(f.requests, request)
	items, ok := f.items[request.Symbol]
	if !ok {
		return finimpulse.Response[finimpulse.NewsResult]{}, errors.New("unknown symbol")
	}
	return finimpulse.Response[finimpulse.NewsResult]{Result: finimpulse.NewsResult{Items: items}}, nil
}

func article(id string, published time.Time) finimpulse.NewsItem {
	return finimpulse.NewsItem{ID: id, Title: id, PubDate: finimpulse.NewsTime{Time: published}}
}

func newFetcher(t *testing.T, client Client) *Fetcher {
	t.Helper()

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	fetcher := NewFetcher(conn, client, 5, 30*24*time.Hour)
	fetcher.now = func() time.Time { return now }
	return fetcher
}

func TestFetchStoresAndReportsEverySymbol(t *testing.T) {
	client := &fakeClient{items: map[string][]finimpulse.NewsItem{
		"XTB.WA": {article("a", now.Add(-time.Hour)), article("b", now.Add(-2*time.Hour))},
	}}
	fetcher := newFetcher(t, client)
	var steps []Step

	result, err := fetcher.Fetch(context.Background(), []string{"XTB.PL", "AMT.US"},
		func(s Step) { steps = append(steps, s) })

	if err != nil || result.Added != 2 || len(result.Failed) != 1 || result.Failed[0] != "AMT.US" {
		t.Errorf("result = %+v, %v; want 2 added and AMT.US failed", result, err)
	}
	if len(steps) != 4 || steps[0].Current != "XTB.PL" || steps[1].Finished.Added != 2 ||
		steps[3].Done != 2 || steps[3].Finished.Err == nil {
		t.Errorf("steps = %+v", steps)
	}
	if got := client.requests[0]; got.StartDate != "2026-09-02" || got.EndDate != "2026-10-03" || got.Limit != 5 {
		t.Errorf("first request = %+v, want the lookback up to tomorrow", got)
	}
	if client.requests[1].Symbol != "AMT" {
		t.Errorf("second request for %s, want the finimpulse name AMT", client.requests[1].Symbol)
	}
}

// A second fetch only asks for what was published since the newest stored
// article, so stored ones are not requested again.
func TestFetchStartsFromNewestStoredArticle(t *testing.T) {
	client := &fakeClient{items: map[string][]finimpulse.NewsItem{
		"XTB.WA": {article("a", time.Date(2026, 9, 28, 15, 0, 0, 0, time.UTC))},
	}}
	fetcher := newFetcher(t, client)
	fetcher.Fetch(context.Background(), []string{"XTB.PL"}, func(Step) {})

	result, _ := fetcher.Fetch(context.Background(), []string{"XTB.PL"}, func(Step) {})

	if got := client.requests[1].StartDate; got != "2026-09-28" {
		t.Errorf("second request starts %s, want the newest stored article's day", got)
	}
	if result.Added != 0 {
		t.Errorf("added %d on refetch, want the stored article skipped", result.Added)
	}
}

func TestFetchStopsWhenCancelled(t *testing.T) {
	client := &fakeClient{items: map[string][]finimpulse.NewsItem{"XTB.WA": nil}}
	fetcher := newFetcher(t, client)
	ctx, cancel := context.WithCancel(context.Background())

	_, err := fetcher.Fetch(ctx, []string{"XTB.PL", "SNT.PL"}, func(s Step) {
		if s.Finished != nil {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) || len(client.requests) != 1 {
		t.Errorf("err = %v after %d requests, want cancelled after the first", err, len(client.requests))
	}
}
