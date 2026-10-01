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

// printed is when the exchange printed every test price.
var printed = start.Add(-5 * time.Minute)

// marketPrice is a regular session price printed at printed.
func marketPrice(symbol, currency string, price float64) finimpulse.MarketPrice {
	printedAt := printed
	return finimpulse.MarketPrice{
		Symbol: symbol, Currency: currency, CurrentPrice: &price,
		RegularMarketPrice: &price, RegularMarketTime: &printedAt,
	}
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

func TestQuoteOfUsesRegularSessionPrice(t *testing.T) {
	if q := quoteOf("XTB.PL", marketPrice("XTB.WA", "PLN", 150)); q == nil ||
		!q.AsOf.Equal(printed) || q.Price != 150 {
		t.Errorf("quote = %+v, want 150 as of when it was printed", q)
	}

	// Regression: before the US open current_price is the pre-market price,
	// which measured against the regular previous close spans two sessions.
	preMarket := marketPrice("AMZN", "USD", 249.15)
	current := 251.81
	preMarket.CurrentPrice = &current
	if q := quoteOf("AMZN.US", preMarket); q == nil || q.Price != 249.15 {
		t.Errorf("quote = %+v, want the regular close 249.15, not the pre-market price", q)
	}
}

func TestQuoteOfSkipsUnusablePrices(t *testing.T) {
	noTime := marketPrice("XTB.WA", "PLN", 150)
	noTime.RegularMarketTime = nil
	cases := map[string]finimpulse.MarketPrice{
		"no print time": noTime,
		"pence":         marketPrice("MXFS.L", "GBp", 8525),
		"no price":      {},
	}
	for name, price := range cases {
		if q := quoteOf("X.PL", price); q != nil {
			t.Errorf("%s: quote = %+v, want none", name, q)
		}
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

func TestQuoteOfCarriesPreviousClose(t *testing.T) {
	price := marketPrice("XTB.WA", "PLN", 150.56)
	prev := 151.7
	price.RegularMarketPreviousClose = &prev

	if q := quoteOf("XTB.PL", price); q == nil || q.PrevClose != 151.7 {
		t.Errorf("quote = %+v, want the reported previous close", q)
	}
}

func TestCloseOfFilesUnderExchangeDay(t *testing.T) {
	price := marketPrice("XTB.WA", "PLN", 150.56)
	regular := 150.4
	// 22:30 UTC is already the next day in Warsaw.
	late := time.Date(2026, 9, 30, 22, 30, 0, 0, time.UTC)
	price.RegularMarketPrice, price.RegularMarketTime = &regular, &late

	c := closeOf("XTB.PL", start, price)
	if c == nil || c.Session.Format(time.DateOnly) != "2026-10-01" || c.Close != 150.4 {
		t.Errorf("close = %+v, want 150.4 on 2026-10-01", c)
	}
	current := 150.0
	if closeOf("XTB.PL", start, finimpulse.MarketPrice{CurrentPrice: &current}) != nil {
		t.Error("close made from a response without a regular market price")
	}
}

func TestPollStoresSessionClose(t *testing.T) {
	price := marketPrice("XTB.WA", "PLN", 150.56)
	poller, _ := setup(t, &fakeFetcher{prices: map[string]finimpulse.MarketPrice{"XTB.WA": price}}, "XTB.PL")

	poller.poll(context.Background())

	var date string
	var close float64
	if err := poller.conn.QueryRow(`SELECT session_date, close FROM eod_price WHERE symbol = 'XTB.PL'`).
		Scan(&date, &close); err != nil {
		t.Fatalf("no close stored: %v", err)
	}
	if date != "2026-09-30" || close != 150.56 {
		t.Errorf("close = %s %v, want 2026-09-30 150.56", date, close)
	}
}
