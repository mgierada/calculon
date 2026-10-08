package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
)

// Recommendations are shared market data, like quotes: reads span every user.

// StoreRecommendations records one page of a symbol's recommendations,
// overwriting months stored before.
func StoreRecommendations(conn *sql.DB, symbol string, fetchedAt time.Time,
	items []finimpulse.Recommendation) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin recommendations transaction: %w", err)
	}
	defer tx.Rollback()

	fetched := formatTime(fetchedAt)
	for _, r := range items {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO recommendation (symbol, date, strong_buy,
			buy, hold, sell, strong_sell, fetched_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			symbol, r.Date, r.StrongBuy, r.Buy, r.Hold, r.Sell, r.StrongSell, fetched); err != nil {
			return fmt.Errorf("failed to store recommendations of %s: %w", symbol, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit recommendations of %s: %w", symbol, err)
	}
	return nil
}

// RecommendationsOf reads a symbol's stored recommendations, oldest month
// first, false when none were stored.
func RecommendationsOf(conn *sql.DB, symbol string) (model.Recommendations, bool, error) {
	rows, err := conn.Query(`SELECT date, strong_buy, buy, hold, sell, strong_sell, fetched_at
		FROM recommendation WHERE symbol = ? ORDER BY date`, symbol)
	if err != nil {
		return model.Recommendations{}, false, fmt.Errorf(
			"failed to query recommendations of %s: %w", symbol, err)
	}
	defer rows.Close()

	recs := model.Recommendations{Symbol: symbol}
	for rows.Next() {
		var (
			r                model.Recommendation
			month, fetchedAt string
		)
		if err := rows.Scan(&month, &r.StrongBuy, &r.Buy, &r.Hold, &r.Sell, &r.StrongSell,
			&fetchedAt); err != nil {
			return model.Recommendations{}, false, fmt.Errorf(
				"failed to read recommendations of %s: %w", symbol, err)
		}
		if r.Month, err = time.Parse(time.DateOnly, month); err != nil {
			return model.Recommendations{}, false, fmt.Errorf(
				"failed to read recommendation month %q: %w", month, err)
		}
		fetched, err := parseRequiredTime(fetchedAt)
		if err != nil {
			return model.Recommendations{}, false, err
		}
		if fetched.After(recs.FetchedAt) {
			recs.FetchedAt = fetched
		}
		recs.Months = append(recs.Months, r)
	}
	if err := rows.Err(); err != nil {
		return model.Recommendations{}, false, fmt.Errorf(
			"failed to read recommendations of %s: %w", symbol, err)
	}
	return recs, len(recs.Months) > 0, nil
}
