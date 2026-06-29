package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

const insertPositionSQL = `
INSERT OR IGNORE INTO positions (
    provider, account_id, external_id, symbol, side, volume,
    open_time, open_price, close_time, close_price,
    purchase_value, sale_value, commission, swap, rollover, gross_pl, comment
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const insertCashOpSQL = `
INSERT OR IGNORE INTO cash_ops (
    provider, account_id, external_id, kind, raw_type,
    op_time, comment, symbol, amount, volume, price
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Duplicate is a record the database already held, skipped on import.
type Duplicate struct {
	Table      string
	ExternalID string
	Symbol     string
	When       time.Time
}

// String renders a duplicate for logging.
func (d Duplicate) String() string {
	symbol := d.Symbol
	if symbol == "" {
		symbol = "-"
	}
	return fmt.Sprintf("%s %s %s %s", d.Table, d.ExternalID, symbol, d.When.Format(time.DateOnly))
}

// ImportResult reports what one statement import changed.
type ImportResult struct {
	Provider   model.Provider
	AccountID  string
	Inserted   int
	Duplicates []Duplicate
}

// Import stores a statement, skipping records already present. Re-importing the
// same or an overlapping statement inserts nothing and reports the skipped
// records instead, which is what makes the command idempotent.
func Import(conn *sql.DB, statement model.Statement) (ImportResult, error) {
	result := ImportResult{Provider: statement.Provider, AccountID: statement.AccountID}

	tx, err := conn.Begin()
	if err != nil {
		return ImportResult{}, fmt.Errorf("failed to begin import transaction: %w", err)
	}
	defer tx.Rollback()

	inserted, duplicates, err := insertPositions(tx, statement.Positions)
	if err != nil {
		return ImportResult{}, err
	}
	result.Inserted += inserted
	result.Duplicates = append(result.Duplicates, duplicates...)

	inserted, duplicates, err = insertCashOps(tx, statement.CashOps)
	if err != nil {
		return ImportResult{}, err
	}
	result.Inserted += inserted
	result.Duplicates = append(result.Duplicates, duplicates...)

	if err := tx.Commit(); err != nil {
		return ImportResult{}, fmt.Errorf("failed to commit import: %w", err)
	}

	return result, nil
}

func insertPositions(tx *sql.Tx, positions []model.Position) (int, []Duplicate, error) {
	stmt, err := tx.Prepare(insertPositionSQL)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to prepare position insert: %w", err)
	}
	defer stmt.Close()

	var inserted int
	var duplicates []Duplicate
	for _, position := range positions {
		if err := position.Validate(); err != nil {
			return 0, nil, err
		}
		res, err := stmt.Exec(
			position.Provider, position.AccountID, position.ExternalID,
			position.Symbol, position.Side, position.Volume,
			formatTime(position.OpenTime), position.OpenPrice,
			nullTime(position.CloseTime), position.ClosePrice,
			position.PurchaseValue, position.SaleValue, position.Commission,
			position.Swap, position.Rollover, position.GrossPL, position.Comment,
		)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to insert position %s: %w", position.ExternalID, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read insert result for position %s: %w",
				position.ExternalID, err)
		}
		if affected == 0 {
			duplicates = append(duplicates, Duplicate{
				Table:      "position",
				ExternalID: position.ExternalID,
				Symbol:     position.Symbol,
				When:       position.OpenTime,
			})
			continue
		}
		inserted++
	}

	return inserted, duplicates, nil
}

func insertCashOps(tx *sql.Tx, cashOps []model.CashOp) (int, []Duplicate, error) {
	stmt, err := tx.Prepare(insertCashOpSQL)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to prepare cash op insert: %w", err)
	}
	defer stmt.Close()

	var inserted int
	var duplicates []Duplicate
	for _, op := range cashOps {
		if err := op.Validate(); err != nil {
			return 0, nil, err
		}
		res, err := stmt.Exec(
			op.Provider, op.AccountID, op.ExternalID, op.Kind, op.RawType,
			formatTime(op.Time), op.Comment, op.Symbol, op.Amount, op.Volume, op.Price,
		)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to insert cash op %s: %w", op.ExternalID, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to read insert result for cash op %s: %w",
				op.ExternalID, err)
		}
		if affected == 0 {
			duplicates = append(duplicates, Duplicate{
				Table:      "cash_op",
				ExternalID: op.ExternalID,
				Symbol:     op.Symbol,
				When:       op.Time,
			})
			continue
		}
		inserted++
	}

	return inserted, duplicates, nil
}
