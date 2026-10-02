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

// sampleNews is a real response, cut to one article.
const sampleNews = `{
  "task_id": "10021418-3863-0028-0000-dacf8822987a",
  "status_code": 20000,
  "status_message": "OK",
  "live": true,
  "cost": 0.0003,
  "result": {
    "total_count": 14,
    "items_count": 1,
    "search_after_token": "eyJ9",
    "items": [{
      "id": "198bdcd0-16a2-3542-95cb-674f73c3ba11",
      "type": "news",
      "title": "Stocks mostly rise as beaten-down tech stocks enjoy bounce",
      "description": "Asian and European stock markets mostly rose Friday.",
      "pub_date": "2026-07-03 15:53:10",
      "display_time": "2026-07-03 15:53:10",
      "canonical_url": "https://finance.yahoo.com/markets/world-indices/articles/x.html",
      "content_type": "STORY",
      "related_tickers": ["HY9H.MU", "XTB.WA"],
      "provider_display_name": "AFP",
      "provider_url": "http://www.afp.com/",
      "is_hosted": true,
      "is_premium_news": false
    }]
  }
}`

func TestNewsPostsSearchAndDecodes(t *testing.T) {
	var (
		method, contentType string
		sent                NewsRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, contentType = r.Method, r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &sent)
		w.Write([]byte(sampleNews))
	}))
	defer srv.Close()

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	request := LatestNews("XTB.WA", from, from.AddDate(0, 1, 0), 3)
	resp, err := New(srv.URL, "secret").News(context.Background(), request)
	if err != nil {
		t.Fatalf("News returned error: %v", err)
	}

	if method != http.MethodPost || contentType != "application/json" {
		t.Errorf("request = %s with %q, want a JSON POST", method, contentType)
	}
	if sent.Symbol != "XTB.WA" || sent.StartDate != "2026-09-01" || sent.EndDate != "2026-10-01" ||
		sent.Limit != 3 || len(sent.SortBy) != 1 || !sent.SortBy[0].Desc {
		t.Errorf("sent = %+v", sent)
	}
	items := resp.Result.Items
	if resp.Result.TotalCount != 14 || len(items) != 1 {
		t.Fatalf("result = %+v", resp.Result)
	}
	want := time.Date(2026, 7, 3, 15, 53, 10, 0, time.UTC)
	if !items[0].PubDate.Equal(want) || items[0].ProviderDisplayName != "AFP" ||
		len(items[0].RelatedTickers) != 2 || !items[0].IsHosted {
		t.Errorf("item = %+v", items[0])
	}
}
