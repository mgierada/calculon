package db

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
)

// Earnings are shared market data, like quotes: reads span every user.

// StoreEarnings records one page of a symbol's earnings, overwriting periods
// stored before. withTarget also records the page's price targets, which
// every page repeats, so a paged fetch keeps them once.
func StoreEarnings(conn *sql.DB, symbol string, fetchedAt time.Time, result finimpulse.EarningsResult,
	withTarget bool) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin earnings transaction: %w", err)
	}
	defer tx.Rollback()

	fetched := formatTime(fetchedAt)
	if withTarget {
		if _, err := tx.Exec(`INSERT INTO earnings_target (symbol, fetched_at, target_price,
			target_average_price, target_low_price, target_high_price, total_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)`, symbol, fetched, result.TargetPrice,
			result.TargetAveragePrice, result.TargetLowPrice, result.TargetHighPrice,
			result.TotalCount); err != nil {
			return fmt.Errorf("failed to store price targets of %s: %w", symbol, err)
		}
	}
	for _, g := range result.Growth {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO earnings_growth (symbol, date, date_type,
			growth, growth_benchmark, symbol_benchmark, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			symbol, g.Date, g.DateType, g.Growth, g.GrowthBenchmark, g.SymbolBenchmark,
			fetched); err != nil {
			return fmt.Errorf("failed to store growth of %s: %w", symbol, err)
		}
	}
	for _, e := range result.EPS {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO earnings_eps (symbol, date, date_type,
			methodology, actual, estimate, surprise, surprise_pct, fetched_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, symbol, e.Date, e.DateType, e.Methodology,
			e.Actual, e.Estimate, e.Surprise, e.SurprisePct, fetched); err != nil {
			return fmt.Errorf("failed to store EPS of %s: %w", symbol, err)
		}
	}
	for _, r := range result.Revenue {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO earnings_revenue (symbol, date, date_type,
			methodology, revenue, earnings, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			symbol, r.Date, r.DateType, r.Methodology, r.Revenue, r.Earnings, fetched); err != nil {
			return fmt.Errorf("failed to store revenue of %s: %w", symbol, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit earnings of %s: %w", symbol, err)
	}
	return nil
}

// EarningsOf reads a symbol's stored earnings in one methodology, false when
// it was never fetched.
func EarningsOf(conn *sql.DB, symbol, methodology string) (model.Earnings, bool, error) {
	e := model.Earnings{Symbol: symbol}
	var fetchedAt string
	err := conn.QueryRow(`SELECT fetched_at, target_price, target_average_price,
		target_low_price, target_high_price, total_count FROM earnings_target
		WHERE symbol = ? ORDER BY fetched_at DESC, id DESC LIMIT 1`, symbol).
		Scan(&fetchedAt, &e.Target.Price, &e.Target.Average, &e.Target.Low, &e.Target.High, &e.TotalCount)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Earnings{}, false, nil
	case err != nil:
		return model.Earnings{}, false, fmt.Errorf("failed to read price targets of %s: %w", symbol, err)
	}
	if e.FetchedAt, err = parseRequiredTime(fetchedAt); err != nil {
		return model.Earnings{}, false, err
	}
	if e.Currency, err = quoteCurrency(conn, symbol); err != nil {
		return model.Earnings{}, false, err
	}
	if e.Revenue, err = earningsRows(conn, `SELECT date, date_type, revenue, earnings
		FROM earnings_revenue WHERE symbol = ? AND methodology = ? ORDER BY date`,
		func(rows *sql.Rows, r *model.EarningsRevenue, date *string) error {
			return rows.Scan(date, &r.Length, &r.Revenue, &r.Earnings)
		}, func(r *model.EarningsRevenue) *model.EarningsPeriod { return &r.EarningsPeriod },
		symbol, methodology); err != nil {
		return model.Earnings{}, false, err
	}
	if e.EPS, err = earningsRows(conn, `SELECT date, date_type, actual, estimate, surprise,
		surprise_pct FROM earnings_eps WHERE symbol = ? AND methodology = ? ORDER BY date`,
		func(rows *sql.Rows, r *model.EarningsEPS, date *string) error {
			return rows.Scan(date, &r.Length, &r.Actual, &r.Estimate, &r.Surprise, &r.SurprisePct)
		}, func(r *model.EarningsEPS) *model.EarningsPeriod { return &r.EarningsPeriod },
		symbol, methodology); err != nil {
		return model.Earnings{}, false, err
	}
	if e.Growth, err = earningsRows(conn, `SELECT date, date_type, growth, growth_benchmark,
		symbol_benchmark FROM earnings_growth WHERE symbol = ? ORDER BY date`,
		func(rows *sql.Rows, r *model.EarningsGrowth, date *string) error {
			return rows.Scan(date, &r.Length, &r.Growth, &r.Benchmark, &r.BenchmarkSymbol)
		}, func(r *model.EarningsGrowth) *model.EarningsPeriod { return &r.EarningsPeriod },
		symbol); err != nil {
		return model.Earnings{}, false, err
	}
	return e, true, nil
}

// earningsRows reads one kind of earnings item, oldest period first.
func earningsRows[T any](conn *sql.DB, query string, scan func(*sql.Rows, *T, *string) error,
	period func(*T) *model.EarningsPeriod, args ...any) ([]T, error) {
	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query earnings: %w", err)
	}
	defer rows.Close()

	var items []T
	for rows.Next() {
		var (
			item T
			date string
		)
		if err := scan(rows, &item, &date); err != nil {
			return nil, fmt.Errorf("failed to read earnings: %w", err)
		}
		if period(&item).Start, err = time.Parse(time.DateOnly, date); err != nil {
			return nil, fmt.Errorf("failed to read earnings period %q: %w", date, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read earnings: %w", err)
	}
	return items, nil
}

// quoteCurrency is the currency a symbol's latest market price came in, empty
// when it was never priced.
func quoteCurrency(conn *sql.DB, symbol string) (string, error) {
	var currency string
	err := conn.QueryRow(`SELECT currency FROM intraday_price WHERE symbol = ?
		ORDER BY fetched_at DESC LIMIT 1`, symbol).Scan(&currency)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to look up currency of %s: %w", symbol, err)
	}
	return currency, nil
}
