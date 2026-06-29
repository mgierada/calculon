package xtb

import (
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

func TestParseStockTradeComment(t *testing.T) {
	tests := []struct {
		comment string
		volume  float64
		price   float64
	}{
		{"OPEN BUY 4 @ 273.60", 4, 273.60},
		{"OPEN BUY 30 @ 6.990", 30, 6.990},
		{"OPEN BUY 0.3108 @ 965.00", 0.3108, 965.00},
		{"OPEN BUY 1/2 @ 206.00", 1, 206.00},
		{"OPEN BUY 0.0101/2.0101 @ 50.24", 0.0101, 50.24},
		{"CLOSE BUY 25/70 @ 29.600", 25, 29.600},
		{"OPEN SELL 2 @ 85.4400", 2, 85.4400},
		{"  OPEN BUY 4 @ 273.60  ", 4, 273.60},
	}

	for _, test := range tests {
		volume, price, err := parseStockTradeComment(test.comment)
		if err != nil {
			t.Errorf("parseStockTradeComment(%q) returned error: %v", test.comment, err)
			continue
		}
		if volume != test.volume {
			t.Errorf("parseStockTradeComment(%q) volume = %v, want %v", test.comment, volume, test.volume)
		}
		if price != test.price {
			t.Errorf("parseStockTradeComment(%q) price = %v, want %v", test.comment, price, test.price)
		}
	}
}

func TestParseStockTradeCommentRejectsOtherComments(t *testing.T) {
	comments := []string{
		"",
		"Profit of position #2548560032",
		"Free-funds Interest 2026-05",
		"VRC.PL PLN 2.7300/ SHR",
		"OPEN BUY 4 @",
		"OPEN BUY @ 273.60",
	}

	for _, comment := range comments {
		if _, _, err := parseStockTradeComment(comment); err == nil {
			t.Errorf("parseStockTradeComment(%q) succeeded, want error", comment)
		}
	}
}

func TestCashOpKind(t *testing.T) {
	tests := []struct {
		rawType string
		want    model.CashOpKind
	}{
		{"deposit", model.CashOpDeposit},
		{"withdrawal", model.CashOpWithdrawal},
		{"Stock purchase", model.CashOpStockPurchase},
		{"Stock sale", model.CashOpStockSale},
		{"close trade", model.CashOpCloseTrade},
		{"DIVIDENT", model.CashOpDividend},
		{"Withholding Tax", model.CashOpWithholdTax},
		{"Free-funds Interest", model.CashOpInterest},
		{"Free-funds Interest Tax", model.CashOpInterestTax},
		{"Something New From XTB", model.CashOpOther},
	}

	for _, test := range tests {
		if got := cashOpKind(test.rawType); got != test.want {
			t.Errorf("cashOpKind(%q) = %q, want %q", test.rawType, got, test.want)
		}
	}
}

func TestParseCashOps(t *testing.T) {
	rows := [][]string{
		{"", "ID", "Type", "Time", "Comment", "Symbol", "Amount"},
		{"", "1", "Stock purchase", "28/05/2026 09:15:33", "OPEN BUY 4 @ 273.60", "SNT.PL", "-1094.4"},
		{"", "2", "deposit", "28/05/2026 09:13:37", "Adyen BLIK deposit", "", "5000"},
		{"", ""},
		{"", "3", "Stock sale", "03/06/2026 09:49:56", "CLOSE BUY 1/2 @ 101.20", "NWG.PL", "109.2"},
		{"", "Total", "", "", "", "", "-1.71", "PLN"},
		{"", "4", "deposit", "01/01/2026 00:00:00", "after total, must be ignored", "", "1"},
	}

	ops, err := parseCashOps(rows, "50747414")
	if err != nil {
		t.Fatalf("parseCashOps returned error: %v", err)
	}
	if len(ops) != 3 {
		t.Fatalf("parseCashOps returned %d ops, want 3", len(ops))
	}

	purchase := ops[0]
	if purchase.Kind != model.CashOpStockPurchase {
		t.Errorf("kind = %q, want %q", purchase.Kind, model.CashOpStockPurchase)
	}
	if purchase.Volume != 4 || purchase.Price != 273.60 {
		t.Errorf("volume/price = %v/%v, want 4/273.6", purchase.Volume, purchase.Price)
	}
	if purchase.AccountID != "50747414" {
		t.Errorf("account id = %q, want %q", purchase.AccountID, "50747414")
	}
	wantTime := time.Date(2026, 5, 28, 9, 15, 33, 0, time.UTC)
	if !purchase.Time.Equal(wantTime) {
		t.Errorf("time = %s, want %s", purchase.Time, wantTime)
	}

	deposit := ops[1]
	if deposit.Volume != 0 || deposit.Price != 0 {
		t.Errorf("deposit got volume/price %v/%v, want 0/0", deposit.Volume, deposit.Price)
	}

	if ops[2].Kind != model.CashOpStockSale || ops[2].Volume != 1 {
		t.Errorf("sale = %+v, want stock_sale with volume 1", ops[2])
	}
}

func TestParseCashOpsFailsOnUnparseableTradeComment(t *testing.T) {
	rows := [][]string{
		{"", "ID", "Type", "Time", "Comment", "Symbol", "Amount"},
		{"", "1", "Stock purchase", "28/05/2026 09:15:33", "BOUGHT SOME SHARES", "SNT.PL", "-1094.4"},
	}

	if _, err := parseCashOps(rows, "1"); err == nil {
		t.Fatal("parseCashOps succeeded on an unrecognized trade comment, want error")
	}
}

func TestParseCashOpsWithoutTable(t *testing.T) {
	rows := [][]string{{"", "PENDING ORDERS HISTORY"}, {"", "27/06/2026 00:00:00"}}

	ops, err := parseCashOps(rows, "1")
	if err != nil {
		t.Fatalf("parseCashOps returned error: %v", err)
	}
	if ops != nil {
		t.Errorf("parseCashOps returned %v, want nil for a sheet with no table", ops)
	}
}
