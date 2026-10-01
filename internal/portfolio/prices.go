package portfolio

import (
	"math"
	"sort"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// Quote is the latest known price of a symbol and, when a quote from an
// earlier day exists, the price it moved from.
type Quote struct {
	Price float64
	AsOf  time.Time
	// PrevClose is the previous session's close the latest quote's source
	// reported, or else the last price observed on an earlier calendar day. It
	// is zero when neither exists, which leaves day-to-date change unknown.
	PrevClose float64
}

// HasPrevClose reports whether day-to-date change can be computed.
func (q Quote) HasPrevClose() bool {
	return q.PrevClose > 0
}

// PriceSource answers what a symbol is worth now. Today that is the quotes
// stored from statement snapshots; a market data source plugs in here, or
// simply stores its quotes and lets QuoteBook read them.
type PriceSource interface {
	Latest(symbol string) (Quote, bool)
}

// pricePoint is one observed price.
type pricePoint struct {
	at    time.Time
	price float64
	// prevClose is the previous close the quote's source reported, zero when
	// unknown or when the point is not a quote.
	prevClose float64
}

// series is a symbol's observed prices, oldest first.
type series []pricePoint

// at returns the last price observed at or before t, or the first price when t
// precedes every observation.
func (s series) at(t time.Time) (float64, bool) {
	if len(s) == 0 {
		return 0, false
	}
	i := sort.Search(len(s), func(i int) bool { return s[i].at.After(t) })
	if i == 0 {
		return s[0].price, true
	}
	return s[i-1].price, true
}

// QuoteBook is a PriceSource over stored quotes.
type QuoteBook struct {
	bySymbol map[string]series
}

// NewQuoteBook indexes quotes by symbol.
func NewQuoteBook(quotes []model.Quote) QuoteBook {
	book := QuoteBook{bySymbol: map[string]series{}}
	for _, quote := range quotes {
		book.bySymbol[quote.Symbol] = append(book.bySymbol[quote.Symbol],
			pricePoint{at: quote.AsOf, price: quote.Price, prevClose: quote.PrevClose})
	}
	for symbol := range book.bySymbol {
		sortSeries(book.bySymbol[symbol])
	}
	return book
}

// Latest implements PriceSource. Day change is measured from the previous close
// the latest quote's source reported; quotes without one, like statement
// snapshots, fall back to the last price of an earlier day.
func (b QuoteBook) Latest(symbol string) (Quote, bool) {
	points := b.bySymbol[symbol]
	if len(points) == 0 {
		return Quote{}, false
	}
	last := points[len(points)-1]
	quote := Quote{Price: last.price, AsOf: last.at}
	if last.prevClose > 0 {
		quote.PrevClose = last.prevClose
		return quote, true
	}
	lastDay := dayOf(last.at)
	for i := len(points) - 2; i >= 0; i-- {
		if dayOf(points[i].at).Before(lastDay) {
			quote.PrevClose = points[i].price
			break
		}
	}
	return quote, true
}

// priceHistory is every price observed per instrument, in the currency of the
// account holding it: stored quotes plus the prices trades executed at.
// Between observations an instrument is marked at its last known price, which
// is exact on trade days and an approximation between them until a market
// data source fills in daily closes.
type priceHistory map[instrumentKey]series

func newPriceHistory(in Input, rates conversions) priceHistory {
	history := priceHistory{}
	add := func(k instrumentKey, at time.Time, price float64) {
		if k.symbol != "" && price > 0 && !at.IsZero() {
			history[k] = append(history[k], pricePoint{at: at, price: price})
		}
	}
	// convert turns a price in the instrument's currency into the account's.
	convert := func(k instrumentKey, at time.Time, price float64) {
		add(k, at, price*rates.at(k, at))
	}

	held := map[string][]instrumentKey{}
	remember := func(k instrumentKey) {
		for _, known := range held[k.symbol] {
			if known == k {
				return
			}
		}
		held[k.symbol] = append(held[k.symbol], k)
	}
	for _, owned := range in.Lots {
		lot := owned.Record
		k := keyOf(owned.Account, lot.Symbol)
		remember(k)
		convert(k, lot.OpenTime, lot.OpenPrice)
	}
	for _, owned := range in.Closed {
		p := owned.Record
		k := keyOf(owned.Account, p.Symbol)
		remember(k)
		convert(k, p.OpenTime, p.OpenPrice)
		convert(k, p.CloseTime, p.ClosePrice)
	}
	for _, owned := range in.CashOps {
		op := owned.Record
		if op.MovesStock() && op.Volume > 0 {
			// The amount is already in the account's currency.
			add(keyOf(owned.Account, op.Symbol), op.Time, math.Abs(op.Amount)/op.Volume)
		}
	}
	// Quotes are in the instrument's currency and carry no account, so they
	// apply to every account currency the symbol is held in.
	for _, quote := range in.Quotes {
		for _, k := range held[quote.Symbol] {
			convert(k, quote.AsOf, quote.Price)
		}
	}
	for k := range history {
		sortSeries(history[k])
	}
	return history
}

func sortSeries(s series) {
	sort.SliceStable(s, func(i, j int) bool { return s[i].at.Before(s[j].at) })
}

// dayOf truncates a time to its UTC calendar day.
func dayOf(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
