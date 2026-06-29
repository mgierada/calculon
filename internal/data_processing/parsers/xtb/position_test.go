package xtb

import (
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// closedPositionHeader mirrors the closed position sheet, leading blank column
// and all.
var closedPositionHeader = []string{
	"", "Position", "Symbol", "Type", "Volume", "Open time", "Open price",
	"Close time", "Close price", "Open origin", "Close origin", "Purchase value",
	"Sale value", "SL", "TP", "Margin", "Commission", "Swap", "Rollover",
	"Gross P/L", "Comment",
}

// openPositionHeader mirrors the open position sheet, which has a market price
// column and no close columns.
var openPositionHeader = []string{
	"", "Position", "Symbol", "Type", "Volume", "Open time", "Open price",
	"Market price", "Purchase value", "SL", "TP", "Margin", "Commission",
	"Swap", "Rollover", "Gross P/L", "Comment",
}

func TestParseClosedPositions(t *testing.T) {
	rows := [][]string{
		{"", "CLOSED POSITION HISTORY "},
		closedPositionHeader,
		{
			"", "2548560032", "NWG.PL", "BUY", "1.0000", "04/05/2026 12:40:36", "109.2000",
			"03/06/2026 09:49:56", "101.2000", "xStation Mobile iOS", "xStation Mobile iOS",
			"109.20", "101.20", "", "", "", "0", "0", "0", "-8",
		},
		{"", "Total", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "0", "0", "0", "-17.2"},
	}

	positions, err := parsePositions(rows, "50747414")
	if err != nil {
		t.Fatalf("parsePositions returned error: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("parsePositions returned %d positions, want 1", len(positions))
	}

	got := positions[0]
	want := model.Position{
		Provider:      model.ProviderXTB,
		AccountID:     "50747414",
		ExternalID:    "2548560032",
		Symbol:        "NWG.PL",
		Side:          model.SideBuy,
		Volume:        1,
		OpenTime:      time.Date(2026, 5, 4, 12, 40, 36, 0, time.UTC),
		OpenPrice:     109.20,
		CloseTime:     time.Date(2026, 6, 3, 9, 49, 56, 0, time.UTC),
		ClosePrice:    101.20,
		PurchaseValue: 109.20,
		SaleValue:     101.20,
		GrossPL:       -8,
	}
	if got != want {
		t.Errorf("parsePositions returned\n%+v\nwant\n%+v", got, want)
	}
	if got.IsOpen() {
		t.Error("position with a close time reports open")
	}
}

func TestParseOpenPositions(t *testing.T) {
	rows := [][]string{
		{"", "OPEN POSITION HISTORY "},
		openPositionHeader,
		{
			"", "999", "CDR.PL", "BUY", "5.0000", "17/06/2026 14:51:32", "227.9000",
			"231.0000", "1139.50", "", "", "", "0", "0", "0", "15.5",
		},
		{"", "Total"},
	}

	positions, err := parsePositions(rows, "50747414")
	if err != nil {
		t.Fatalf("parsePositions returned error: %v", err)
	}
	if len(positions) != 1 {
		t.Fatalf("parsePositions returned %d positions, want 1", len(positions))
	}

	got := positions[0]
	if !got.IsOpen() {
		t.Error("position from the open sheet reports closed")
	}
	if got.ClosePrice != 0 {
		t.Errorf("close price = %v, want 0", got.ClosePrice)
	}
	if got.OpenPrice != 227.90 || got.Volume != 5 {
		t.Errorf("open price/volume = %v/%v, want 227.9/5", got.OpenPrice, got.Volume)
	}
}

func TestParsePositionsRejectsInvalidRow(t *testing.T) {
	tests := map[string][]string{
		"zero volume": {
			"", "1", "NWG.PL", "BUY", "0", "04/05/2026 12:40:36", "109.2000",
			"", "", "", "", "", "", "", "", "", "0", "0", "0", "0",
		},
		"unknown side": {
			"", "1", "NWG.PL", "HOLD", "1", "04/05/2026 12:40:36", "109.2000",
			"", "", "", "", "", "", "", "", "", "0", "0", "0", "0",
		},
		"missing open time": {
			"", "1", "NWG.PL", "BUY", "1", "", "109.2000",
			"", "", "", "", "", "", "", "", "", "0", "0", "0", "0",
		},
		"unparseable price": {
			"", "1", "NWG.PL", "BUY", "1", "04/05/2026 12:40:36", "one hundred",
			"", "", "", "", "", "", "", "", "", "0", "0", "0", "0",
		},
		"close before open": {
			"", "1", "NWG.PL", "BUY", "1", "04/05/2026 12:40:36", "109.2000",
			"03/05/2026 09:49:56", "101.2", "", "", "", "", "", "", "", "0", "0", "0", "0",
		},
	}

	for name, row := range tests {
		rows := [][]string{closedPositionHeader, row}
		if _, err := parsePositions(rows, "1"); err == nil {
			t.Errorf("parsePositions accepted a row with %s, want error", name)
		}
	}
}

func TestParsePositionsWithoutTable(t *testing.T) {
	rows := [][]string{{"", "CASH OPERATION HISTORY"}, {"", "ID", "Type", "Time", "Amount"}}

	positions, err := parsePositions(rows, "1")
	if err != nil {
		t.Fatalf("parsePositions returned error: %v", err)
	}
	if positions != nil {
		t.Errorf("parsePositions returned %v, want nil for a sheet with no table", positions)
	}
}
