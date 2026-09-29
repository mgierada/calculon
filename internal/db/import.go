package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// statementQuoteSource names the quotes taken from statement snapshots.
const statementQuoteSource = "statement"

// Changes counts what an import did to one table.
type Changes struct {
	Inserted  int
	Updated   int
	Unchanged int
}

func (c *Changes) add(o outcome) {
	switch o {
	case inserted:
		c.Inserted++
	case updated:
		c.Updated++
	case unchanged:
		c.Unchanged++
	}
}

// String renders the counts for logging.
func (c Changes) String() string {
	return fmt.Sprintf("%d new, %d updated, %d unchanged", c.Inserted, c.Updated, c.Unchanged)
}

// ImportResult reports what one statement import changed.
type ImportResult struct {
	Account   model.Account
	Positions Changes
	CashOps   Changes
	Quotes    Changes
	// OpenLots is the size of the stored snapshot when it was replaced.
	OpenLots int
	// SnapshotReplaced is false when a newer snapshot was already stored.
	SnapshotReplaced bool
}

// Import stores a statement for a user. History records are upserted: a known
// record whose content is unchanged is skipped, one whose content differs is
// updated. The open lot snapshot replaces the stored one unless that is newer.
// Everything happens in one transaction, so a bad record leaves nothing behind.
func Import(conn *sql.DB, userID int64, statement model.Statement) (ImportResult, error) {
	result := ImportResult{Account: statement.Account}

	tx, err := conn.Begin()
	if err != nil {
		return ImportResult{}, fmt.Errorf("failed to begin import transaction: %w", err)
	}
	defer tx.Rollback()

	storedAsOf, err := ensureAccount(tx, userID, statement.Account)
	if err != nil {
		return ImportResult{}, err
	}
	if err := upsertAll(tx, statement.Positions, &result.Positions,
		func(p model.Position) (row, error) { return positionRow(statement.Account, p) }); err != nil {
		return ImportResult{}, err
	}
	if err := upsertAll(tx, statement.CashOps, &result.CashOps,
		func(op model.CashOp) (row, error) { return cashOpRow(statement.Account, op) }); err != nil {
		return ImportResult{}, err
	}
	quotes := statementQuotes(statement)
	if err := upsertAll(tx, quotes, &result.Quotes, quoteRow); err != nil {
		return ImportResult{}, err
	}

	if storedAsOf.IsZero() || !statement.AsOf.Before(storedAsOf) {
		if err := replaceSnapshot(tx, statement); err != nil {
			return ImportResult{}, err
		}
		result.SnapshotReplaced = true
		result.OpenLots = len(statement.OpenLots)
	}

	if err := tx.Commit(); err != nil {
		return ImportResult{}, fmt.Errorf("failed to commit import: %w", err)
	}
	return result, nil
}

// ensureAccount registers the account for the user on first sight and returns
// when its stored snapshot was taken. An account already owned by someone else
// is refused, as is a currency change, which would corrupt every stored amount.
func ensureAccount(tx *sql.Tx, userID int64, account model.Account) (time.Time, error) {
	if err := account.Validate(); err != nil {
		return time.Time{}, err
	}

	var (
		ownerID  int64
		currency string
		asOf     sql.NullString
	)
	err := tx.QueryRow(`SELECT user_id, currency, snapshot_as_of FROM accounts
		WHERE provider = ? AND account_id = ?`, account.Provider, account.ID).
		Scan(&ownerID, &currency, &asOf)
	if errors.Is(err, sql.ErrNoRows) {
		_, err := tx.Exec(`INSERT INTO accounts (provider, account_id, user_id, currency)
			VALUES (?, ?, ?, ?)`, account.Provider, account.ID, userID, account.Currency)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to register account %s: %w", account.ID, err)
		}
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to look up account %s: %w", account.ID, err)
	}
	if ownerID != userID {
		return time.Time{}, fmt.Errorf("account %s belongs to another user", account.ID)
	}
	if currency != account.Currency {
		return time.Time{}, fmt.Errorf("account %s is stored in %s, statement says %s",
			account.ID, currency, account.Currency)
	}
	return parseTime(asOf)
}

