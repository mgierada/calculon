// Package portfolio turns a user's stored records into the numbers dashboards
// show: holdings valued at the latest known prices, totals converted into a
// base currency, allocation, and value over time.
//
// Build is a pure function of its Input, so every figure is testable without a
// database, and a new data source (live quotes, FX rates) only has to feed
// Options or store quotes; the dashboards do not change.
package portfolio

import (
	"fmt"
	"sort"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// Input is everything stored for one user.
type Input struct {
	User     string
	Accounts []model.AccountSnapshot
	Lots     []model.Owned[model.OpenLot]
	Closed   []model.Owned[model.Position]
	CashOps  []model.Owned[model.CashOp]
	Quotes   []model.Quote
}

// Options are the pluggable sources Build values the portfolio with.
type Options struct {
	FX FX
	// Prices overrides the stored quotes as the source of latest prices.
	Prices PriceSource
}

// Delta is a change that may not be computable yet, e.g. day-to-date change
// before any previous close is known.
type Delta struct {
	Amount float64
	Pct    float64
	Known  bool
}

// Holding is the open exposure to one symbol in one account, in the account's
// currency, plus its value in the base currency.
type Holding struct {
	Account model.Account
	model.Instrument
	Volume       float64
	AvgOpenPrice float64
	CostBasis    float64
	Price        float64
	PriceAsOf    time.Time
	Value        float64
	PL           Delta
	Day          Delta
	ValueBase    float64
	// Weight is the holding's share of the whole portfolio, cash included.
	Weight    float64
	FirstOpen time.Time
	Lots      []model.OpenLot
}

// AccountSummary totals one account in its own currency.
type AccountSummary struct {
	model.AccountSnapshot
	Cash           float64
	PositionsValue float64
	Total          float64
	CashBase       float64
	TotalBase      float64
}

// Totals are portfolio-wide figures in the base currency.
type Totals struct {
	PositionsValue float64
	Cash           float64
	Total          float64
	CostBasis      float64
	Unrealized     Delta
	Day            Delta
	RealizedPL     float64
	Dividends      float64
	Contributions  float64
}

// ValuePoint is the portfolio's worth at the end of one day, in the base
// currency, next to the money contributed up to then.
type ValuePoint struct {
	Time          time.Time
	Value         float64
	Contributions float64
}

// Report is what the dashboards render.
type Report struct {
	User string
	// Scope is the account the report covers, nil for the summary of all of
	// them. Available lists every account the user has, whatever the scope.
	Scope     *model.Account
	Available []model.AccountSnapshot
	Base      string
	FXNote    string
	AsOf      time.Time
	Accounts  []AccountSummary
	Holdings  []Holding
	Totals    Totals
	History   []ValuePoint
	Closed    []model.Owned[model.Position]
	CashOps   []model.Owned[model.CashOp]
	// Warnings are data problems worth surfacing, e.g. a missing FX rate.
	Warnings []string
}

// Build values the input.
func Build(in Input, opts Options) Report {
	if opts.Prices == nil {
		opts.Prices = NewQuoteBook(in.Quotes)
	}
	conv := converter{fx: opts.FX, missing: map[string]bool{}}

	report := Report{
		User:      in.User,
		Available: in.Accounts,
		Base:      opts.FX.Base(),
		FXNote:    opts.FX.Describe(),
		AsOf:      latestSnapshot(in.Accounts),
		Closed:    in.Closed,
		CashOps:   in.CashOps,
	}
	report.Holdings = buildHoldings(in.Lots, opts.Prices, conv)
	report.Accounts = summarizeAccounts(in.Accounts, report.Holdings, in.CashOps, conv)
	report.Totals = totals(report, in, conv)
	assignWeights(report.Holdings, report.Totals.Total)
	report.History = valueHistory(in, conv, report.AsOf)
	report.Warnings = conv.warnings()
	return report
}

// buildHoldings groups open lots by account and symbol, valued at the latest
// price. CFD lots are left out: their notional is not owned, and their profit
// already reaches the cash ledger when they close.
func buildHoldings(lots []model.Owned[model.OpenLot], prices PriceSource, conv converter) []Holding {
	type key struct{ provider, account, symbol string }
	index := map[key]int{}
	var holdings []Holding
	for _, owned := range lots {
		lot := owned.Record
		if lot.Category == model.CategoryCFD {
			continue
		}
		k := key{string(owned.Account.Provider), owned.Account.ID, lot.Symbol}
		i, ok := index[k]
		if !ok {
			i = len(holdings)
			index[k] = i
			holdings = append(holdings, Holding{
				Account: owned.Account, Instrument: lot.Instrument, FirstOpen: lot.OpenTime,
			})
		}
		h := &holdings[i]
		h.Volume += lot.Volume
		h.CostBasis += lot.CostBasis()
		h.Lots = append(h.Lots, lot)
		if lot.OpenTime.Before(h.FirstOpen) {
			h.FirstOpen = lot.OpenTime
		}
		if lot.CurrentPrice > 0 && h.Price == 0 {
			h.Price = lot.CurrentPrice
		}
	}

	for i := range holdings {
		valueHolding(&holdings[i], prices, conv)
	}
	sort.SliceStable(holdings, func(i, j int) bool {
		return holdings[i].ValueBase > holdings[j].ValueBase
	})
	return holdings
}

// valueHolding prices a holding and derives its P/L and day change.
func valueHolding(h *Holding, prices PriceSource, conv converter) {
	if quote, ok := prices.Latest(h.Symbol); ok {
		h.Price, h.PriceAsOf = quote.Price, quote.AsOf
		if quote.HasPrevClose() {
			change := (quote.Price - quote.PrevClose) * h.Volume
			h.Day = Delta{Amount: change, Pct: pct(change, quote.PrevClose*h.Volume), Known: true}
		}
	}
	if h.Volume > 0 {
		h.AvgOpenPrice = h.CostBasis / h.Volume
	}
	h.Value = h.Volume * h.Price
	h.PL = Delta{Amount: h.Value - h.CostBasis, Pct: pct(h.Value-h.CostBasis, h.CostBasis), Known: true}
	h.ValueBase = conv.toBase(h.Value, h.Account.Currency)
}

// summarizeAccounts totals cash and positions per account.
func summarizeAccounts(accounts []model.AccountSnapshot, holdings []Holding,
	cashOps []model.Owned[model.CashOp], conv converter) []AccountSummary {
	summaries := make([]AccountSummary, len(accounts))
	index := map[model.AccountKey]int{}
	for i, account := range accounts {
		summaries[i] = AccountSummary{AccountSnapshot: account}
		index[account.Key()] = i
	}
	for _, op := range cashOps {
		if i, ok := index[op.Account.Key()]; ok {
			summaries[i].Cash += op.Record.Amount
		}
	}
	for _, h := range holdings {
		if i, ok := index[h.Account.Key()]; ok {
			summaries[i].PositionsValue += h.Value
		}
	}
	for i := range summaries {
		s := &summaries[i]
		s.Total = s.Cash + s.PositionsValue
		s.CashBase = conv.toBase(s.Cash, s.Currency)
		s.TotalBase = conv.toBase(s.Total, s.Currency)
	}
	return summaries
}

// totals rolls accounts, holdings and history up into base-currency figures.
func totals(report Report, in Input, conv converter) Totals {
	var t Totals
	dayBase, dayPrev, dayKnown := 0.0, 0.0, false
	for _, h := range report.Holdings {
		t.PositionsValue += h.ValueBase
		t.CostBasis += conv.toBase(h.CostBasis, h.Account.Currency)
		if h.Day.Known {
			change := conv.toBase(h.Day.Amount, h.Account.Currency)
			dayBase += change
			dayPrev += h.ValueBase - change
			dayKnown = true
		}
	}
	for _, account := range report.Accounts {
		t.Cash += account.CashBase
	}
	for _, p := range in.Closed {
		t.RealizedPL += conv.toBase(p.Record.NetPL, p.Account.Currency)
	}
	for _, op := range in.CashOps {
		amount := conv.toBase(op.Record.Amount, op.Account.Currency)
		switch {
		case op.Record.Kind == model.CashOpDividend || op.Record.Kind == model.CashOpWithholdTax:
			t.Dividends += amount
		case op.Record.Kind.IsContribution():
			t.Contributions += amount
		}
	}

	t.Total = t.PositionsValue + t.Cash
	unrealized := t.PositionsValue - t.CostBasis
	t.Unrealized = Delta{Amount: unrealized, Pct: pct(unrealized, t.CostBasis), Known: true}
	if dayKnown {
		t.Day = Delta{Amount: dayBase, Pct: pct(dayBase, dayPrev), Known: true}
	}
	return t
}

// assignWeights sets each holding's share of the portfolio total.
func assignWeights(holdings []Holding, total float64) {
	if total == 0 {
		return
	}
	for i := range holdings {
		holdings[i].Weight = holdings[i].ValueBase / total
	}
}

// latestSnapshot is the newest snapshot time across accounts.
func latestSnapshot(accounts []model.AccountSnapshot) time.Time {
	var latest time.Time
	for _, account := range accounts {
		if account.AsOf.After(latest) {
			latest = account.AsOf
		}
	}
	return latest
}

// converter converts into the base currency and remembers which currencies it
// could not convert, so the report can say its totals are incomplete.
type converter struct {
	fx      FX
	missing map[string]bool
}

func (c converter) toBase(amount float64, currency string) float64 {
	rate, ok := c.fx.Rate(currency)
	if !ok {
		c.missing[currency] = true
		return 0
	}
	return amount * rate
}

func (c converter) warnings() []string {
	var warnings []string
	for currency := range c.missing {
		warnings = append(warnings, fmt.Sprintf(
			"no FX rate for %s: its amounts are left out of %s totals (set FX_RATES)",
			currency, c.fx.Base()))
	}
	sort.Strings(warnings)
	return warnings
}

// pct is part over whole in percent, or zero when whole is zero.
func pct(part, whole float64) float64 {
	if whole == 0 {
		return 0
	}
	return part / whole * 100
}
