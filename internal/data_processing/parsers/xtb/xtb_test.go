package xtb

import "testing"

func TestFindAccountID(t *testing.T) {
	sheets := map[string][][]string{
		"CASH OPERATION HISTORY": {
			{},
			{"", "", "", "Name and surname", "Account", "Currency", "", "27/06/2026 17:43:47"},
			{"", "", "", "Taylor Swift", "50747414", "PLN"},
		},
	}

	accountID, err := findAccountID(sheets)
	if err != nil {
		t.Fatalf("findAccountID returned error: %v", err)
	}
	if accountID != "50747414" {
		t.Errorf("findAccountID = %q, want %q", accountID, "50747414")
	}
}

// Some XTB exports leave the name blank but still fill the account column.
func TestFindAccountIDWithBlankNeighbours(t *testing.T) {
	sheets := map[string][][]string{
		"CLOSED POSITION HISTORY": {
			{"", "", "", "", "", "Name and surname", "", "", "Account", "", "", "Currency"},
			{"", "", "", "", "", "", "", "", "51099570"},
		},
	}

	accountID, err := findAccountID(sheets)
	if err != nil {
		t.Fatalf("findAccountID returned error: %v", err)
	}
	if accountID != "51099570" {
		t.Errorf("findAccountID = %q, want %q", accountID, "51099570")
	}
}

func TestFindAccountIDMissing(t *testing.T) {
	sheets := map[string][][]string{"SHEET": {{"", "no header block here"}}}

	if _, err := findAccountID(sheets); err == nil {
		t.Fatal("findAccountID succeeded without an account label, want error")
	}
}

func TestFindSheetMatchesDatedName(t *testing.T) {
	sheets := map[string][][]string{
		"OPEN POSITION 27062026": {{"open"}},
		"CASH OPERATION HISTORY": {{"cash"}},
	}

	if _, ok := findSheet(sheets, sheetOpenPositions); !ok {
		t.Error("findSheet did not match the dated open position sheet")
	}
	if _, ok := findSheet(sheets, sheetCashOps); !ok {
		t.Error("findSheet did not match the cash operation sheet")
	}
	if _, ok := findSheet(sheets, sheetClosedPositions); ok {
		t.Error("findSheet matched a sheet that is not present")
	}
}
