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

// sampleEarnings is a real response, with an item of a type not known yet.
const sampleEarnings = `{
  "task_id": "10041852-3863-0031-0000-7e4880a6829d",
  "status_code": 20000,
  "status_message": "OK",
  "live": true,
  "cost": 0.0014,
  "result": {
    "symbol": "EQIX",
    "target_price": 1011.58,
    "target_average_price": 1234.1936,
    "target_low_price": 1060,
    "target_high_price": 1380,
    "total_count": 12,
    "items_count": 5,
    "items": [
      {"type": "growth", "date": "2026-07-01", "date_type": "quarter",
       "growth": 0.0388, "growth_benchmark": 0.4987, "symbol_benchmark": "SP5"},
      {"type": "growth", "date": "2026-04-01", "date_type": "quarter",
       "growth": 0.2602, "growth_benchmark": 0.2598, "symbol_benchmark": "SP5"},
      {"type": "eps_actual", "date": "2026-04-01", "date_type": "quarter", "methodology": "gaap",
       "actual": 4.83, "estimate": 4.7138, "surprise": 0.1162, "surprise_pct": 2.47},
      {"type": "earnings_revenue", "date": "2026-04-01", "date_type": "quarter",
       "methodology": "gaap", "revenue": 2625000000, "earnings": 479000000},
      {"type": "dividends", "date": "2026-04-01"}
    ]
  }
}`

func TestEarningsSplitsItemsByType(t *testing.T) {
	var sent EarningsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/analysis/earnings" || r.Method != http.MethodPost {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &sent)
		w.Write([]byte(sampleEarnings))
	}))
	defer srv.Close()

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	types := []string{EarningsTypeEPS, EarningsTypeRevenue, EarningsTypeGrowth}
	request := NewEarningsRequest("EQIX", types, MethodologyGAAP, from, from.AddDate(0, 9, 0), 3, 6)
	resp, err := New(srv.URL, "secret").Earnings(context.Background(), request)
	if err != nil {
		t.Fatalf("Earnings returned error: %v", err)
	}

	if sent.Symbol != "EQIX" || len(sent.Types) != 3 || len(sent.Methodologies) != 1 ||
		sent.StartDate != "2026-01-01" || sent.EndDate != "2026-10-01" || sent.Offset != 6 {
		t.Errorf("sent = %+v", sent)
	}
	r := resp.Result
	if *r.TargetPrice != 1011.58 || *r.TargetLowPrice != 1060 || *r.TargetHighPrice != 1380 || r.TotalCount != 12 {
		t.Errorf("targets = %+v", r)
	}
	if len(r.Growth) != 2 || len(r.EPS) != 1 || len(r.Revenue) != 1 || r.Items() != 4 {
		t.Fatalf("items = %d growth, %d eps, %d revenue", len(r.Growth), len(r.EPS), len(r.Revenue))
	}
	if r.Growth[0].Date != "2026-07-01" || *r.Growth[0].Growth != 0.0388 || r.Growth[0].SymbolBenchmark != "SP5" {
		t.Errorf("growth = %+v", r.Growth[0])
	}
	if *r.EPS[0].SurprisePct != 2.47 || r.EPS[0].Methodology != "gaap" {
		t.Errorf("eps = %+v", r.EPS[0])
	}
	if *r.Revenue[0].Revenue != 2625000000 || *r.Revenue[0].Earnings != 479000000 {
		t.Errorf("revenue = %+v", r.Revenue[0])
	}
}

func TestEarningsRequestOmitsMethodologyWhenEmpty(t *testing.T) {
	encoded, _ := json.Marshal(NewEarningsRequest("EQIX", nil, "", time.Now(), time.Now(), 1, 0))
	var fields map[string]any
	json.Unmarshal(encoded, &fields)
	if _, ok := fields["methodologies"]; ok {
		t.Errorf("request = %s, want no methodologies to ask for all", encoded)
	}
}
