package db

import (
	"database/sql"
	"fmt"

	"github.com/mgierada/calculon/internal/model"
)

// Every read below is scoped to one user through the accounts they own, so a
// session can never see another user's records.

const accountsSQL = `
SELECT provider, account_id, currency, snapshot_as_of
FROM accounts WHERE user_id = ? ORDER BY provider, account_id`

const openLotsSQL = `
SELECT a.provider, a.account_id, a.currency,
       l.position_id, l.seq, l.symbol, l.name, l.category, l.product, l.side, l.volume,
       l.open_time, l.open_price, l.current_price, l.value, l.gross_pl, l.net_pl,
       l.commission, l.swap
FROM open_lots l
JOIN accounts a ON a.provider = l.provider AND a.account_id = l.account_id
WHERE a.user_id = ?
ORDER BY l.symbol, l.open_time`

const positionsSQL = `
SELECT a.provider, a.account_id, a.currency,
       p.position_id, p.seq, p.symbol, p.name, p.category, p.product, p.side, p.volume,
       p.open_time, p.open_price, p.close_time, p.close_price, p.purchase_value,
       p.sale_value, p.commission, p.swap, p.rollover, p.gross_pl, p.net_pl,
       p.close_origin, p.comment
FROM positions p
JOIN accounts a ON a.provider = p.provider AND a.account_id = p.account_id
WHERE a.user_id = ?
ORDER BY p.close_time DESC, p.position_id`

const cashOpsSQL = `
SELECT a.provider, a.account_id, a.currency,
       c.external_id, c.kind, c.raw_type, c.op_time, c.symbol, c.name, c.category,
       c.product, c.position_id, c.comment, c.amount, c.volume, c.price
FROM cash_ops c
JOIN accounts a ON a.provider = c.provider AND a.account_id = c.account_id
WHERE a.user_id = ?
ORDER BY c.op_time DESC, c.external_id`

// quotesSQL reads the quotes of every symbol the user holds or has traded.
// Quotes are shared market data, not owned, so scoping is by symbol.
const quotesSQL = `
SELECT symbol, as_of, price, source FROM quotes
WHERE symbol IN (
    SELECT l.symbol FROM open_lots l
    JOIN accounts a ON a.provider = l.provider AND a.account_id = l.account_id
    WHERE a.user_id = ?1
    UNION
    SELECT p.symbol FROM positions p
    JOIN accounts a ON a.provider = p.provider AND a.account_id = p.account_id
    WHERE a.user_id = ?1
    UNION
    SELECT c.symbol FROM cash_ops c
    JOIN accounts a ON a.provider = c.provider AND a.account_id = c.account_id
    WHERE a.user_id = ?1 AND c.symbol != ''
)
ORDER BY symbol, as_of`

// Accounts lists the user's accounts with their snapshot times.
func Accounts(conn *sql.DB, userID int64) ([]model.AccountSnapshot, error) {
	return query(conn, accountsSQL, userID, "accounts", func(rows *sql.Rows) (model.AccountSnapshot, error) {
		var (
			account model.AccountSnapshot
			asOf    sql.NullString
		)
		if err := rows.Scan(&account.Provider, &account.ID, &account.Currency, &asOf); err != nil {
			return model.AccountSnapshot{}, err
		}
		var err error
		account.AsOf, err = parseTime(asOf)
		return account, err
	})
}

// OpenLots lists the user's open lots from each account's latest snapshot.
func OpenLots(conn *sql.DB, userID int64) ([]model.Owned[model.OpenLot], error) {
	return query(conn, openLotsSQL, userID, "open lots", func(rows *sql.Rows) (model.Owned[model.OpenLot], error) {
		var (
			owned    model.Owned[model.OpenLot]
			openTime string
		)
		lot := &owned.Record
		err := rows.Scan(
			&owned.Account.Provider, &owned.Account.ID, &owned.Account.Currency,
			&lot.PositionID, &lot.Seq, &lot.Symbol, &lot.Name, &lot.Category, &lot.Product,
			&lot.Side, &lot.Volume, &openTime, &lot.OpenPrice, &lot.CurrentPrice, &lot.Value,
			&lot.GrossPL, &lot.NetPL, &lot.Commission, &lot.Swap,
		)
		if err != nil {
			return owned, err
		}
		lot.OpenTime, err = parseRequiredTime(openTime)
		return owned, err
	})
}

// ClosedPositions lists the user's closed trades, most recently closed first.
func ClosedPositions(conn *sql.DB, userID int64) ([]model.Owned[model.Position], error) {
	return query(conn, positionsSQL, userID, "positions", func(rows *sql.Rows) (model.Owned[model.Position], error) {
		var (
			owned               model.Owned[model.Position]
			openTime, closeTime string
		)
		p := &owned.Record
		err := rows.Scan(
			&owned.Account.Provider, &owned.Account.ID, &owned.Account.Currency,
			&p.PositionID, &p.Seq, &p.Symbol, &p.Name, &p.Category, &p.Product, &p.Side,
			&p.Volume, &openTime, &p.OpenPrice, &closeTime, &p.ClosePrice, &p.PurchaseValue,
			&p.SaleValue, &p.Commission, &p.Swap, &p.Rollover, &p.GrossPL, &p.NetPL,
			&p.CloseOrigin, &p.Comment,
		)
		if err != nil {
			return owned, err
		}
		if p.OpenTime, err = parseRequiredTime(openTime); err != nil {
			return owned, err
		}
		p.CloseTime, err = parseRequiredTime(closeTime)
		return owned, err
	})
}

// CashOps lists the user's cash ledger, newest first.
func CashOps(conn *sql.DB, userID int64) ([]model.Owned[model.CashOp], error) {
	return query(conn, cashOpsSQL, userID, "cash ops", func(rows *sql.Rows) (model.Owned[model.CashOp], error) {
		var (
			owned  model.Owned[model.CashOp]
			opTime string
		)
		op := &owned.Record
		err := rows.Scan(
			&owned.Account.Provider, &owned.Account.ID, &owned.Account.Currency,
			&op.ExternalID, &op.Kind, &op.RawType, &opTime, &op.Symbol, &op.Name, &op.Category,
			&op.Product, &op.PositionID, &op.Comment, &op.Amount, &op.Volume, &op.Price,
		)
		if err != nil {
			return owned, err
		}
		op.Time, err = parseRequiredTime(opTime)
		return owned, err
	})
}

// Quotes lists every stored price of the symbols the user holds or has traded,
// oldest first per symbol.
func Quotes(conn *sql.DB, userID int64) ([]model.Quote, error) {
	return query(conn, quotesSQL, userID, "quotes", func(rows *sql.Rows) (model.Quote, error) {
		var (
			quote model.Quote
			asOf  string
		)
		if err := rows.Scan(&quote.Symbol, &asOf, &quote.Price, &quote.Source); err != nil {
			return quote, err
		}
		var err error
		quote.AsOf, err = parseRequiredTime(asOf)
		return quote, err
	})
}

// query runs a user-scoped read and scans every row.
func query[T any](conn *sql.DB, sqlText string, userID int64, what string,
	scan func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := conn.Query(sqlText, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s: %w", what, err)
	}
	defer rows.Close()

	var records []T
	for rows.Next() {
		record, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", what, err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", what, err)
	}
	return records, nil
}
