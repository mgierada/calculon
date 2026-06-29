package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// openTestDB returns a connection to a fresh database on disk. A file rather
// than :memory: so the schema survives the pool handing out a second connection.
func openTestDB(t *testing.T) *Conn {
	t.Helper()

	conn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return conn
}

func testPosition(externalID, symbol string) model.Position {
	return model.Position{
		Provider:   model.ProviderXTB,
		AccountID:  "50747414",
		ExternalID: externalID,
		Symbol:     symbol,
		Side:       model.SideBuy,
		Volume:     1,
		OpenTime:   time.Date(2026, 5, 4, 12, 40, 36, 0, time.UTC),
		OpenPrice:  109.20,
	}
}

func testCashOp(externalID, symbol string, kind model.CashOpKind, volume, price float64) model.CashOp {
	return model.CashOp{
		Provider:   model.ProviderXTB,
		AccountID:  "50747414",
		ExternalID: externalID,
		Kind:       kind,
		RawType:    string(kind),
		Time:       time.Date(2026, 5, 28, 9, 15, 33, 0, time.UTC),
		Symbol:     symbol,
		Volume:     volume,
		Price:      price,
	}
}

func TestImportIsIdempotent(t *testing.T) {
	conn := openTestDB(t)
	statement := model.Statement{
		Provider:  model.ProviderXTB,
		AccountID: "50747414",
		Positions: []model.Position{testPosition("p1", "NWG.PL"), testPosition("p2", "NWG.PL")},
		CashOps: []model.CashOp{
			testCashOp("c1", "SNT.PL", model.CashOpStockPurchase, 4, 273.60),
			testCashOp("c2", "", model.CashOpDeposit, 0, 0),
		},
	}

	first, err := Import(conn, statement)
	if err != nil {
		t.Fatalf("first Import returned error: %v", err)
	}
	if first.Inserted != 4 {
		t.Errorf("first import inserted %d records, want 4", first.Inserted)
	}
	if len(first.Duplicates) != 0 {
		t.Errorf("first import reported duplicates %v, want none", first.Duplicates)
	}

	second, err := Import(conn, statement)
	if err != nil {
		t.Fatalf("second Import returned error: %v", err)
	}
	if second.Inserted != 0 {
		t.Errorf("second import inserted %d records, want 0", second.Inserted)
	}
	if len(second.Duplicates) != 4 {
		t.Fatalf("second import reported %d duplicates, want 4", len(second.Duplicates))
	}

	wantIDs := map[string]bool{"p1": true, "p2": true, "c1": true, "c2": true}
	for _, duplicate := range second.Duplicates {
		if !wantIDs[duplicate.ExternalID] {
			t.Errorf("unexpected duplicate %s", duplicate)
		}
		delete(wantIDs, duplicate.ExternalID)
	}
	if len(wantIDs) != 0 {
		t.Errorf("duplicates missing for %v", wantIDs)
	}
}

// An overlapping statement re-lists records already held and adds the new ones.
func TestImportOverlappingStatement(t *testing.T) {
	conn := openTestDB(t)

	if _, err := Import(conn, model.Statement{
		Positions: []model.Position{testPosition("p1", "NWG.PL")},
	}); err != nil {
		t.Fatalf("first Import returned error: %v", err)
	}

	result, err := Import(conn, model.Statement{
		Positions: []model.Position{testPosition("p1", "NWG.PL"), testPosition("p2", "CDR.PL")},
	})
	if err != nil {
		t.Fatalf("second Import returned error: %v", err)
	}
	if result.Inserted != 1 {
		t.Errorf("inserted %d records, want 1", result.Inserted)
	}
	if len(result.Duplicates) != 1 || result.Duplicates[0].ExternalID != "p1" {
		t.Errorf("duplicates = %v, want just p1", result.Duplicates)
	}
}

// Accounts are part of the natural key, so the same symbol in two accounts is
// two records rather than a duplicate.
func TestImportSeparatesAccounts(t *testing.T) {
	conn := openTestDB(t)

	first := testPosition("shared-id", "SNT.PL")
	second := first
	second.AccountID = "51099570"

	result, err := Import(conn, model.Statement{Positions: []model.Position{first, second}})
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	if result.Inserted != 2 {
		t.Errorf("inserted %d records, want 2", result.Inserted)
	}
}

func TestImportRejectsInvalidRecords(t *testing.T) {
	conn := openTestDB(t)

	invalid := testPosition("p1", "")
	if _, err := Import(conn, model.Statement{Positions: []model.Position{invalid}}); err == nil {
		t.Fatal("Import accepted a position with no symbol, want error")
	}

	// The failed import must have rolled back rather than leaving half a statement.
	holdings, err := OpenHoldings(conn)
	if err != nil {
		t.Fatalf("OpenHoldings returned error: %v", err)
	}
	if len(holdings) != 0 {
		t.Errorf("holdings after failed import = %v, want none", holdings)
	}
}
