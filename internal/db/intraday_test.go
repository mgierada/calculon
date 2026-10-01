package db

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/model"
)

var testFetchedAt = time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)

func testIntradayPrice(symbol string, fetchedAt time.Time, price float64) IntradayPrice {
	updated := fetchedAt.Add(-2 * time.Minute)
	return IntradayPrice{
		Symbol:    symbol,
		FetchedAt: fetchedAt,
		Response: finimpulse.Response[finimpulse.MarketPrice]{
			Meta: finimpulse.Meta{TaskID: "task", StatusCode: 20000, StatusMessage: "OK", Live: true, Cost: 0.0001},
			Result: finimpulse.MarketPrice{
				Symbol: "SNT.WA", Name: "Synektik", QuoteType: "stock", Currency: "PLN",
				MarketState: "REGULAR", CurrentPrice: &price, CurrentPriceUpdateTime: &updated,
			},
		},
	}
}

func mustStoreIntradayPrice(t *testing.T, conn *Conn, price IntradayPrice, quote *model.Quote,
	daily *DailyClose) {
	t.Helper()

	if err := StoreIntradayPrice(conn, price, quote, daily); err != nil {
		t.Fatalf("StoreIntradayPrice returned error: %v", err)
	}
}

func TestStoreIntradayPriceKeepsEveryResponseAndQuote(t *testing.T) {
	conn := openTestDB(t)
	price := testIntradayPrice("SNT.PL", testFetchedAt, 350)
	quote := model.Quote{Symbol: "SNT.PL", AsOf: testFetchedAt, Price: 350, Source: "finimpulse"}
	mustStoreIntradayPrice(t, conn, price, &quote, nil)
	mustStoreIntradayPrice(t, conn, testIntradayPrice("SNT.PL", testFetchedAt.Add(15*time.Minute), 351), nil, nil)

	var (
		count                int
		providerSymbol       string
		currentPrice         float64
		updateTime           string
		preMarket, marketCap sql.NullFloat64
		live                 bool
	)
	if err := conn.QueryRow(`SELECT COUNT(*) FROM intraday_price`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("stored %d responses, want every one", count)
	}
	err := conn.QueryRow(`SELECT provider_symbol, current_price, current_price_update_time,
		pre_market_price, market_cap, live FROM intraday_price ORDER BY fetched_at LIMIT 1`).
		Scan(&providerSymbol, &currentPrice, &updateTime, &preMarket, &marketCap, &live)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if providerSymbol != "SNT.WA" || currentPrice != 350 || updateTime != "2026-09-30T14:58:00Z" || !live {
		t.Errorf("row = %s %v %s live=%v", providerSymbol, currentPrice, updateTime, live)
	}
	if preMarket.Valid || marketCap.Valid {
		t.Errorf("null fields stored as set: pre=%v cap=%v", preMarket, marketCap)
	}

	quotes, err := allQuotes(conn)
	if err != nil {
		t.Fatalf("quotes: %v", err)
	}
	if len(quotes) != 1 || quotes[0] != quote.Price {
		t.Errorf("quotes = %v, want only the one passed", quotes)
	}
}

