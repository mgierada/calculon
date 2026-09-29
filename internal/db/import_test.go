package db

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

var (
	testAccount = model.Account{Provider: model.ProviderXTB, ID: "50747414", Currency: "PLN"}
	testAsOf    = time.Date(2026, 9, 28, 9, 17, 56, 0, time.UTC)
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

func createTestUser(t *testing.T, conn *Conn, name string) User {
	t.Helper()

	user, err := CreateUser(conn, name)
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	return user
}

func testPosition(positionID string, volume float64) model.Position {
	return model.Position{
		Instrument: model.Instrument{Symbol: "NWG.PL", Name: "Newag", Category: "STOCK"},
		PositionID: positionID,
		Side:       model.SideBuy,
		Volume:     volume,
		OpenTime:   time.Date(2026, 5, 4, 10, 40, 36, 0, time.UTC),
		OpenPrice:  109.20,
		CloseTime:  time.Date(2026, 6, 3, 7, 49, 56, 0, time.UTC),
		ClosePrice: 101.20,
		NetPL:      -8 * volume,
	}
}

func testCashOp(externalID string, amount float64) model.CashOp {
	return model.CashOp{
		ExternalID: externalID,
		Kind:       model.CashOpDeposit,
		RawType:    "Deposit",
		Time:       time.Date(2026, 5, 28, 9, 15, 33, 0, time.UTC),
		Amount:     amount,
	}
}

func testLot(positionID, symbol string, price float64) model.OpenLot {
	return model.OpenLot{
		Instrument:   model.Instrument{Symbol: symbol, Name: symbol, Category: "STOCK"},
		PositionID:   positionID,
		Side:         model.SideBuy,
		Volume:       5,
		OpenTime:     time.Date(2026, 5, 27, 13, 24, 27, 0, time.UTC),
		OpenPrice:    274.6,
		CurrentPrice: price,
		Value:        5 * price,
	}
}

func testStatement() model.Statement {
	return model.Statement{
		Account: testAccount,
		AsOf:    testAsOf,
		Positions: []model.Position{
			testPosition("p1", 1),
			// A partial close of the same position is its own record.
			testPosition("p1", 2),
		},
		CashOps:  []model.CashOp{testCashOp("c1", 2000), testCashOp("c2", 500)},
		OpenLots: []model.OpenLot{testLot("l1", "SNT.PL", 346.4), testLot("l2", "SNT.PL", 346.4)},
	}
}

func mustImport(t *testing.T, conn *Conn, userID int64, statement model.Statement) ImportResult {
	t.Helper()

	result, err := Import(conn, userID, statement)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}
	return result
}

func TestImportIsIdempotent(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")

	first := mustImport(t, conn, user.ID, testStatement())
	if first.Positions != (Changes{Inserted: 2}) || first.CashOps != (Changes{Inserted: 2}) {
		t.Errorf("first import = positions %s, cash ops %s", first.Positions, first.CashOps)
	}
	if first.Quotes != (Changes{Inserted: 1}) {
		t.Errorf("first import quotes = %s, want one per symbol", first.Quotes)
	}

	second := mustImport(t, conn, user.ID, testStatement())
	if second.Positions != (Changes{Unchanged: 2}) || second.CashOps != (Changes{Unchanged: 2}) {
		t.Errorf("second import = positions %s, cash ops %s", second.Positions, second.CashOps)
	}
}

func TestImportUpdatesChangedRecords(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	mustImport(t, conn, user.ID, testStatement())

	corrected := testStatement()
	corrected.CashOps[0].Comment = "corrected by broker"
	corrected.Positions[1].NetPL = -99
	result := mustImport(t, conn, user.ID, corrected)

	if result.CashOps != (Changes{Updated: 1, Unchanged: 1}) {
		t.Errorf("cash ops = %s, want 1 updated", result.CashOps)
	}
	if result.Positions != (Changes{Updated: 1, Unchanged: 1}) {
		t.Errorf("positions = %s, want 1 updated", result.Positions)
	}

	positions, err := ClosedPositions(conn, user.ID)
	if err != nil {
		t.Fatalf("ClosedPositions returned error: %v", err)
	}
	var netPLs []float64
	for _, p := range positions {
		netPLs = append(netPLs, p.Record.NetPL)
	}
	if len(netPLs) != 2 || (netPLs[0] != -99 && netPLs[1] != -99) {
		t.Errorf("net P/Ls after update = %v, want one of them -99", netPLs)
	}
}

