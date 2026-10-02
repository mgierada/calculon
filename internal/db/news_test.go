package db

import (
	"slices"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
)

func testNews(id string, published time.Time) finimpulse.NewsItem {
	return finimpulse.NewsItem{
		ID: id, Type: "news", Title: "title " + id, Description: "about " + id,
		PubDate: finimpulse.NewsTime{Time: published}, CanonicalURL: "https://example.test/" + id,
		ContentType: "STORY", RelatedTickers: []string{"XTB.WA", "SNT.WA"},
		ProviderDisplayName: "AFP", ProviderURL: "http://www.afp.com/",
	}
}

func mustStoreNews(t *testing.T, conn *Conn, symbol string, items ...finimpulse.NewsItem) int {
	t.Helper()

	added, err := StoreNews(conn, symbol, testFetchedAt, items)
	if err != nil {
		t.Fatalf("StoreNews returned error: %v", err)
	}
	return added
}

func TestStoreNewsSkipsStoredArticles(t *testing.T) {
	conn := openTestDB(t)
	day := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

	if added := mustStoreNews(t, conn, "XTB.PL", testNews("a", day), testNews("b", day)); added != 2 {
		t.Errorf("first store added %d, want 2", added)
	}
	if added := mustStoreNews(t, conn, "XTB.PL", testNews("b", day), testNews("c", day)); added != 1 {
		t.Errorf("second store added %d, want only the unseen article", added)
	}
	latest, err := LatestNewsTime(conn, "XTB.PL")
	if err != nil || !latest.Equal(day) {
		t.Errorf("LatestNewsTime = %v, %v; want %v", latest, err, day)
	}
	if none, _ := LatestNewsTime(conn, "SNT.PL"); !none.IsZero() {
		t.Errorf("LatestNewsTime of a symbol without news = %v, want zero", none)
	}
}

func TestNewsListsNewestPerSymbolOnce(t *testing.T) {
	conn := openTestDB(t)
	day := func(d int) time.Time { return time.Date(2026, 9, d, 12, 0, 0, 0, time.UTC) }
	mustStoreNews(t, conn, "XTB.PL", testNews("old", day(1)), testNews("mid", day(2)), testNews("shared", day(3)))
	mustStoreNews(t, conn, "SNT.PL", testNews("shared", day(3)), testNews("snt", day(4)))
	mustStoreNews(t, conn, "CDR.PL", testNews("cdr", day(5)))

	items, err := News(conn, []string{"XTB.PL", "SNT.PL"}, 2)
	if err != nil {
		t.Fatalf("News returned error: %v", err)
	}
	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if want := []string{"snt", "shared", "mid"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want two newest per symbol, newest first, %v", ids, want)
	}
	shared := items[1]
	if !slices.Equal(shared.Symbols, []string{"SNT.PL", "XTB.PL"}) || len(shared.RelatedTickers) != 2 ||
		shared.Source != "AFP" || !shared.Published.Equal(day(3)) {
		t.Errorf("shared = %+v", shared)
	}
}