func allQuotes(conn *Conn) ([]float64, error) {
	rows, err := conn.Query(`SELECT price FROM quotes WHERE source = 'finimpulse'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prices []float64
	for rows.Next() {
		var p float64
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}
	return prices, rows.Err()
}

func TestPriceTargetsListsMappedHeldSymbolsWithLastFetch(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	statement := testStatement()
	statement.OpenLots = append(statement.OpenLots, testLot("l3", "AMT.US", 168.98),
		testLot("l4", "US500", 6000))
	mustImport(t, conn, user.ID, statement)

	mappings := map[string]string{"SNT.PL": "SNT.WA", "AMT.US": "AMT", "OLD.PL": "OLD.WA"}
	if err := StoreSymbolMappings(conn, "finimpulse", mappings); err != nil {
		t.Fatalf("StoreSymbolMappings returned error: %v", err)
	}
	mustStoreIntradayPrice(t, conn, testIntradayPrice("SNT.PL", testFetchedAt, 350), nil, nil)
	mustStoreIntradayPrice(t, conn, testIntradayPrice("SNT.PL", testFetchedAt.Add(time.Hour), 351), nil, nil)

	targets, err := PriceTargets(conn, "finimpulse")
	if err != nil {
		t.Fatalf("PriceTargets returned error: %v", err)
	}
	want := []PriceTarget{
		{Symbol: "AMT.US", ProviderSymbol: "AMT"},
		{Symbol: "SNT.PL", ProviderSymbol: "SNT.WA", LastFetched: testFetchedAt.Add(time.Hour)},
	}
	if len(targets) != len(want) {
		t.Fatalf("targets = %+v, want %+v", targets, want)
	}
	for i := range want {
		if targets[i].Symbol != want[i].Symbol || targets[i].ProviderSymbol != want[i].ProviderSymbol ||
			!targets[i].LastFetched.Equal(want[i].LastFetched) {
			t.Errorf("targets[%d] = %+v, want %+v", i, targets[i], want[i])
		}
	}
}

// A mapping fixed by hand must survive the rules seeding it again.
func TestStoreSymbolMappingsKeepsStoredMapping(t *testing.T) {
	conn := openTestDB(t)
	if err := StoreSymbolMappings(conn, "finimpulse", map[string]string{"MXFS.UK": "MXFS.L"}); err != nil {
		t.Fatalf("first store: %v", err)
	}
	if _, err := conn.Exec(`UPDATE symbol_map SET provider_symbol = 'MXF.L'`); err != nil {
		t.Fatalf("hand fix: %v", err)
	}
	if err := StoreSymbolMappings(conn, "finimpulse", map[string]string{"MXFS.UK": "MXFS.L"}); err != nil {
		t.Fatalf("second store: %v", err)
	}

	var stored string
	if err := conn.QueryRow(`SELECT provider_symbol FROM symbol_map`).Scan(&stored); err != nil {
		t.Fatalf("select: %v", err)
	}
	if stored != "MXF.L" {
		t.Errorf("provider_symbol = %q, want the hand fix kept", stored)
	}
}

func TestQuotesRoundTripPrevClose(t *testing.T) {
	conn := openTestDB(t)
	user := createTestUser(t, conn, "alice")
	mustImport(t, conn, user.ID, testStatement())
	quote := model.Quote{Symbol: "SNT.PL", AsOf: testFetchedAt, Price: 350, Source: "finimpulse", PrevClose: 345}
	mustStoreIntradayPrice(t, conn, testIntradayPrice("SNT.PL", testFetchedAt, 350), &quote, nil)

	quotes, err := Quotes(conn, user.ID)
	if err != nil {
		t.Fatalf("Quotes returned error: %v", err)
	}
	if len(quotes) != 2 || quotes[0].PrevClose != 0 || quotes[1].PrevClose != 345 {
		t.Errorf("quotes = %+v, want the statement's without a close, the API's with one", quotes)
	}
}

func TestDailyCloseKeepsLatestPricePerSession(t *testing.T) {
	conn := openTestDB(t)
	day := func(d int, at time.Time, price float64) *DailyClose {
		return &DailyClose{
			Symbol: "SNT.PL", Session: time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC),
			Close: price, Currency: "PLN", AsOf: at, FetchedAt: at,
		}
	}
	price := testIntradayPrice("SNT.PL", testFetchedAt, 350)
	mustStoreIntradayPrice(t, conn, price, nil, day(30, testFetchedAt, 350))
	mustStoreIntradayPrice(t, conn, price, nil, day(30, testFetchedAt.Add(time.Hour), 352))
	mustStoreIntradayPrice(t, conn, price, nil, day(29, testFetchedAt, 340))

	rows, err := conn.Query(`SELECT session_date, close FROM eod_price ORDER BY session_date`)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var (
			date  string
			close float64
		)
		if err := rows.Scan(&date, &close); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, fmt.Sprintf("%s=%v", date, close))
	}
	if want := "[2026-09-29=340 2026-09-30=352]"; fmt.Sprint(got) != want {
		t.Errorf("closes = %v, want %s", got, want)
	}
}