func TestImportReplacesSnapshotOnlyWhenNewer(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	mustImport(t, conn, user.ID, testStatement())

	newer := testStatement()
	newer.AsOf = testAsOf.Add(24 * time.Hour)
	newer.OpenLots = []model.OpenLot{testLot("l1", "SNT.PL", 350), testLot("l3", "CDR.PL", 250)}
	if result := mustImport(t, conn, user.ID, newer); !result.SnapshotReplaced {
		t.Error("newer statement did not replace the snapshot")
	}

	older := testStatement()
	older.AsOf = testAsOf.Add(-24 * time.Hour)
	if result := mustImport(t, conn, user.ID, older); result.SnapshotReplaced {
		t.Error("older statement replaced the snapshot")
	}

	lots, err := OpenLots(conn, user.ID)
	if err != nil {
		t.Fatalf("OpenLots returned error: %v", err)
	}
	if len(lots) != 2 || lots[0].Record.PositionID != "l3" || lots[1].Record.CurrentPrice != 350 {
		t.Errorf("open lots = %+v, want l3 and l1 from the newest snapshot", lots)
	}

	accounts, err := Accounts(conn, user.ID)
	if err != nil {
		t.Fatalf("Accounts returned error: %v", err)
	}
	if len(accounts) != 1 || !accounts[0].AsOf.Equal(newer.AsOf) {
		t.Errorf("accounts = %+v, want snapshot time %s", accounts, newer.AsOf)
	}

	// Quotes from the older statement are history and still kept.
	quotes, err := Quotes(conn, user.ID)
	if err != nil {
		t.Fatalf("Quotes returned error: %v", err)
	}
	if len(quotes) != 4 {
		t.Errorf("stored %d quotes, want 4: %+v", len(quotes), quotes)
	}
}

func TestImportScopesRecordsToTheirOwner(t *testing.T) {
	conn := openTestDB(t)
	alice := createTestUser(t, conn, "alice")
	bob := createTestUser(t, conn, "bob")
	mustImport(t, conn, alice.ID, testStatement())

	if _, err := Import(conn, bob.ID, testStatement()); err == nil {
		t.Fatal("bob imported alice's account, want error")
	}

	lots, err := OpenLots(conn, bob.ID)
	if err != nil {
		t.Fatalf("OpenLots returned error: %v", err)
	}
	cashOps, err := CashOps(conn, bob.ID)
	if err != nil {
		t.Fatalf("CashOps returned error: %v", err)
	}
	quotes, err := Quotes(conn, bob.ID)
	if err != nil {
		t.Fatalf("Quotes returned error: %v", err)
	}
	if len(lots)+len(cashOps)+len(quotes) != 0 {
		t.Errorf("bob sees %d lots, %d cash ops, %d quotes, want none",
			len(lots), len(cashOps), len(quotes))
	}
}

func TestImportRejectsCurrencyChange(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	mustImport(t, conn, user.ID, testStatement())

	statement := testStatement()
	statement.Account.Currency = "USD"
	if _, err := Import(conn, user.ID, statement); err == nil {
		t.Fatal("Import accepted a currency change")
	}
}

func TestImportRollsBackInvalidStatement(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")

	statement := testStatement()
	statement.CashOps[1].ExternalID = ""
	if _, err := Import(conn, user.ID, statement); err == nil {
		t.Fatal("Import accepted a cash op without an id")
	}

	positions, err := ClosedPositions(conn, user.ID)
	if err != nil {
		t.Fatalf("ClosedPositions returned error: %v", err)
	}
	accounts, err := Accounts(conn, user.ID)
	if err != nil {
		t.Fatalf("Accounts returned error: %v", err)
	}
	if len(positions) != 0 || len(accounts) != 0 {
		t.Errorf("failed import left %d positions and %d accounts", len(positions), len(accounts))
	}
}

