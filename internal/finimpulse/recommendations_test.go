package finimpulse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// sampleRecommendations is a real response.
const sampleRecommendations = `{
  "task_id": "10080721-3863-0032-0000-8e8541d8a623",
  "status_code": 20000,
  "status_message": "OK",
  "live": true,
  "cost": 0.00125,
  "data": {"symbol": "XTB.WA", "limit": 3, "offset": 0,
           "start_date": "2026-01-01", "end_date": "2026-10-01"},
  "result": {
    "total_count": 9,
    "items_count": 3,
    "items": [
      {"date": "2026-09-01", "strong_buy": 0, "buy": 3, "hold": 1, "sell": 0, "strong_sell": 0},
      {"date": "2026-08-01", "strong_buy": 1, "buy": 3, "hold": 1, "sell": 0, "strong_sell": 0},
      {"date": "2026-07-01", "strong_buy": 0, "buy": 2, "hold": 1, "sell": 1, "strong_sell": 2}
    ]
  }
}`

func TestRecommendationsDecodesMonths(t *testing.T) {
	var sent RecommendationsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/analysis/recommendations" || r.Method != http.MethodPost {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &sent)
		w.Write([]byte(sampleRecommendations))
	}))
	defer srv.Close()

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	request := NewRecommendationsRequest("XTB.WA", from, from.AddDate(0, 9, 0), 3, 6)
	resp, err := New(srv.URL, "secret").Recommendations(context.Background(), request)
	if err != nil {
		t.Fatalf("Recommendations returned error: %v", err)
	}

	if sent.Symbol != "XTB.WA" || sent.StartDate != "2026-01-01" || sent.EndDate != "2026-10-01" ||
		sent.Limit != 3 || sent.Offset != 6 {
		t.Errorf("sent = %+v", sent)
	}
	r := resp.Result
	if r.TotalCount != 9 || len(r.Items) != 3 {
		t.Fatalf("result = %+v", r)
	}
	if got := r.Items[2]; got.Date != "2026-07-01" || got.Buy != 2 || got.Sell != 1 || got.StrongSell != 2 {
		t.Errorf("oldest month = %+v", got)
	}
	if r.Items[1].StrongBuy != 1 {
		t.Errorf("middle month = %+v", r.Items[1])
	}
}
