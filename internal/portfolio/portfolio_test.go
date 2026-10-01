package portfolio

import (
	"math"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

var (
	pln   = model.Account{Provider: model.ProviderXTB, ID: "50747414", Currency: "PLN"}
	usd   = model.Account{Provider: model.ProviderXTB, ID: "51727538", Currency: "USD"}
	asOf  = time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	start = time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	fx    = NewStaticFX("PLN", map[string]float64{"USD": 4})
)

func lot(account model.Account, id, symbol string, volume, openPrice, price float64) model.Owned[model.OpenLot] {
	return model.Owned[model.OpenLot]{Account: account, Record: model.OpenLot{
		Instrument:   model.Instrument{Symbol: symbol, Name: symbol, Category: "STOCK"},
		PositionID:   id,
		Side:         model.SideBuy,
		Volume:       volume,
		OpenTime:     start,
		OpenPrice:    openPrice,
		CurrentPrice: price,
		Value:        volume * price,
	}}
}

func cash(account model.Account, id string, kind model.CashOpKind, amount float64, at time.Time) model.Owned[model.CashOp] {
	return model.Owned[model.CashOp]{Account: account, Record: model.CashOp{
		ExternalID: id, Kind: kind, Time: at, Amount: amount,
	}}
}

// testInput is a PLN account with 10 SNT bought at 100 now worth 120 and 50
// PLN cash left, and a USD account with 2 AAPL bought at 200 now worth 250
// and 100 USD cash left.
func testInput() Input {
	return Input{
		User: "alice",
		Accounts: []model.AccountSnapshot{
			{Account: pln, AsOf: asOf}, {Account: usd, AsOf: asOf},
		},
		Lots: []model.Owned[model.OpenLot]{
			lot(pln, "1", "SNT.PL", 6, 100, 120),
			lot(pln, "2", "SNT.PL", 4, 100, 120),
			lot(usd, "3", "AAPL.US", 2, 200, 250),
		},
		CashOps: []model.Owned[model.CashOp]{
			cash(pln, "c1", model.CashOpDeposit, 1050, start.Add(-time.Hour)),
			cash(pln, "c2", model.CashOpStockPurchase, -1000, start),
			cash(usd, "c3", model.CashOpDeposit, 500, start.Add(-time.Hour)),
			cash(usd, "c4", model.CashOpStockPurchase, -400, start),
		},
		Quotes: []model.Quote{
			{Symbol: "SNT.PL", AsOf: asOf, Price: 120},
			{Symbol: "AAPL.US", AsOf: asOf, Price: 250},
		},
	}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func TestBuildHoldingsAggregatesLotsPerAccountAndSymbol(t *testing.T) {
	report := Build(testInput(), Options{FX: fx})

	if len(report.Holdings) != 2 {
		t.Fatalf("got %d holdings, want 2", len(report.Holdings))
	}
	// AAPL is worth 2 * 250 * 4 = 2000 PLN, more than SNT's 1200, so it leads.
	aapl, snt := report.Holdings[0], report.Holdings[1]
	if aapl.Symbol != "AAPL.US" || !near(aapl.ValueBase, 2000) {
		t.Errorf("first holding = %s worth %v PLN, want AAPL.US worth 2000", aapl.Symbol, aapl.ValueBase)
	}
	if snt.Volume != 10 || len(snt.Lots) != 2 || !near(snt.Value, 1200) || !near(snt.AvgOpenPrice, 100) {
		t.Errorf("SNT holding = %+v", snt)
	}
	if !near(snt.PL.Amount, 200) || !near(snt.PL.Pct, 20) {
		t.Errorf("SNT P/L = %+v, want 200 (20%%)", snt.PL)
	}
}

func TestBuildTotalsConvertIntoBase(t *testing.T) {
	report := Build(testInput(), Options{FX: fx})
	totals := report.Totals

	// Cash: 50 PLN + 100 USD * 4 = 450 PLN. Positions: 1200 + 2000.
	if !near(totals.Cash, 450) || !near(totals.PositionsValue, 3200) || !near(totals.Total, 3650) {
		t.Errorf("totals = %+v, want cash 450, positions 3200, total 3650", totals)
	}
	// Cost: 1000 + 400 * 4 = 2600, so unrealized is 600.
	if !near(totals.Unrealized.Amount, 600) {
		t.Errorf("unrealized = %+v, want 600", totals.Unrealized)
	}
	// Deposits: 1050 + 500 * 4.
	if !near(totals.Contributions, 3050) {
		t.Errorf("contributions = %v, want 3050", totals.Contributions)
	}
	if !near(report.Holdings[0].Weight, 2000.0/3650) {
		t.Errorf("AAPL weight = %v, want %v", report.Holdings[0].Weight, 2000.0/3650)
	}
	if len(report.Warnings) != 0 {
		t.Errorf("warnings = %v, want none", report.Warnings)
	}
}

func TestBuildWarnsAboutMissingFX(t *testing.T) {
	report := Build(testInput(), Options{FX: NewStaticFX("PLN", nil)})

	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %v, want one about USD", report.Warnings)
	}
	if !near(report.Totals.PositionsValue, 1200) {
		t.Errorf("positions value = %v, want only the PLN holding", report.Totals.PositionsValue)
	}
}

func TestDayChangeUnknownWithoutPreviousClose(t *testing.T) {
	report := Build(testInput(), Options{FX: fx})

	if report.Totals.Day.Known {
		t.Errorf("day change = %+v, want unknown with a single quote", report.Totals.Day)
	}
}

func TestDayChangeFromEarlierDayQuote(t *testing.T) {
	in := testInput()
	in.Quotes = append(in.Quotes,
		model.Quote{Symbol: "SNT.PL", AsOf: asOf.AddDate(0, 0, -1), Price: 100},
		// A quote from the same day is not a previous close.
		model.Quote{Symbol: "SNT.PL", AsOf: asOf.Add(-time.Hour), Price: 110},
	)
	report := Build(in, Options{FX: fx})

	snt := report.Holdings[1]
	if !snt.Day.Known || !near(snt.Day.Amount, 200) || !near(snt.Day.Pct, 20) {
		t.Errorf("SNT day change = %+v, want +200 (20%%)", snt.Day)
	}
	if !report.Totals.Day.Known || !near(report.Totals.Day.Amount, 200) {
		t.Errorf("total day change = %+v, want +200", report.Totals.Day)
	}
}

// The previous close a market data source reports beats our own history,
// which may hold no close at all, only a mid-session snapshot.
func TestDayChangeFromReportedPreviousClose(t *testing.T) {
	in := testInput()
	in.Quotes = append(in.Quotes,
		model.Quote{Symbol: "SNT.PL", AsOf: asOf.AddDate(0, 0, -1), Price: 100},
		model.Quote{Symbol: "SNT.PL", AsOf: asOf.Add(time.Hour), Price: 130, PrevClose: 125},
	)
	report := Build(in, Options{FX: fx})

	snt := report.Holdings[1]
	if !snt.Day.Known || !near(snt.Day.Amount, 50) || !near(snt.Day.Pct, 4) {
		t.Errorf("SNT day change = %+v, want +50 (4%%) from the reported close", snt.Day)
	}
}

// A newer quote without a reported close, like a statement imported after the
// last poll, measures day change from history as before.
func TestDayChangeFallsBackWhenLatestQuoteReportsNoClose(t *testing.T) {
	in := testInput()
	in.Quotes = append(in.Quotes,
		model.Quote{Symbol: "SNT.PL", AsOf: asOf.AddDate(0, 0, -1), Price: 100},
		model.Quote{Symbol: "SNT.PL", AsOf: asOf.Add(-time.Hour), Price: 110, PrevClose: 105},
	)
	report := Build(in, Options{FX: fx})

	snt := report.Holdings[1]
	if !snt.Day.Known || !near(snt.Day.Amount, 200) {
		t.Errorf("SNT day change = %+v, want +200 from the earlier day's quote", snt.Day)
	}
}

// fixedPrices is a PriceSource stub standing in for a market data API.
type fixedPrices map[string]Quote

func (f fixedPrices) Latest(symbol string) (Quote, bool) {
	q, ok := f[symbol]
	return q, ok
}

func TestBuildUsesInjectedPriceSource(t *testing.T) {
	prices := fixedPrices{"SNT.PL": {Price: 130, PrevClose: 125, AsOf: asOf}}
	report := Build(testInput(), Options{FX: fx, Prices: prices})

	snt := report.Holdings[1]
	if !near(snt.Value, 1300) || !snt.Day.Known {
		t.Errorf("SNT = value %v day %+v, want 1300 and a known day change", snt.Value, snt.Day)
	}
	// AAPL has no quote from the source and keeps its statement price.
	if !near(report.Holdings[0].Price, 250) {
		t.Errorf("AAPL price = %v, want the statement's 250", report.Holdings[0].Price)
	}
}

func TestBuildSkipsCFDLots(t *testing.T) {
	in := testInput()
	cfd := lot(pln, "9", "GOLD", 1, 2000, 2100)
	cfd.Record.Category = model.CategoryCFD
	in.Lots = append(in.Lots, cfd)

	if report := Build(in, Options{FX: fx}); len(report.Holdings) != 2 {
		t.Errorf("got %d holdings, want the CFD left out", len(report.Holdings))
	}
}

func TestValueHistory(t *testing.T) {
	report := Build(testInput(), Options{FX: fx})
	history := report.History

	// One point per day from the first deposit on the 20th through the 28th.
	if len(history) != 9 {
		t.Fatalf("got %d history points, want 9", len(history))
	}
	// The 20th: shares bought at their open prices, so value is what was
	// deposited: 1050 + 500 * 4.
	if first := history[0]; !near(first.Value, 3050) || !near(first.Contributions, 3050) {
		t.Errorf("first point = %+v, want value and contributions 3050", first)
	}
	// Until the snapshot quote, shares stay marked at their open price.
	if middle := history[4]; !near(middle.Value, 3050) {
		t.Errorf("middle point = %+v, want 3050", middle)
	}
	// The last point is valued at the snapshot and matches the report total.
	if last := history[len(history)-1]; !near(last.Value, report.Totals.Total) {
		t.Errorf("last point = %+v, want the total %v", last, report.Totals.Total)
	}
}

func TestValueHistoryEndsClosedPositions(t *testing.T) {
	in := testInput()
	in.Lots = in.Lots[2:] // only AAPL still open
	in.Closed = []model.Owned[model.Position]{{Account: pln, Record: model.Position{
		Instrument: model.Instrument{Symbol: "SNT.PL", Category: "STOCK"},
		PositionID: "1", Side: model.SideBuy, Volume: 10,
		OpenTime: start, OpenPrice: 100,
		CloseTime: start.AddDate(0, 0, 3), ClosePrice: 110, NetPL: 100,
	}}}
	in.CashOps = append(in.CashOps, cash(pln, "c5", model.CashOpStockSale, 1100, start.AddDate(0, 0, 3)))
	report := Build(in, Options{FX: fx})

	// After the sale the PLN side is all cash: 50 + 1100.
	last := report.History[len(report.History)-1]
	if want := 1150 + 2*250*4 + 100*4.0; !near(last.Value, want) {
		t.Errorf("last point = %v, want %v", last.Value, want)
	}
	if !near(report.Totals.RealizedPL, 100) {
		t.Errorf("realized = %v, want 100", report.Totals.RealizedPL)
	}
}

func TestAllocation(t *testing.T) {
	report := Build(testInput(), Options{FX: fx})

	bySymbol := Allocation(report, BySymbol)
	labels := []string{}
	for _, s := range bySymbol {
		labels = append(labels, s.Label)
	}
	if len(bySymbol) != 3 || labels[0] != "AAPL.US" || labels[1] != "SNT.PL" || labels[2] != "cash" {
		t.Errorf("allocation by symbol = %v, want AAPL.US, SNT.PL, cash", labels)
	}
	var total float64
	for _, s := range bySymbol {
		total += s.Weight
	}
	if !near(total, 100) {
		t.Errorf("weights sum to %v, want 100", total)
	}

	byCurrency := Allocation(report, ByCurrency)
	if len(byCurrency) != 2 || byCurrency[0].Label != "USD" || !near(byCurrency[0].Value, 2400) {
		t.Errorf("allocation by currency = %+v, want USD 2400 first", byCurrency)
	}
}

func TestTopSlicesFoldsTheRest(t *testing.T) {
	slices := []Slice{{"a", 50, 50}, {"b", 30, 30}, {"c", 15, 15}, {"d", 5, 5}}

	top := TopSlices(slices, 3)
	if len(top) != 3 || top[2].Label != "other" || !near(top[2].Value, 20) {
		t.Errorf("TopSlices = %+v, want a, b, other(20)", top)
	}
}

func TestParseRates(t *testing.T) {
	rates, err := ParseRates("USD=3.65, eur=4.26")
	if err != nil {
		t.Fatalf("ParseRates returned error: %v", err)
	}
	if rates["USD"] != 3.65 || rates["EUR"] != 4.26 {
		t.Errorf("rates = %v", rates)
	}

	for _, bad := range []string{"USD", "USD=abc", "USD=-1"} {
		if _, err := ParseRates(bad); err == nil {
			t.Errorf("ParseRates(%q) succeeded, want error", bad)
		}
	}
}

func TestScopeKeepsOneAccountInItsCurrency(t *testing.T) {
	in, opts, ok := Scope(testInput(), Options{FX: fx}, usd.Key())
	if !ok {
		t.Fatal("Scope did not find the USD account")
	}
	report := Build(in, opts)

	if report.Base != "USD" {
		t.Errorf("base = %q, want the account's own currency", report.Base)
	}
	if len(report.Holdings) != 1 || report.Holdings[0].Symbol != "AAPL.US" {
		t.Errorf("holdings = %+v, want only AAPL.US", report.Holdings)
	}
	// 2 * 250 in positions plus 100 cash, unconverted.
	if !near(report.Totals.Total, 600) || !near(report.Totals.Contributions, 500) {
		t.Errorf("totals = %+v, want total 600 and deposits 500 USD", report.Totals)
	}
	if !near(report.Holdings[0].Weight, 500.0/600) {
		t.Errorf("weight = %v, want share of this account only", report.Holdings[0].Weight)
	}
}

func TestScopeUnknownAccount(t *testing.T) {
	missing := model.AccountKey{Provider: model.ProviderXTB, ID: "nope"}
	if _, _, ok := Scope(testInput(), Options{FX: fx}, missing); ok {
		t.Error("Scope found an account the user does not have")
	}
}