func TestReadsRoundTripRecords(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	statement := testStatement()
	mustImport(t, conn, user.ID, statement)

	positions, err := ClosedPositions(conn, user.ID)
	if err != nil {
		t.Fatalf("ClosedPositions returned error: %v", err)
	}
	got := positions[0]
	if got.Account != testAccount {
		t.Errorf("position account = %+v, want %+v", got.Account, testAccount)
	}
	want := statement.Positions[0]
	if got.Record.Key() != want.Key() && got.Record.Key() != statement.Positions[1].Key() {
		t.Errorf("position key %q matches neither imported position", got.Record.Key())
	}
	if !got.Record.CloseTime.Equal(want.CloseTime) || got.Record.Name != "Newag" {
		t.Errorf("position = %+v", got.Record)
	}

	cashOps, err := CashOps(conn, user.ID)
	if err != nil {
		t.Fatalf("CashOps returned error: %v", err)
	}
	if len(cashOps) != 2 || cashOps[0].Record.Kind != model.CashOpDeposit {
		t.Errorf("cash ops = %+v", cashOps)
	}
}

func TestStoreQuotes(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	mustImport(t, conn, user.ID, testStatement())

	quote := model.Quote{Symbol: "SNT.PL", AsOf: testAsOf.Add(time.Hour), Price: 350, Source: "api"}
	changes, err := StoreQuotes(conn, []model.Quote{quote})
	if err != nil {
		t.Fatalf("StoreQuotes returned error: %v", err)
	}
	if changes != (Changes{Inserted: 1}) {
		t.Errorf("changes = %s, want 1 new", changes)
	}
}

func TestMigrateRefusesOldSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE positions (provider TEXT)`); err != nil {
		t.Fatalf("creating old table: %v", err)
	}
	raw.Close()

	if _, err := Open(path); !errors.Is(err, ErrIncompatibleSchema) {
		t.Fatalf("Open on an old database returned %v, want ErrIncompatibleSchema", err)
	}

	// Refusing the file must leave it as it was, not switched to WAL.
	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	defer raw.Close()
	var mode string
	if err := raw.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("reading journal mode: %v", err)
	}
	if mode != "delete" {
		t.Errorf("journal mode after refusal = %q, want the untouched default", mode)
	}
}

func TestMigrateUpgradesVersion2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open returned error: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL)`,
		`CREATE TABLE accounts (provider TEXT NOT NULL, account_id TEXT NOT NULL,
			user_id INTEGER NOT NULL, currency TEXT NOT NULL, snapshot_as_of TEXT,
			PRIMARY KEY (provider, account_id))`,
		`INSERT INTO users VALUES (1, 'alice', '2026-01-01T00:00:00Z')`,
		`INSERT INTO accounts VALUES ('xtb', '50747414', 1, 'PLN', NULL)`,
		`PRAGMA user_version = 2`,
	} {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("building v2 database: %v", err)
		}
	}
	raw.Close()

	conn, err := Open(path)
	if err != nil {
		t.Fatalf("Open on a v2 database returned error: %v", err)
	}
	defer conn.Close()

	accounts, err := Accounts(conn, 1)
	if err != nil {
		t.Fatalf("Accounts returned error: %v", err)
	}
	if len(accounts) != 1 || accounts[0].ID != "50747414" || accounts[0].Name != "" {
		t.Errorf("accounts after migration = %+v, want the old row with an empty name", accounts)
	}
}

func TestImportNamesAccountButKeepsRenames(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	statement := testStatement()
	statement.Account.Name = "PLN"
	mustImport(t, conn, user.ID, statement)

	if err := RenameAccount(conn, user.ID, model.ProviderXTB, testAccount.ID, "main"); err != nil {
		t.Fatalf("RenameAccount returned error: %v", err)
	}
	mustImport(t, conn, user.ID, statement)

	accounts, err := Accounts(conn, user.ID)
	if err != nil {
		t.Fatalf("Accounts returned error: %v", err)
	}
	if accounts[0].Name != "main" {
		t.Errorf("name after re-import = %q, want the rename kept", accounts[0].Name)
	}
	lots, err := OpenLots(conn, user.ID)
	if err != nil {
		t.Fatalf("OpenLots returned error: %v", err)
	}
	if lots[0].Account.Name != "main" {
		t.Errorf("lot account name = %q, want it read with the record", lots[0].Account.Name)
	}
}

func TestRenameAccountIsScopedToOwner(t *testing.T) {
	conn := openTestDB(t)
	alice := createTestUser(t, conn, "alice")
	bob := createTestUser(t, conn, "bob")
	mustImport(t, conn, alice.ID, testStatement())

	if err := RenameAccount(conn, bob.ID, model.ProviderXTB, testAccount.ID, "mine"); err == nil {
		t.Fatal("bob renamed alice's account")
	}
}
