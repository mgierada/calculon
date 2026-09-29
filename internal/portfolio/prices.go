package portfolio

import (
	"sort"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// Quote is the latest known price of a symbol and, when a quote from an
// earlier day exists, the price it moved from.
type Quote struct {
	Price float64
	AsOf  time.Time
	// PrevClose is the last price observed on an earlier calendar day. It is
	// zero until such a quote exists, which leaves day-to-date change unknown.
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
			pricePoint{at: quote.AsOf, price: quote.Price})
	}
	for symbol := range book.bySymbol {
		sortSeries(book.bySymbol[symbol])
	}
	return book
}

// Latest implements PriceSource.
func (b QuoteBook) Latest(symbol string) (Quote, bool) {
	points := b.bySymbol[symbol]
	if len(points) == 0 {
		return Quote{}, false
	}
	last := points[len(points)-1]
	quote := Quote{Price: last.price, AsOf: last.at}
	lastDay := dayOf(last.at)
	for i := len(points) - 2; i >= 0; i-- {
		if dayOf(points[i].at).Before(lastDay) {
			quote.PrevClose = points[i].price
			break
		}
	}
	return quote, true
}

// priceHistory is every price observed per symbol: stored quotes plus the
// prices trades executed at. Between observations a symbol is marked at its
// last known price, which is exact on trade days and an approximation between
// them until a market data source fills in daily closes.
type priceHistory map[string]series

func newPriceHistory(in Input) priceHistory {
	history := priceHistory{}
	add := func(symbol string, at time.Time, price float64) {
		if symbol != "" && price > 0 && !at.IsZero() {
			history[symbol] = append(history[symbol], pricePoint{at: at, price: price})
		}
	}
	for _, quote := range in.Quotes {
		add(quote.Symbol, quote.AsOf, quote.Price)
	}
	for _, lot := range in.Lots {
		add(lot.Record.Symbol, lot.Record.OpenTime, lot.Record.OpenPrice)
	}
	for _, position := range in.Closed {
		p := position.Record
		add(p.Symbol, p.OpenTime, p.OpenPrice)
		add(p.Symbol, p.CloseTime, p.ClosePrice)
	}
	for _, op := range in.CashOps {
		if op.Record.MovesStock() {
			add(op.Record.Symbol, op.Record.Time, op.Record.Price)
		}
	}
	for symbol := range history {
		sortSeries(history[symbol])
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
