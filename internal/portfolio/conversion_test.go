package portfolio

import (
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// foreignInput is a PLN account that deposits 5000 PLN and buys 10 shares of a
// EUR-priced ETF at 100 EUR while EUR/PLN is 4.3, which later quotes 110 EUR.
func foreignInput() Input {
	etf := model.Instrument{Symbol: "SXRV.DE", Name: "Nasdaq 100", Category: "ETF"}
	return Input{
		Accounts: []model.AccountSnapshot{{Account: pln, AsOf: asOf}},
		Lots: []model.Owned[model.OpenLot]{{Account: pln, Record: model.OpenLot{
			Instrument: etf, PositionID: "1", Side: model.SideBuy, Volume: 10,
			OpenTime: start, OpenPrice: 100, CurrentPrice: 110, Value: 1100,
		}}},
		CashOps: []model.Owned[model.CashOp]{
			cash(pln, "c1", model.CashOpDeposit, 5000, start.Add(-time.Hour)),
			{Account: pln, Record: model.CashOp{
				Instrument: etf, ExternalID: "c2", Kind: model.CashOpStockPurchase,
				Time: start, Amount: -4300, Volume: 10, Price: 100,
			}},
		},
		Quotes: []model.Quote{{Symbol: "SXRV.DE", AsOf: asOf, Price: 110}},
	}
}

// Regression: a foreign stock was valued at its EUR price as if it were PLN,
// so buying it looked like losing three quarters of the money spent.
func TestHistoryConvertsForeignInstruments(t *testing.T) {
	report := Build(foreignInput(), Options{FX: NewStaticFX("PLN", nil)})
	history := report.History

	if first := history[0]; !near(first.Value, 5000) {
		t.Errorf("value on the purchase day = %v, want the 5000 deposited", first.Value)
	}
	// 700 cash plus 10 * 110 EUR at 4.3.
	if last := history[len(history)-1]; !near(last.Value, 700+4730) {
		t.Errorf("value at the snapshot = %v, want 5430", last.Value)
	}
	if !near(report.Returns.TWR.Pct, 8.6) {
		t.Errorf("TWR = %v, want +8.6%% from the 10%% price rise on 4300 of 5000", report.Returns.TWR.Pct)
	}
}

func TestHoldingsConvertForeignInstruments(t *testing.T) {
	report := Build(foreignInput(), Options{FX: NewStaticFX("PLN", nil)})
	h := report.Holdings[0]

	if !near(h.Conversion, 4.3) || !near(h.Value, 4730) || !near(h.CostBasis, 4300) {
		t.Errorf("holding = conversion %v, value %v, cost %v; want 4.3, 4730, 4300",
			h.Conversion, h.Value, h.CostBasis)
	}
	// Prices stay in the instrument's currency, as the broker quotes them.
	if !near(h.Price, 110) || !near(h.AvgOpenPrice, 100) {
		t.Errorf("price %v, avg open %v, want 110 and 100 EUR", h.Price, h.AvgOpenPrice)
	}
	if len(h.LotCosts) != 1 || !near(h.LotCosts[0], 4300) {
		t.Errorf("lot costs = %v, want [4300] in account currency", h.LotCosts)
	}
	if !near(h.PL.Amount, 430) || !near(h.PL.Pct, 10) {
		t.Errorf("P/L = %+v, want +430 PLN (+10%%)", h.PL)
	}
}

// Broker values are rounded to the cent, so same-currency trades imply rates
// a hair off 1; they must not nudge values away from the broker's totals.
func TestSameCurrencyRoundingSnapsToOne(t *testing.T) {
	in := testInput()
	in.CashOps[1].Record.Amount = -1000.04
	in.CashOps[1].Record.Instrument = model.Instrument{Symbol: "SNT.PL"}
	in.CashOps[1].Record.Volume, in.CashOps[1].Record.Price = 10, 100

	rates := newConversions(in)
	if got := rates.latest(keyOf(pln, "SNT.PL")); got != 1 {
		t.Errorf("rate = %v, want exactly 1", got)
	}
	if snapRate(4.3045) != 4.3045 {
		t.Error("snapRate changed a real conversion rate")
	}
}

// One symbol held from accounts in two currencies converts separately.
func TestConversionsAreKeptPerAccountCurrency(t *testing.T) {
	stock := model.Instrument{Symbol: "NVDA.US"}
	in := Input{CashOps: []model.Owned[model.CashOp]{
		{Account: pln, Record: model.CashOp{Instrument: stock, ExternalID: "a",
			Kind: model.CashOpStockPurchase, Time: start, Amount: -3650, Volume: 10, Price: 100}},
		{Account: usd, Record: model.CashOp{Instrument: stock, ExternalID: "b",
			Kind: model.CashOpStockPurchase, Time: start, Amount: -1000, Volume: 10, Price: 100}},
	}}
	rates := newConversions(in)

	if got := rates.latest(keyOf(pln, "NVDA.US")); !near(got, 3.65) {
		t.Errorf("PLN rate = %v, want 3.65", got)
	}
	if got := rates.latest(keyOf(usd, "NVDA.US")); got != 1 {
		t.Errorf("USD rate = %v, want 1", got)
	}
}
