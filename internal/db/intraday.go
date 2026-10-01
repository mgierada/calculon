package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
)

// Market data is shared, not owned: the reads below span every user.

// heldSymbolsSQL lists every symbol some user holds an open lot of.
const heldSymbolsSQL = `SELECT DISTINCT symbol FROM open_lots ORDER BY symbol`

// priceTargetsSQL pairs each mapped held symbol with when it was last fetched.
const priceTargetsSQL = `
SELECT m.symbol, m.provider_symbol, MAX(i.fetched_at)
FROM symbol_map m
LEFT JOIN intraday_price i ON i.symbol = m.symbol
WHERE m.provider = ? AND m.symbol IN (SELECT symbol FROM open_lots)
GROUP BY m.symbol, m.provider_symbol
ORDER BY m.symbol`

// PriceTarget is a held symbol to fetch a market price for.
type PriceTarget struct {
	Symbol         string
	ProviderSymbol string
	// LastFetched is the zero time when it was never fetched.
	LastFetched time.Time
}

// IntradayPrice is one market price response for one of our symbols.
type IntradayPrice struct {
	Symbol    string
	FetchedAt time.Time
	finimpulse.Response[finimpulse.MarketPrice]
}

// HeldSymbols lists every symbol any user holds.
func HeldSymbols(conn *sql.DB) ([]string, error) {
	rows, err := conn.Query(heldSymbolsSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query held symbols: %w", err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to read held symbols: %w", err)
		}
		symbols = append(symbols, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read held symbols: %w", err)
	}
	return symbols, nil
}

// StoreSymbolMappings records how provider names each symbol, keeping any
// mapping already stored.
func StoreSymbolMappings(conn *sql.DB, provider string, providerSymbols map[string]string) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin symbol map transaction: %w", err)
	}
	defer tx.Rollback()

	for symbol, providerSymbol := range providerSymbols {
		if _, err := tx.Exec(`INSERT INTO symbol_map (symbol, provider, provider_symbol)
			VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, symbol, provider, providerSymbol); err != nil {
			return fmt.Errorf("failed to map %s for %s: %w", symbol, provider, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit symbol map: %w", err)
	}
	return nil
}

// PriceTargets lists the held symbols provider has a mapping for, with when
// each was last fetched.
func PriceTargets(conn *sql.DB, provider string) ([]PriceTarget, error) {
	rows, err := conn.Query(priceTargetsSQL, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to query price targets: %w", err)
	}
	defer rows.Close()

	var targets []PriceTarget
	for rows.Next() {
		var (
			target      PriceTarget
			lastFetched sql.NullString
		)
		if err := rows.Scan(&target.Symbol, &target.ProviderSymbol, &lastFetched); err != nil {
			return nil, fmt.Errorf("failed to read price targets: %w", err)
		}
		if target.LastFetched, err = parseTime(lastFetched); err != nil {
			return nil, fmt.Errorf("failed to read price targets: %w", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read price targets: %w", err)
	}
	return targets, nil
}

// StoreIntradayPrice records a market price response and, when given, the
// quote taken from it, together so dashboards never see one without the other.
func StoreIntradayPrice(conn *sql.DB, price IntradayPrice, quote *model.Quote) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin intraday price transaction: %w", err)
	}
	defer tx.Rollback()

	if err := insert(tx, intradayPriceRow(price)); err != nil {
		return fmt.Errorf("failed to insert intraday price of %s: %w", price.Symbol, err)
	}
	if quote != nil {
		r, err := quoteRow(*quote)
		if err != nil {
			return err
		}
		if _, err := upsert(tx, r); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit intraday price of %s: %w", price.Symbol, err)
	}
	return nil
}

func intradayPriceRow(price IntradayPrice) row {
	p := price.Result
	return row{
		table: "intraday_price",
		data: []field{
			{"symbol", price.Symbol}, {"fetched_at", formatTime(price.FetchedAt)},
			{"task_id", price.TaskID}, {"status_code", price.StatusCode},
			{"status_message", price.StatusMessage}, {"live", price.Live}, {"cost", price.Cost},
			{"provider_symbol", p.Symbol}, {"name", p.Name}, {"quote_type", p.QuoteType},
			{"currency", p.Currency}, {"regular_market_volume", p.RegularMarketVolume},
			{"market_cap", p.MarketCap}, {"usd_rate", p.USDRate},
			{"market_state", p.MarketState}, {"regular_market_open", p.RegularMarketOpen},
			{"regular_market_previous_close", p.RegularMarketPreviousClose},
			{"current_price", p.CurrentPrice}, {"current_price_usd", p.CurrentPriceUSD},
			{"current_price_change", p.CurrentPriceChange},
			{"current_price_change_percent", p.CurrentPriceChangePercent},
			{"current_price_update_time", nullTimePtr(p.CurrentPriceUpdateTime)},
			{"regular_market_price", p.RegularMarketPrice},
			{"regular_market_price_change", p.RegularMarketPriceChange},
			{"regular_market_price_change_percent", p.RegularMarketPriceChangePercent},
			{"regular_market_time", nullTimePtr(p.RegularMarketTime)},
			{"pre_market_price", p.PreMarketPrice},
			{"pre_market_price_change", p.PreMarketPriceChange},
			{"pre_market_price_change_percent", p.PreMarketPriceChangePercent},
			{"pre_market_time", nullTimePtr(p.PreMarketTime)},
			{"post_market_price", p.PostMarketPrice},
			{"post_market_price_change", p.PostMarketPriceChange},
			{"post_market_price_change_percent", p.PostMarketPriceChangePercent},
			{"post_market_time", nullTimePtr(p.PostMarketTime)},
		},
	}
}

// nullTimePtr renders an optional timestamp for storage, NULL when absent.
func nullTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return nullTime(*t)
}
