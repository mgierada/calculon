package portfolio

import (
	"math"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// sameCurrencyTolerance absorbs the rounding in broker values. XTB states
// purchase and sale values to the cent, so a same-currency trade can imply a
// rate like 0.9998; anything this close to 1 is treated as exactly 1, which
// keeps same-currency holdings matching the broker's own totals.
const sameCurrencyTolerance = 0.005

// instrumentKey is a symbol as held in accounts of one currency. A symbol can
// sit in accounts of different currencies, e.g. a US stock bought from both a
// PLN and a USD account, and needs a conversion rate for each.
type instrumentKey struct {
	currency string
	symbol   string
}

func keyOf(account model.Account, symbol string) instrumentKey {
	return instrumentKey{currency: account.Currency, symbol: symbol}
}

// conversions are the observed rates from an instrument's own currency into
// the currency of the account holding it. Brokers let an account hold stocks
// priced in other currencies: prices are quoted in the stock's currency while
// cash moves in the account's. Nothing states the rate directly, but every
// trade implies it: its value in account currency over volume times price.
type conversions map[instrumentKey]series

func newConversions(in Input) conversions {
	c := conversions{}
	add := func(account model.Account, symbol string, at time.Time, accountValue, volume, price float64) {
		if symbol == "" || at.IsZero() || accountValue <= 0 || volume <= 0 || price <= 0 {
			return
		}
		k := keyOf(account, symbol)
		c[k] = append(c[k], pricePoint{at: at, price: snapRate(accountValue / (volume * price))})
	}
	for _, owned := range in.Closed {
		p := owned.Record
		add(owned.Account, p.Symbol, p.OpenTime, p.PurchaseValue, p.Volume, p.OpenPrice)
		add(owned.Account, p.Symbol, p.CloseTime, p.SaleValue, p.Volume, p.ClosePrice)
	}
	for _, owned := range in.CashOps {
		op := owned.Record
		if op.MovesStock() {
			add(owned.Account, op.Symbol, op.Time, math.Abs(op.Amount), op.Volume, op.Price)
		}
	}
	for k := range c {
		sortSeries(c[k])
	}
	return c
}

// at is the rate last observed at or before t, the first one before any
// trade, and 1 for an instrument never seen converting.
func (c conversions) at(k instrumentKey, t time.Time) float64 {
	if rate, ok := c[k].at(t); ok {
		return rate
	}
	return 1
}

// latest is the most recent rate observed, 1 when there is none.
func (c conversions) latest(k instrumentKey) float64 {
	s := c[k]
	if len(s) == 0 {
		return 1
	}
	return s[len(s)-1].price
}

// snapRate treats a rate within rounding distance of 1 as the same currency.
func snapRate(rate float64) float64 {
	if math.Abs(rate-1) <= sameCurrencyTolerance {
		return 1
	}
	return rate
}
