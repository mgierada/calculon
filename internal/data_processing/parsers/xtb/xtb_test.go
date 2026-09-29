package xtb

import (
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

var closedSheet = [][]string{
	{"Account number", "51099570"},
	{"Closed Positions"},
	{"Date from (UTC)", "2006-01-01 00:00:00"},
	{"Date to (UTC)", "2026-09-28 09:17:54"},
	{"Instrument", "Ticker", "Category", "Type", "Volume", "Open Price", "Open Time (UTC)",
		"Close Price", "Close Time (UTC)", "Product", "Profit/Loss", "Gross Profit",
		"Purchase Value", "Sale Value", "Stop Loss", "Take Profit", "Commission", "Margin",
		"Swap", "Rollover", "Open Conversion Rate", "Close Conversion Rate", "Close Origin",
		"Position ID", "Comment"},
	// Two partial closes of one position, then an exact duplicate of the first.
	{"Kruk", "KRU.PL", "STOCK", "BUY", "4", "399.4", "2025-07-02 11:03:14", "424.4",
		"2026-06-30 14:20:11", "IKE", "100", "100", "1597.6", "1697.6", "", "", "0", "", "", "",
		"", "", "iOS", "1903896705"},
	{"Kruk", "KRU.PL", "STOCK", "BUY", "1", "399.4", "2025-07-02 11:03:14", "424.4",
		"2026-06-30 14:20:11", "IKE", "25", "25", "399.4", "424.4", "", "", "0", "", "", "",
		"", "", "iOS", "1903896705"},
	{"Kruk", "KRU.PL", "STOCK", "BUY", "4", "399.4", "2025-07-02 11:03:14", "424.4",
		"2026-06-30 14:20:11", "IKE", "100", "100", "1597.6", "1697.6", "", "", "0", "", "", "",
		"", "", "iOS", "1903896705"},
	{"Profit/loss", "", "", "", "", "", "", "", "", "", "225", "225"},
}

var cashSheet = [][]string{
	{"Account number", "51099570"},
	{"Cash Operations"},
	{"Date from (UTC)", "2006-01-01 00:00:00"},
	{"Date to (UTC)", "2026-09-28 09:17:54"},
	{"Type", "Instrument", "Ticker", "Category", "Time", "Amount", "ID", "Comment", "Product",
		"Position ID"},
	{"Stock purchase", "LPP", "LPP.PL", "STOCK", "2026-09-16 08:17:01", "-179.11", "1440598663",
		"OPEN BUY 0.0083 @ 21580.00", "IKE", "2818425338"},
	{"Stock sell", "Kruk", "KRU.PL", "STOCK", "2026-06-30 14:20:11", "424.4", "1300000001",
		"CLOSE BUY 1/5 @ 424.40", "IKE", "1903896705"},
	{"Dividend", "Digital Network", "DIG.PL", "STOCK", "2026-09-15 09:57:36", "180", "1439349927",
		"DIG.PL PLN 5.0/ SHR", "IKE"},
	{"IKE deposit", "", "", "", "2024-10-02 07:19:40", "10000", "627247955",
		"Transfer in operation on account with id 51099570", "IKE"},
	{"Something new", "", "", "", "2024-10-02 07:19:40", "1", "627247956", "", "IKE"},
	{"Total", "", "", "", "", "10425.29"},
}

// XTB stamps the parent account number on the open position sheet of an IKE
// sub-account and leaves its currency blank.
var openSheet = [][]string{
	{"Account number", "50747414"},
	{"Open Positions"},
	{"Data as of report generated", "2026-09-28 09:17:56"},
	{"Product", "Metric", "Amount", "Currency"},
	{"IKE", "Open position value", "132651.9"},
	{"IKE", "Open position profit", "44794.1"},
	{},
	{"Note", "Summary values and open positions are shown as of the report generation time"},
	{"Product", "Instrument/Position", "Ticker", "Category", "Type", "Volume", "Value",
		"Current price", "Open price", "Open time (UTC)", "Stop Loss", "Take Profit",
		"Net Profit %", "Net Profit", "Gross Profit", "Margin", "Open Commission", "Swap",
		"Rollover"},
	{"IKE", "XTB", "XTB.PL", "STOCK", "", "92", "13818.4", "", "65.01", "", "", "", "131",
		"7837.4", "7837.4"},
	{"IKE", "1484232628", "XTB.PL", "", "BUY", "77", "11565.4", "150.2", "64",
		"2024-10-07 07:06:45", "", "", "134.69", "6637.4", "6637.4"},
	{"IKE", "1909730385", "XTB.PL", "", "BUY", "15", "2253", "150.2", "70.78",
		"2025-07-07 11:43:26", "", "", "112.21", "1191.3", "1191.3"},
}

func testSheets() map[string][][]string {
	return map[string][][]string{
		sheetClosedPositions: closedSheet,
		sheetCashOps:         cashSheet,
		sheetOpenPositions:   openSheet,
	}
}

func parseTestStatement(t *testing.T, fileName string) model.Statement {
	t.Helper()

	statement, err := parseSheets(testSheets(), fileName)
	if err != nil {
		t.Fatalf("parseSheets returned error: %v", err)
	}
	return statement
}

func TestParseAccountPrefersHistorySheets(t *testing.T) {
	statement := parseTestStatement(t, "IKE_51099570_2006-01-01_2026-09-28.xlsx")

	want := model.Account{Provider: model.ProviderXTB, ID: "51099570", Currency: "PLN"}
	if statement.Account != want {
		t.Errorf("account = %+v, want %+v", statement.Account, want)
	}
}

func TestParseCurrencyFromSummaryColumn(t *testing.T) {
	sheets := map[string][][]string{sheetOpenPositions: {
		{"Product", "Metric", "Amount", "Currency"},
		{"My Trades", "Open position value", "23605.97", "USD"},
	}}

	if got := parseCurrency(sheets, "whatever.xlsx"); got != "USD" {
		t.Errorf("parseCurrency = %q, want USD", got)
	}
}

func TestParseCurrencyFallsBackToFileName(t *testing.T) {
	if got := parseCurrency(map[string][][]string{}, "EUR_1_2006-01-01_2026-09-28.xlsx"); got != "EUR" {
		t.Errorf("parseCurrency = %q, want EUR", got)
	}
}

// The lot table below the summary has a column at the summary's "Currency"
// index; it must never be read as a currency.
func TestParseCurrencyIgnoresLotRows(t *testing.T) {
	sheets := map[string][][]string{sheetOpenPositions: {
		{"Product", "Metric", "Amount", "Currency"},
		{"Unknown", "Open position value", "1"},
		{"Unknown", "1484232628", "XTB.PL", "STOCK", "BUY"},
	}}

	if got := parseCurrency(sheets, "statement.xlsx"); got != "" {
		t.Errorf("parseCurrency = %q, want no currency", got)
	}
}

func TestParseAsOf(t *testing.T) {
	statement := parseTestStatement(t, "IKE.xlsx")

	want := time.Date(2026, 9, 28, 9, 17, 56, 0, time.UTC)
	if !statement.AsOf.Equal(want) {
		t.Errorf("as of = %s, want %s", statement.AsOf, want)
	}
}

func TestParseClosedPositionsKeepsPartialCloses(t *testing.T) {
	statement := parseTestStatement(t, "IKE.xlsx")

	if len(statement.Positions) != 3 {
		t.Fatalf("parsed %d positions, want 3", len(statement.Positions))
	}
	keys := map[string]bool{}
	for _, position := range statement.Positions {
		keys[position.Key()] = true
	}
	if len(keys) != 3 {
		t.Errorf("positions share keys: %v", keys)
	}

	first := statement.Positions[0]
	if first.Symbol != "KRU.PL" || first.Name != "Kruk" || first.Volume != 4 ||
		first.NetPL != 100 || first.CloseOrigin != "iOS" || first.Product != "IKE" {
		t.Errorf("first position = %+v", first)
	}
	if statement.Positions[2].Seq != 1 {
		t.Errorf("duplicate row seq = %d, want 1", statement.Positions[2].Seq)
	}
}

func TestParseOpenLotsTakesNamesFromGroupRows(t *testing.T) {
	statement := parseTestStatement(t, "IKE.xlsx")

	if len(statement.OpenLots) != 2 {
		t.Fatalf("parsed %d open lots, want 2", len(statement.OpenLots))
	}
	lot := statement.OpenLots[0]
	want := model.Instrument{Symbol: "XTB.PL", Name: "XTB", Category: "STOCK"}
	if lot.Instrument != want {
		t.Errorf("lot instrument = %+v, want %+v", lot.Instrument, want)
	}
	if lot.PositionID != "1484232628" || lot.Volume != 77 || lot.CurrentPrice != 150.2 ||
		lot.Value != 11565.4 || lot.OpenPrice != 64 {
		t.Errorf("lot = %+v", lot)
	}
}

func TestParseCashOps(t *testing.T) {
	statement := parseTestStatement(t, "IKE.xlsx")

	if len(statement.CashOps) != 5 {
		t.Fatalf("parsed %d cash ops, want 5", len(statement.CashOps))
	}
	tests := []struct {
		kind   model.CashOpKind
		volume float64
		price  float64
	}{
		{model.CashOpStockPurchase, 0.0083, 21580},
		{model.CashOpStockSale, 1, 424.4},
		{model.CashOpDividend, 0, 0},
		{model.CashOpTransfer, 0, 0},
		{model.CashOpOther, 0, 0},
	}
	for i, test := range tests {
		op := statement.CashOps[i]
		if op.Kind != test.kind || op.Volume != test.volume || op.Price != test.price {
			t.Errorf("cash op %d = %s %v@%v, want %s %v@%v",
				i, op.Kind, op.Volume, op.Price, test.kind, test.volume, test.price)
		}
	}
	if got := statement.CashOps[4].RawType; got != "Something new" {
		t.Errorf("unmapped cash op raw type = %q, want it kept", got)
	}
}

func TestParseStockTradeComment(t *testing.T) {
	tests := []struct {
		comment string
		volume  float64
		price   float64
	}{
		{"OPEN BUY 4 @ 273.60", 4, 273.60},
		{"OPEN BUY 1/2 @ 206.00", 1, 206},
		{"CLOSE BUY 25/70 @ 29.600", 25, 29.6},
		{"OPEN BUY 0.0083 @ 21580.00", 0.0083, 21580},
	}
	for _, test := range tests {
		volume, price, err := parseStockTradeComment(test.comment)
		if err != nil {
			t.Errorf("parseStockTradeComment(%q) returned error: %v", test.comment, err)
			continue
		}
		if volume != test.volume || price != test.price {
			t.Errorf("parseStockTradeComment(%q) = %v @ %v, want %v @ %v",
				test.comment, volume, price, test.volume, test.price)
		}
	}

	if _, _, err := parseStockTradeComment("DIG.PL PLN 5.0/ SHR"); err == nil {
		t.Error("parseStockTradeComment accepted a dividend comment")
	}
}

func TestParseFailsOnMalformedRow(t *testing.T) {
	sheets := testSheets()
	broken := append([][]string{}, cashSheet...)
	broken[5] = []string{"Stock purchase", "LPP", "LPP.PL", "STOCK", "2026-09-16 08:17:01",
		"not a number", "1440598663", "OPEN BUY 1 @ 1", "IKE"}
	sheets[sheetCashOps] = broken

	if _, err := parseSheets(sheets, "IKE.xlsx"); err == nil {
		t.Fatal("parseSheets accepted a malformed amount")
	}
}

func TestParseFailsWithoutAccount(t *testing.T) {
	if _, err := parseSheets(map[string][][]string{"x": {{"nothing"}}}, "PLN_1.xlsx"); err == nil {
		t.Fatal("parseSheets succeeded without an account number")
	}
}
