// Package marketdata keeps the prices of held symbols current: it polls the
// market data API, stores every response and tells subscribers when prices
// change so dashboards can reload.
package marketdata

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/symbolmap"
)

// quoteSource names the quotes taken from finimpulse.
const quoteSource = "finimpulse"

// subUnitCurrencies quote prices in hundredths of a currency, e.g. pence on
// the LSE.
// TODO: confirm which unit XTB values these instruments in and convert rather
// than skip.
var subUnitCurrencies = map[string]bool{"GBp": true, "GBX": true, "ZAc": true, "ILA": true}

// Fetcher gets the current market price of a symbol in the provider's notation.
type Fetcher interface {
	MarketPrice(ctx context.Context, symbol string) (finimpulse.Response[finimpulse.MarketPrice], error)
}

// Update reports one poll that fetched at least one symbol.
type Update struct {
	At      time.Time
	Fetched int
	// Failed lists our symbols whose fetch or storage failed.
	Failed []string
}

// Poller fetches every held symbol once its stored price is older than the
// interval while its exchange trades, plus once after the close for the closing
// price. A restart soon after a poll, or outside trading hours once the close
// is stored, calls the API for nothing.
type Poller struct {
	conn     *sql.DB
	fetcher  Fetcher
	interval time.Duration
	logf     func(format string, args ...any)
	now      func() time.Time

	mu   sync.Mutex
	subs map[chan Update]struct{}
}

// NewPoller polls fetcher every interval. logf receives failures; nil drops
// them, for the local UI where logging would draw over the dashboards.
func NewPoller(conn *sql.DB, fetcher Fetcher, interval time.Duration,
	logf func(format string, args ...any)) *Poller {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &Poller{
		conn: conn, fetcher: fetcher, interval: interval, logf: logf, now: time.Now,
		subs: map[chan Update]struct{}{},
	}
}

// Subscribe returns a channel of updates and a function that stops them and
// closes it. A slow subscriber only ever misses older updates.
func (p *Poller) Subscribe() (<-chan Update, func()) {
	ch := make(chan Update, 1)
	p.mu.Lock()
	p.subs[ch] = struct{}{}
	p.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			p.mu.Lock()
			delete(p.subs, ch)
			p.mu.Unlock()
			close(ch)
		})
	}
}

// Run polls until ctx is cancelled: right away for whatever is stale, then
// whenever the next symbol falls due.
func (p *Poller) Run(ctx context.Context) {
	for {
		next := p.poll(ctx)
		timer := time.NewTimer(next.Sub(p.now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

// poll fetches every due target and returns when the next one falls due, at
// most an interval away so newly imported symbols are picked up. A failed
// fetch waits that interval rather than retrying in a hot loop.
func (p *Poller) poll(ctx context.Context) time.Time {
	now := p.now()
	next := now.Add(p.interval)

	targets, err := p.targets()
	if err != nil {
		p.logf("market data: %v", err)
		return next
	}

	update := Update{At: now}
	for _, target := range targets {
		if due := dueAt(target.Symbol, target.LastFetched, p.interval); due.After(now) {
			next = earliest(next, due)
			continue
		}
		if ctx.Err() != nil {
			return next
		}
		if err := p.fetch(ctx, target, now); err != nil {
			p.logf("market data: %s: %v", target.Symbol, err)
			update.Failed = append(update.Failed, target.Symbol)
			continue
		}
		update.Fetched++
	}
	if update.Fetched > 0 || len(update.Failed) > 0 {
		p.broadcast(update)
	}
	return next
}

// targets maps any newly held symbols, then lists every one to price.
func (p *Poller) targets() ([]db.PriceTarget, error) {
	if _, err := symbolmap.Sync(p.conn); err != nil {
		return nil, err
	}
	return db.PriceTargets(p.conn, symbolmap.Finimpulse)
}

// fetch gets one target's price and stores it with the quote it yields.
func (p *Poller) fetch(ctx context.Context, target db.PriceTarget, fetchedAt time.Time) error {
	resp, err := p.fetcher.MarketPrice(ctx, target.ProviderSymbol)
	if err != nil {
		return err
	}
	price := db.IntradayPrice{Symbol: target.Symbol, FetchedAt: fetchedAt, Response: resp}
	return db.StoreIntradayPrice(p.conn, price, quoteOf(target.Symbol, fetchedAt, resp.Result))
}

// quoteOf is the price dashboards value a holding at, or nil when the response
// has none usable.
func quoteOf(symbol string, fetchedAt time.Time, price finimpulse.MarketPrice) *model.Quote {
	if price.CurrentPrice == nil || *price.CurrentPrice <= 0 || subUnitCurrencies[price.Currency] {
		return nil
	}
	asOf := fetchedAt
	if price.CurrentPriceUpdateTime != nil {
		asOf = *price.CurrentPriceUpdateTime
	}
	return &model.Quote{Symbol: symbol, AsOf: asOf, Price: *price.CurrentPrice, Source: quoteSource}
}

// broadcast hands update to every subscriber, replacing one they have not
// taken yet.
func (p *Poller) broadcast(update Update) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for ch := range p.subs {
		select {
		case <-ch:
		default:
		}
		ch <- update
	}
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
