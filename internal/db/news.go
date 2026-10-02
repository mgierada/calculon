package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
)

// News is shared market data, like quotes: reads span every user.

// newsSQL reads up to perSymbol of the newest articles of each symbol, every
// article once with all the listed symbols it was found for. %s is the
// placeholders of the symbols.
const newsSQL = `
WITH ranked AS (
    SELECT s.symbol, s.news_id,
           ROW_NUMBER() OVER (PARTITION BY s.symbol ORDER BY n.pub_date DESC) AS rank
    FROM news_symbol s JOIN news_item n ON n.id = s.news_id
    WHERE s.symbol IN (%s)
)
SELECT n.id, n.title, n.description, n.pub_date, n.canonical_url, n.provider_display_name,
       n.is_premium_news, n.related_tickers, group_concat(r.symbol, ',')
FROM ranked r JOIN news_item n ON n.id = r.news_id
WHERE r.rank <= ?
GROUP BY n.id
ORDER BY n.pub_date DESC, n.id`

// StoreNews records articles found for symbol, skipping ones already stored,
// and returns how many were new.
func StoreNews(conn *sql.DB, symbol string, fetchedAt time.Time, items []finimpulse.NewsItem) (int, error) {
	tx, err := conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin news transaction: %w", err)
	}
	defer tx.Rollback()

	added := 0
	for _, item := range items {
		inserted, err := insertNewsItem(tx, item, fetchedAt)
		if err != nil {
			return 0, err
		}
		if inserted {
			added++
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO news_symbol (symbol, news_id) VALUES (?, ?)`,
			symbol, item.ID); err != nil {
			return 0, fmt.Errorf("failed to link news %s to %s: %w", item.ID, symbol, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit news: %w", err)
	}
	return added, nil
}

// insertNewsItem stores an article unless it is stored already.
func insertNewsItem(tx *sql.Tx, item finimpulse.NewsItem, fetchedAt time.Time) (bool, error) {
	tickers, err := json.Marshal(item.RelatedTickers)
	if err != nil {
		return false, fmt.Errorf("failed to encode tickers of news %s: %w", item.ID, err)
	}
	res, err := tx.Exec(`INSERT OR IGNORE INTO news_item (id, type, title, description, pub_date,
		display_time, canonical_url, content_type, related_tickers, provider_display_name,
		provider_url, is_hosted, is_premium_news, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Type, item.Title, item.Description, formatTime(item.PubDate.Time),
		nullTime(item.DisplayTime.Time), item.CanonicalURL, item.ContentType, string(tickers),
		item.ProviderDisplayName, item.ProviderURL, item.IsHosted, item.IsPremiumNews,
		formatTime(fetchedAt))
	if err != nil {
		return false, fmt.Errorf("failed to store news %s: %w", item.ID, err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// LatestNewsTime is when the newest article stored for symbol was published,
// the zero time when none is.
func LatestNewsTime(conn *sql.DB, symbol string) (time.Time, error) {
	var latest sql.NullString
	if err := conn.QueryRow(`SELECT MAX(n.pub_date) FROM news_symbol s
		JOIN news_item n ON n.id = s.news_id WHERE s.symbol = ?`, symbol).Scan(&latest); err != nil {
		return time.Time{}, fmt.Errorf("failed to look up news of %s: %w", symbol, err)
	}
	return parseTime(latest)
}

// ProviderSymbol is how provider names symbol, false when it is not mapped.
func ProviderSymbol(conn *sql.DB, symbol, provider string) (string, bool, error) {
	var providerSymbol string
	err := conn.QueryRow(`SELECT provider_symbol FROM symbol_map WHERE symbol = ? AND provider = ?`,
		symbol, provider).Scan(&providerSymbol)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("failed to look up %s at %s: %w", symbol, provider, err)
	}
	return providerSymbol, true, nil
}

// News lists up to perSymbol of the newest stored articles about each of the
// symbols, newest first.
func News(conn *sql.DB, symbols []string, perSymbol int) ([]model.NewsItem, error) {
	if len(symbols) == 0 || perSymbol <= 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(symbols)), ", ")
	args := make([]any, 0, len(symbols)+1)
	for _, symbol := range symbols {
		args = append(args, symbol)
	}
	rows, err := conn.Query(fmt.Sprintf(newsSQL, placeholders), append(args, perSymbol)...)
	if err != nil {
		return nil, fmt.Errorf("failed to query news: %w", err)
	}
	defer rows.Close()

	var items []model.NewsItem
	for rows.Next() {
		item, err := scanNewsItem(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to read news: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read news: %w", err)
	}
	return items, nil
}

func scanNewsItem(rows *sql.Rows) (model.NewsItem, error) {
	var (
		item               model.NewsItem
		published, tickers string
		symbols            string
	)
	if err := rows.Scan(&item.ID, &item.Title, &item.Description, &published, &item.URL,
		&item.Source, &item.Premium, &tickers, &symbols); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(tickers), &item.RelatedTickers); err != nil {
		return item, err
	}
	item.Symbols = strings.Split(symbols, ",")
	slices.Sort(item.Symbols)
	var err error
	item.Published, err = parseRequiredTime(published)
	return item, err
}
