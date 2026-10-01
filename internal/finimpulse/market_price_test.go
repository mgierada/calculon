package finimpulse

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// sampleMarketPrice is a real response from the API.
const sampleMarketPrice = `{
  "task_id": "10011137-3863-0055-0000-cef0340134d5",
  "status_code": 20000,
  "status_message": "OK",
  "live": true,
  "cost": 0.0001,
  "data": {"symbol": "XTB.WA"},
  "result": {
    "symbol": "XTB.WA",
    "name": "XTB S.A.",
    "quote_type": "stock",
    "currency": "PLN",
    "regular_market_volume": 307538,
    "market_cap": 17675419648,
    "usd_rate": null,
    "market_state": "REGULAR",
    "regular_market_open": 152.6,
    "regular_market_previous_close": 151.7,
    "current_price": 150.56,
    "current_price_usd": 39.234490624,
    "current_price_change": -1.1399994,
    "current_price_change_percent": -0.7514828,
    "current_price_update_time": "2026-09-30T14:57:42Z",
    "regular_market_price": 150.56,
    "regular_market_price_change": -1.1399994,
    "regular_market_price_change_percent": -0.7514828,
    "regular_market_time": "2026-09-30T14:42:19Z",
    "pre_market_price": null,
    "pre_market_price_change": null,
    "pre_market_price_change_percent": null,
    "pre_market_time": null,
    "post_market_price": null,
    "post_market_price_change": null,
    "post_market_price_change_percent": null,
    "post_market_time": null
  }
}`

// serve answers every request with status and body, recording the request.
func serve(t *testing.T, status int, body string) (*Client, *http.Request) {
	t.Helper()

	seen := &http.Request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r.Clone(context.Background())
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL+"/", "secret"), seen
}

func TestMarketPriceDecodesResponse(t *testing.T) {
	client, seen := serve(t, http.StatusOK, sampleMarketPrice)

	resp, err := client.MarketPrice(context.Background(), "XTB.WA")
	if err != nil {
		t.Fatalf("MarketPrice returned error: %v", err)
	}
	if seen.URL.Path != "/v1/market-price/XTB.WA" {
		t.Errorf("path = %q", seen.URL.Path)
	}
	if got := seen.Header.Get("Authorization"); got != "Bearer secret" {
		t.Errorf("Authorization = %q", got)
	}
	if resp.TaskID != "10011137-3863-0055-0000-cef0340134d5" || resp.Cost != 0.0001 || !resp.Live {
		t.Errorf("meta = %+v", resp.Meta)
	}

	price := resp.Result
	if price.Symbol != "XTB.WA" || price.Currency != "PLN" || price.MarketState != "REGULAR" {
		t.Errorf("result = %+v", price)
	}
	if price.CurrentPrice == nil || *price.CurrentPrice != 150.56 {
		t.Errorf("CurrentPrice = %v", price.CurrentPrice)
	}
	if price.MarketCap == nil || *price.MarketCap != 17675419648 {
		t.Errorf("MarketCap = %v", price.MarketCap)
	}
	want := time.Date(2026, 9, 30, 14, 57, 42, 0, time.UTC)
	if price.CurrentPriceUpdateTime == nil || !price.CurrentPriceUpdateTime.Equal(want) {
		t.Errorf("CurrentPriceUpdateTime = %v", price.CurrentPriceUpdateTime)
	}
	if price.USDRate != nil || price.PreMarketPrice != nil || price.PostMarketTime != nil {
		t.Errorf("null fields decoded as set: %+v", price)
	}
}

func TestMarketPriceRejectsHTTPError(t *testing.T) {
	client, _ := serve(t, http.StatusUnauthorized, `{"status_code": 40100, "status_message": "bad token"}`)

	_, err := client.MarketPrice(context.Background(), "XTB.WA")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.HTTPStatus != http.StatusUnauthorized || apiErr.Message != "bad token" {
		t.Errorf("apiErr = %+v", apiErr)
	}
}

// The API can answer 200 while its own status code reports a failure.
func TestMarketPriceRejectsStatusCodeInBody(t *testing.T) {
	client, _ := serve(t, http.StatusOK,
		`{"status_code": 40400, "status_message": "symbol not found", "result": null}`)

	_, err := client.MarketPrice(context.Background(), "NOPE")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 40400 {
		t.Fatalf("err = %v, want *APIError with status 40400", err)
	}
}

func TestMarketPriceReportsNonJSONBody(t *testing.T) {
	client, _ := serve(t, http.StatusBadGateway, "<html>bad gateway</html>")

	_, err := client.MarketPrice(context.Background(), "XTB.WA")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "<html>bad gateway</html>" {
		t.Fatalf("err = %v, want the raw body as message", err)
	}
}
