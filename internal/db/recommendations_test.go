package db

import (
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
)

func TestRecommendationsRoundTripKeepsRevisedMonths(t *testing.T) {
	conn := openTestDB(t)
	first := []finimpulse.Recommendation{
		{Date: "2026-09-01", Buy: 3, Hold: 1},
		{Date: "2026-08-01", StrongBuy: 1, Buy: 2},
	}
	if err := StoreRecommendations(conn, "XTB.PL", testFetchedAt, first); err != nil {
		t.Fatalf("StoreRecommendations returned error: %v", err)
	}
	// A later fetch revises September and adds October.
	later := testFetchedAt.Add(time.Hour)
	revised := []finimpulse.Recommendation{
		{Date: "2026-10-01", Buy: 4},
		{Date: "2026-09-01", Buy: 2, Hold: 1, Sell: 1},
	}
	if err := StoreRecommendations(conn, "XTB.PL", later, revised); err != nil {
		t.Fatalf("StoreRecommendations returned error: %v", err)
	}

	recs, ok, err := RecommendationsOf(conn, "XTB.PL")
	if err != nil || !ok {
		t.Fatalf("RecommendationsOf = %v, %v", ok, err)
	}
	if !recs.FetchedAt.Equal(later) || len(recs.Months) != 3 {
		t.Fatalf("recs = %+v, want three months fetched at the latest", recs)
	}
	if recs.Months[0].Month.Format(time.DateOnly) != "2026-08-01" || recs.Months[0].StrongBuy != 1 {
		t.Errorf("oldest = %+v, want August first", recs.Months[0])
	}
	if sep := recs.Months[1]; sep.Buy != 2 || sep.Sell != 1 {
		t.Errorf("september = %+v, want the revision", sep)
	}
	if _, ok, _ := RecommendationsOf(conn, "EQIX.US"); ok {
		t.Error("recommendations found for a symbol never fetched")
	}
}
