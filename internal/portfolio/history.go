package portfolio

import (
	"sort"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// holdingSpan is a volume of a symbol held in one account between two times.
// An open lot has no end.
type holdingSpan struct {
	account model.Account
	symbol  string
	volume  float64
	from    time.Time
	until   time.Time
}

func (s holdingSpan) heldAt(t time.Time) bool {
	return !s.from.After(t) && (s.until.IsZero() || s.until.After(t))
}

// cashEvent is one ledger entry, reduced to what the history needs.
type cashEvent struct {
	at           time.Time
	amount       float64
	contribution float64
}

// valueHistory rebuilds the portfolio's worth at the end of every day from the
// first recorded event to asOf, in the base currency.
//
// Holdings come from lots rather than from the cash ledger: every closed row
// and every open lot says exactly what volume was held when, including shares
// that moved through corporate actions with no cash entry. Cash is the running
// sum of the ledger. Prices are the last observed price per symbol, see
// priceHistory. Conversion uses today's FX rates for every day.
func valueHistory(in Input, conv converter, asOf time.Time) []ValuePoint {
	spans := holdingSpans(in)
	events := cashEvents(in, conv)
	if len(spans) == 0 && len(events) == 0 {
		return nil
	}
	prices := newPriceHistory(in)

	start := firstEvent(spans, events)
	if asOf.IsZero() || asOf.Before(start) {
		asOf = start
	}

	var (
		points        []ValuePoint
		cash, contrib float64
		next          int
	)
	for day := dayOf(start); !day.After(asOf); day = day.AddDate(0, 0, 1) {
		end := day.AddDate(0, 0, 1)
		if end.After(asOf) {
			end = asOf
		}
		for ; next < len(events) && !events[next].at.After(end); next++ {
			cash += events[next].amount
			contrib += events[next].contribution
		}
		points = append(points, ValuePoint{
			Time:          day,
			Value:         cash + positionsValueAt(spans, prices, conv, end),
			Contributions: contrib,
		})
	}
	return points
}

// holdingSpans lists every non-CFD volume held, closed or open.
func holdingSpans(in Input) []holdingSpan {
	var spans []holdingSpan
	for _, owned := range in.Closed {
		p := owned.Record
		if p.Category == model.CategoryCFD {
			continue
		}
		spans = append(spans, holdingSpan{
			account: owned.Account, symbol: p.Symbol, volume: p.Volume,
			from: p.OpenTime, until: p.CloseTime,
		})
	}
	for _, owned := range in.Lots {
		lot := owned.Record
		if lot.Category == model.CategoryCFD {
			continue
		}
		spans = append(spans, holdingSpan{
			account: owned.Account, symbol: lot.Symbol, volume: lot.Volume, from: lot.OpenTime,
		})
	}
	return spans
}

// cashEvents converts the ledger to base-currency events, oldest first.
func cashEvents(in Input, conv converter) []cashEvent {
	events := make([]cashEvent, 0, len(in.CashOps))
	for _, owned := range in.CashOps {
		op := owned.Record
		event := cashEvent{at: op.Time, amount: conv.toBase(op.Amount, owned.Account.Currency)}
		if op.Kind.IsContribution() {
			event.contribution = event.amount
		}
		events = append(events, event)
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	return events
}

// positionsValueAt values every span held at t.
func positionsValueAt(spans []holdingSpan, prices priceHistory, conv converter, t time.Time) float64 {
	var total float64
	for _, span := range spans {
		if !span.heldAt(t) {
			continue
		}
		price, ok := prices[span.symbol].at(t)
		if !ok {
			continue
		}
		total += conv.toBase(span.volume*price, span.account.Currency)
	}
	return total
}

func firstEvent(spans []holdingSpan, events []cashEvent) time.Time {
	var first time.Time
	consider := func(t time.Time) {
		if !t.IsZero() && (first.IsZero() || t.Before(first)) {
			first = t
		}
	}
	for _, span := range spans {
		consider(span.from)
	}
	if len(events) > 0 {
		consider(events[0].at)
	}
	return first
}