// replaceSnapshot swaps the account's open lots for the statement's.
func replaceSnapshot(tx *sql.Tx, statement model.Statement) error {
	account := statement.Account
	if _, err := tx.Exec(`DELETE FROM open_lots WHERE provider = ? AND account_id = ?`,
		account.Provider, account.ID); err != nil {
		return fmt.Errorf("failed to clear open lots of %s: %w", account.ID, err)
	}
	for _, lot := range statement.OpenLots {
		lotRow, err := openLotRow(account, lot)
		if err != nil {
			return err
		}
		if err := insert(tx, lotRow); err != nil {
			return fmt.Errorf("failed to insert open lot %s: %w", lot.PositionID, err)
		}
	}
	if _, err := tx.Exec(`UPDATE accounts SET snapshot_as_of = ? WHERE provider = ? AND account_id = ?`,
		nullTime(statement.AsOf), account.Provider, account.ID); err != nil {
		return fmt.Errorf("failed to record snapshot time of %s: %w", account.ID, err)
	}
	return nil
}

// statementQuotes takes one quote per held symbol from the snapshot.
func statementQuotes(statement model.Statement) []model.Quote {
	if statement.AsOf.IsZero() {
		return nil
	}
	seen := map[string]bool{}
	var quotes []model.Quote
	for _, lot := range statement.OpenLots {
		if seen[lot.Symbol] || lot.CurrentPrice <= 0 {
			continue
		}
		seen[lot.Symbol] = true
		quotes = append(quotes, model.Quote{
			Symbol: lot.Symbol, AsOf: statement.AsOf, Price: lot.CurrentPrice,
			Source: statementQuoteSource,
		})
	}
	return quotes
}

// StoreQuotes upserts prices observed outside a statement, e.g. from a market
// data API.
func StoreQuotes(conn *sql.DB, quotes []model.Quote) (Changes, error) {
	var changes Changes
	tx, err := conn.Begin()
	if err != nil {
		return Changes{}, fmt.Errorf("failed to begin quote transaction: %w", err)
	}
	defer tx.Rollback()

	if err := upsertAll(tx, quotes, &changes, quoteRow); err != nil {
		return Changes{}, err
	}
	if err := tx.Commit(); err != nil {
		return Changes{}, fmt.Errorf("failed to commit quotes: %w", err)
	}
	return changes, nil
}

// upsertAll converts each record to a row and upserts it, tallying outcomes.
func upsertAll[T any](tx *sql.Tx, records []T, changes *Changes, toRow func(T) (row, error)) error {
	for _, record := range records {
		r, err := toRow(record)
		if err != nil {
			return err
		}
		o, err := upsert(tx, r)
		if err != nil {
			return err
		}
		changes.add(o)
	}
	return nil
}

func positionRow(account model.Account, p model.Position) (row, error) {
	if err := p.Validate(); err != nil {
		return row{}, err
	}
	return row{
		table: "positions",
		key:   accountKey(account, field{"row_key", p.Key()}),
		data: []field{
			{"position_id", p.PositionID}, {"seq", p.Seq},
			{"symbol", p.Symbol}, {"name", p.Name}, {"category", p.Category},
			{"product", p.Product}, {"side", p.Side}, {"volume", p.Volume},
			{"open_time", formatTime(p.OpenTime)}, {"open_price", p.OpenPrice},
			{"close_time", formatTime(p.CloseTime)}, {"close_price", p.ClosePrice},
			{"purchase_value", p.PurchaseValue}, {"sale_value", p.SaleValue},
			{"commission", p.Commission}, {"swap", p.Swap}, {"rollover", p.Rollover},
			{"gross_pl", p.GrossPL}, {"net_pl", p.NetPL},
			{"close_origin", p.CloseOrigin}, {"comment", p.Comment},
		},
	}, nil
}

func cashOpRow(account model.Account, op model.CashOp) (row, error) {
	if err := op.Validate(); err != nil {
		return row{}, err
	}
	return row{
		table: "cash_ops",
		key:   accountKey(account, field{"external_id", op.ExternalID}),
		data: []field{
			{"kind", op.Kind}, {"raw_type", op.RawType}, {"op_time", formatTime(op.Time)},
			{"symbol", op.Symbol}, {"name", op.Name}, {"category", op.Category},
			{"product", op.Product}, {"position_id", op.PositionID}, {"comment", op.Comment},
			{"amount", op.Amount}, {"volume", op.Volume}, {"price", op.Price},
		},
	}, nil
}

