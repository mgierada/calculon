package db

import (
	"math"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

func cashOpAt(externalID, symbol string, kind model.CashOpKind, volume, price float64,
	when time.Time) model.CashOp {
	op := testCashOp(externalID, symbol, kind, volume, price)
	op.Time = when
	return op
}

func TestOpenHoldingsWeightsAverageByVolume(t *testing.T) {
	conn := openTestDB(t)
	first := time.Date(2026, 5, 28, 9, 15, 33, 0, time.UTC)

	statement := model.Statement{CashOps: []model.CashOp{
		cashOpAt("1", "SNT.PL", model.CashOpStockPurchase, 4, 273.60, first),
		cashOpAt("2", "SNT.PL", model.CashOpStockPurchase, 4, 273.00, first.AddDate(0, 0, 1)),
		cashOpAt("3", "SNT.PL", model.CashOpStockPurchase, 2, 293.00, first.AddDate(0, 0, 20)),
		// Cash ops without a volume must not affect the holding.
		cashOpAt("4", "SNT.PL", model.CashOpDividend, 0, 0, first.AddDate(0, 0, 5)),
		cashOpAt("5", "", model.CashOpDeposit, 0, 0, first),
	}}
	if _, err := Import(conn, statement); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	holdings, err := OpenHoldings(conn)
	if err != nil {
		t.Fatalf("OpenHoldings returned error: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("OpenHoldings returned %d holdings, want 1", len(holdings))
	}

	got := holdings[0]
	if got.Symbol != "SNT.PL" {
		t.Errorf("symbol = %q, want %q", got.Symbol, "SNT.PL")
	}
	if got.Volume != 10 {
		t.Errorf("volume = %v, want 10", got.Volume)
	}
	// (4*273.60 + 4*273.00 + 2*293.00) / 10
	if wantAvg := 277.24; math.Abs(got.AvgOpenPrice-wantAvg) > 1e-9 {
		t.Errorf("avg open price = %v, want %v", got.AvgOpenPrice, wantAvg)
	}
	if !got.FirstOpen.Equal(first) {
		t.Errorf("first open = %s, want %s", got.FirstOpen, first)
	}
	if days := got.DaysHeld(first.AddDate(0, 0, 62)); days != 62 {
		t.Errorf("days held = %d, want 62", days)
	}
}

func TestOpenHoldingsNetsSalesAndDropsClosed(t *testing.T) {
	conn := openTestDB(t)
	when := time.Date(2026, 5, 28, 9, 15, 33, 0, time.UTC)

	statement := model.Statement{CashOps: []model.CashOp{
		// Fully sold: must not appear at all.
		cashOpAt("1", "NWG.PL", model.CashOpStockPurchase, 1, 109.20, when),
		cashOpAt("2", "NWG.PL", model.CashOpStockPurchase, 1, 110.40, when),
		cashOpAt("3", "NWG.PL", model.CashOpStockSale, 1, 101.20, when.AddDate(0, 0, 6)),
		cashOpAt("4", "NWG.PL", model.CashOpStockSale, 1, 101.20, when.AddDate(0, 0, 6)),
		// Partly sold: the remainder keeps the full purchase average as its basis.
		cashOpAt("5", "DNP.PL", model.CashOpStockPurchase, 70, 42.85, when),
		cashOpAt("6", "DNP.PL", model.CashOpStockSale, 25, 29.60, when.AddDate(0, 0, 10)),
	}}
	if _, err := Import(conn, statement); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	holdings, err := OpenHoldings(conn)
	if err != nil {
		t.Fatalf("OpenHoldings returned error: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("OpenHoldings returned %v, want only DNP.PL", holdings)
	}
	if holdings[0].Symbol != "DNP.PL" || holdings[0].Volume != 45 {
		t.Errorf("holding = %+v, want DNP.PL with volume 45", holdings[0])
	}
	if math.Abs(holdings[0].AvgOpenPrice-42.85) > 1e-9 {
		t.Errorf("avg open price = %v, want 42.85", holdings[0].AvgOpenPrice)
	}
}

// Fractional share fills leave floating point residue that must not read as a
// still-open holding.
func TestOpenHoldingsIgnoresFractionalResidue(t *testing.T) {
	conn := openTestDB(t)
	when := time.Date(2026, 6, 1, 9, 43, 19, 0, time.UTC)

	statement := model.Statement{CashOps: []model.CashOp{
		cashOpAt("1", "CRI.PL", model.CashOpStockPurchase, 0.3108, 965.00, when),
		cashOpAt("2", "CRI.PL", model.CashOpStockPurchase, 0.505, 994.00, when),
		cashOpAt("3", "CRI.PL", model.CashOpStockSale, 0.3108, 970.00, when),
		cashOpAt("4", "CRI.PL", model.CashOpStockSale, 0.505, 999.00, when),
	}}
	if _, err := Import(conn, statement); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	holdings, err := OpenHoldings(conn)
	if err != nil {
		t.Fatalf("OpenHoldings returned error: %v", err)
	}
	if len(holdings) != 0 {
		t.Errorf("OpenHoldings returned %+v, want none", holdings)
	}
}

// Holdings aggregate across accounts, so the same symbol in two accounts is one
// portfolio row.
func TestOpenHoldingsAggregatesAcrossAccounts(t *testing.T) {
	conn := openTestDB(t)
	when := time.Date(2026, 5, 28, 9, 15, 33, 0, time.UTC)

	other := cashOpAt("2", "SNT.PL", model.CashOpStockPurchase, 1, 271.80, when)
	other.AccountID = "51099570"

	statement := model.Statement{CashOps: []model.CashOp{
		cashOpAt("1", "SNT.PL", model.CashOpStockPurchase, 4, 273.60, when),
		other,
	}}
	if _, err := Import(conn, statement); err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	holdings, err := OpenHoldings(conn)
	if err != nil {
		t.Fatalf("OpenHoldings returned error: %v", err)
	}
	if len(holdings) != 1 || holdings[0].Volume != 5 {
		t.Fatalf("holdings = %+v, want one SNT.PL row with volume 5", holdings)
	}
}

func TestOpenHoldingsEmptyDatabase(t *testing.T) {
	holdings, err := OpenHoldings(openTestDB(t))
	if err != nil {
		t.Fatalf("OpenHoldings returned error: %v", err)
	}
	if len(holdings) != 0 {
		t.Errorf("OpenHoldings returned %+v, want none", holdings)
	}
}
