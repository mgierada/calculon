package marketdata

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
)

const interval = 15 * time.Minute

var start = time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)

// fakeFetcher answers from prices, keyed by provider symbol, and records calls.
type fakeFetcher struct {
	prices   map[string]finimpulse.MarketPrice
	requests []string
}

func (f *fakeFetcher) MarketPrice(_ context.Context, symbol string) (finimpulse.Response[finimpulse.MarketPrice], error) {
	f.requests = append(f.requests, symbol)
	price, ok := f.prices[symbol]
	if !ok {
		return finimpulse.Response[finimpulse.MarketPrice]{}, errors.New("unknown symbol")
	}
	return finimpulse.Response[finimpulse.MarketPrice]{
		Meta:   finimpulse.Meta{TaskID: "task", StatusCode: 20000, StatusMessage: "OK"},
		Result: price,
	}, nil
}

func marketPrice(symbol, currency string, price float64) finimpulse.MarketPrice {
	return finimpulse.MarketPrice{Symbol: symbol, Currency: currency, CurrentPrice: &price}
}

// setup stores a user holding symbols and a poller over it whose clock reads
// *now.
func setup(t *testing.T, fetcher *fakeFetcher, symbols ...string) (*Poller, *time.Time) {
	t.Helper()

	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	user, err := db.CreateUser(conn, "alice")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	statement := model.Statement{
		Account: model.Account{Provider: model.ProviderXTB, ID: "1", Currency: "PLN"},
	}
	for i, symbol := range symbols {
		statement.OpenLots = append(statement.OpenLots, model.OpenLot{
			Instrument: model.Instrument{Symbol: symbol, Name: symbol, Category: "STOCK"},
			PositionID: string(rune('a' + i)), Side: model.SideBuy, Volume: 1,
			OpenTime: start.Add(-24 * time.Hour), OpenPrice: 10,
		})
	}
	if _, err := db.Import(conn, user.ID, statement); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	now := start
	poller := NewPoller(conn, fetcher, interval, nil)
	poller.now = func() time.Time { return now }
	return poller, &now
}

func TestPollFetchesHeldSymbolsAndNotifies(t *testing.T) {
	fetcher := &fakeFetcher{prices: map[string]finimpulse.MarketPrice{
		"XTB.WA": marketPrice("XTB.WA", "PLN", 150.56),
		"AMT":    marketPrice("AMT", "USD", 170),
	}}
	poller, _ := setup(t, fetcher, "XTB.PL", "AMT.US", "US500")
	updates, cancel := poller.Subscribe()
	defer cancel()

	next := poller.poll(context.Background())

	if !slices.Equal(fetcher.requests, []string{"AMT", "XTB.WA"}) {
		t.Errorf("requests = %v, want mapped held symbols only", fetcher.requests)
	}
	if !next.Equal(start.Add(interval)) {
		t.Errorf("next = %v, want one interval on", next)
	}
	select {
	case update := <-updates:
		if update.Fetched != 2 || len(update.Failed) != 0 {
			t.Errorf("update = %+v", update)
		}
	default:
		t.Fatal("no update sent")
	}

	quotes, err := db.Quotes(poller.conn, 1)
	if err != nil {
		t.Fatalf("Quotes returned error: %v", err)
	}
	var fromAPI []model.Quote
	for _, q := range quotes {
		if q.Source == quoteSource {
			fromAPI = append(fromAPI, q)
		}
	}
	if len(fromAPI) != 2 || fromAPI[0].Symbol != "AMT.US" || fromAPI[1].Price != 150.56 {
		t.Errorf("quotes = %+v, want one per fetched symbol under our names", fromAPI)
	}
}

// A restart within the interval of the last poll must not call the API, and
// must wake when the stored prices go stale rather than a full interval later.
func TestPollSkipsFreshPricesAndWakesWhenStale(t *testing.T) {
	fetcher := &fakeFetcher{prices: map[string]finimpulse.MarketPrice{
		"XTB.WA": marketPrice("XTB.WA", "PLN", 150.56),
	}}
	poller, now := setup(t, fetcher, "XTB.PL")
	poller.poll(context.Background())

	*now = start.Add(10 * time.Minute)
	fetcher.requests = nil
	next := poller.poll(context.Background())

	if len(fetcher.requests) != 0 {
		t.Errorf("requests = %v, want none while fresh", fetcher.requests)
	}
	if !next.Equal(start.Add(interval)) {
		t.Errorf("next = %v, want when the stored price goes stale", next)
	}

	*now = start.Add(interval)
	poller.poll(context.Background())
	if len(fetcher.requests) != 1 {
		t.Errorf("requests = %v, want a fetch once stale", fetcher.requests)
	}
}

func TestPollReportsFailuresAndKeepsGoing(t *testing.T) {
	fetcher := &fakeFetcher{prices: map[string]finimpulse.MarketPrice{
		"XTB.WA": marketPrice("XTB.WA", "PLN", 150.56),
	}}
	poller, _ := setup(t, fetcher, "AMT.US", "XTB.PL")
	updates, cancel := poller.Subscribe()
	defer cancel()

	next := poller.poll(context.Background())

	update := <-updates
	if update.Fetched != 1 || !slices.Equal(update.Failed, []string{"AMT.US"}) {
		t.Errorf("update = %+v", update)
	}
	if !next.Equal(start.Add(interval)) {
		t.Errorf("next = %v, want a failure retried after a full interval", next)
	}
}

func TestQuoteOfSkipsUnusablePrices(t *testing.T) {
	updated := start.Add(-time.Minute)
	withTime := marketPrice("XTB.WA", "PLN", 150)
	withTime.CurrentPriceUpdateTime = &updated

	if q := quoteOf("XTB.PL", start, withTime); q == nil || !q.AsOf.Equal(updated) {
		t.Errorf("quote = %+v, want one as of the price's update time", q)
	}
	if q := quoteOf("XTB.PL", start, marketPrice("XTB.WA", "PLN", 150)); q == nil || !q.AsOf.Equal(start) {
		t.Errorf("quote = %+v, want one as of the fetch without an update time", q)
	}
	if q := quoteOf("MXFS.UK", start, marketPrice("MXFS.L", "GBp", 8525)); q != nil {
		t.Errorf("quote = %+v, want none for a pence price", q)
	}
	if q := quoteOf("X.PL", start, finimpulse.MarketPrice{}); q != nil {
		t.Errorf("quote = %+v, want none without a price", q)
	}
}

func TestSubscribeCancelClosesChannel(t *testing.T) {
	poller := NewPoller(nil, nil, interval, nil)
	updates, cancel := poller.Subscribe()
	cancel()
	cancel()

	if _, ok := <-updates; ok {
		t.Error("channel still open after cancel")
	}
	poller.broadcast(Update{})
}
