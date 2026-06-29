package db

import (
	"database/sql"
	"fmt"
	"math"

	"github.com/mgierada/calculon/internal/model"
)

// volumeEpsilon is the residue below which a netted holding counts as closed.
// Fractional-share fills mean sums rarely cancel to exactly zero.
const volumeEpsilon = 1e-9

// openHoldingsSQL nets stock purchases against sales per symbol.
//
// Holdings come from the cash ledger alone, not from the positions table. For a
// cash-equity account every buy is a "Stock purchase" cash op, and a closed
// trade produces both a positions row and a "Stock sale" cash op, so summing
// both sources would double count the same volume.
var openHoldingsSQL = fmt.Sprintf(`
SELECT
    symbol,
    SUM(CASE WHEN kind = '%[1]s' THEN volume ELSE -volume END)      AS volume,
    SUM(CASE WHEN kind = '%[1]s' THEN volume * price ELSE 0 END)    AS cost,
    SUM(CASE WHEN kind = '%[1]s' THEN volume ELSE 0 END)            AS bought,
    MIN(CASE WHEN kind = '%[1]s' THEN op_time END)                  AS first_open
FROM cash_ops
WHERE kind IN ('%[1]s', '%[2]s')
GROUP BY symbol
ORDER BY symbol`, model.CashOpStockPurchase, model.CashOpStockSale)

// OpenHoldings returns the still-open exposure per symbol, aggregated across
// every account and provider in the database. AvgOpenPrice is the
// volume-weighted price of all purchases, so it is an average cost basis rather
// than a FIFO lot cost.
func OpenHoldings(conn *sql.DB) ([]model.Holding, error) {
	rows, err := conn.Query(openHoldingsSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query open holdings: %w", err)
	}
	defer rows.Close()

	var holdings []model.Holding
	for rows.Next() {
		var (
			symbol    string
			volume    float64
			cost      float64
			bought    float64
			firstOpen sql.NullString
		)
		if err := rows.Scan(&symbol, &volume, &cost, &bought, &firstOpen); err != nil {
			return nil, fmt.Errorf("failed to scan holding: %w", err)
		}
		if volume <= volumeEpsilon {
			continue
		}

		openedAt, err := parseTime(firstOpen)
		if err != nil {
			return nil, fmt.Errorf("holding %s: bad first open time: %w", symbol, err)
		}

		var avgOpenPrice float64
		if bought > 0 {
			avgOpenPrice = cost / bought
		}

		holdings = append(holdings, model.Holding{
			Symbol:       symbol,
			Volume:       round(volume, 4),
			AvgOpenPrice: avgOpenPrice,
			FirstOpen:    openedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read holdings: %w", err)
	}

	return holdings, nil
}

// round trims floating point noise accumulated by summing fractional fills.
func round(value float64, decimals int) float64 {
	factor := math.Pow(10, float64(decimals))
	return math.Round(value*factor) / factor
}