func openLotRow(account model.Account, lot model.OpenLot) (row, error) {
	if err := lot.Validate(); err != nil {
		return row{}, err
	}
	return row{
		table: "open_lots",
		key:   accountKey(account, field{"row_key", lot.Key()}),
		data: []field{
			{"position_id", lot.PositionID}, {"seq", lot.Seq},
			{"symbol", lot.Symbol}, {"name", lot.Name}, {"category", lot.Category},
			{"product", lot.Product}, {"side", lot.Side}, {"volume", lot.Volume},
			{"open_time", formatTime(lot.OpenTime)}, {"open_price", lot.OpenPrice},
			{"current_price", lot.CurrentPrice}, {"value", lot.Value},
			{"gross_pl", lot.GrossPL}, {"net_pl", lot.NetPL},
			{"commission", lot.Commission}, {"swap", lot.Swap},
		},
	}, nil
}

func quoteRow(q model.Quote) (row, error) {
	if q.Symbol == "" || q.AsOf.IsZero() || q.Price <= 0 {
		return row{}, fmt.Errorf("invalid quote %+v", q)
	}
	return row{
		table: "quotes",
		key:   []field{{"symbol", q.Symbol}, {"as_of", formatTime(q.AsOf)}},
		data:  []field{{"price", q.Price}, {"source", q.Source}},
	}, nil
}

func accountKey(account model.Account, recordKey field) []field {
	return []field{{"provider", account.Provider}, {"account_id", account.ID}, recordKey}
}

// field is one column and the value stored in it.
type field struct {
	name  string
	value any
}

// row is a record ready for storage: the columns that identify it and the
// columns that describe it, which may change between imports.
type row struct {
	table string
	key   []field
	data  []field
}

type outcome int

const (
	inserted outcome = iota
	updated
	unchanged
)

// upsert stores a history row, comparing content hashes to decide whether a
// known row needs updating.
func upsert(tx *sql.Tx, r row) (outcome, error) {
	hash := contentHash(r.data)
	where, keyArgs := whereKey(r.key)

	var stored string
	err := tx.QueryRow("SELECT content_hash FROM "+r.table+" WHERE "+where, keyArgs...).Scan(&stored)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		withHash := r
		withHash.data = append(append([]field{}, r.data...), field{"content_hash", hash})
		if err := insert(tx, withHash); err != nil {
			return 0, fmt.Errorf("failed to insert into %s: %w", r.table, err)
		}
		return inserted, nil
	case err != nil:
		return 0, fmt.Errorf("failed to look up %s row: %w", r.table, err)
	case stored == hash:
		return unchanged, nil
	}

	sets := make([]string, 0, len(r.data)+1)
	args := make([]any, 0, len(r.data)+1+len(keyArgs))
	for _, f := range append(r.data, field{"content_hash", hash}) {
		sets = append(sets, f.name+" = ?")
		args = append(args, f.value)
	}
	query := "UPDATE " + r.table + " SET " + strings.Join(sets, ", ") + " WHERE " + where
	if _, err := tx.Exec(query, append(args, keyArgs...)...); err != nil {
		return 0, fmt.Errorf("failed to update %s row: %w", r.table, err)
	}
	return updated, nil
}

// insert writes every key and data column of a row.
func insert(tx *sql.Tx, r row) error {
	fields := append(append([]field{}, r.key...), r.data...)
	names := make([]string, len(fields))
	args := make([]any, len(fields))
	for i, f := range fields {
		names[i], args[i] = f.name, f.value
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(fields)), ", ")
	query := "INSERT INTO " + r.table + " (" + strings.Join(names, ", ") + ") VALUES (" + placeholders + ")"
	_, err := tx.Exec(query, args...)
	return err
}

func whereKey(key []field) (string, []any) {
	conditions := make([]string, len(key))
	args := make([]any, len(key))
	for i, f := range key {
		conditions[i], args[i] = f.name+" = ?", f.value
	}
	return strings.Join(conditions, " AND "), args
}

// contentHash fingerprints a row's data columns.
func contentHash(data []field) string {
	h := sha256.New()
	for _, f := range data {
		fmt.Fprintf(h, "%s=%v\x1f", f.name, f.value)
	}
	return hex.EncodeToString(h.Sum(nil))
}
