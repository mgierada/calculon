package db

import (
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
)

func ptr(v float64) *float64 { return &v }

func testEarnings(revenue float64, target float64) finimpulse.EarningsResult {
	q := func(date string) finimpulse.EarningsPeriod {
		return finimpulse.EarningsPeriod{Date: date, DateType: "quarter"}
	}
	return finimpulse.EarningsResult{
		TargetPrice: ptr(target), TargetAveragePrice: ptr(1234.19), TargetLowPrice: ptr(1060),
		TargetHighPrice: nil, TotalCount: 12,
		Growth: []finimpulse.EarningsGrowth{{EarningsPeriod: q("2026-04-01"), Growth: ptr(0.26),
			GrowthBenchmark: ptr(0.25), SymbolBenchmark: "SP5"}},
		EPS: []finimpulse.EarningsEPS{
			{EarningsPeriod: q("2026-04-01"), Methodology: "gaap", Actual: ptr(4.83), Estimate: ptr(4.71)},
			{EarningsPeriod: q("2026-04-01"), Methodology: "normalized", Actual: ptr(5)},
		},
		Revenue: []finimpulse.EarningsRevenue{
			{EarningsPeriod: q("2026-04-01"), Methodology: "gaap", Revenue: ptr(revenue), Earnings: ptr(479e6)},
			{EarningsPeriod: q("2026-01-01"), Methodology: "gaap", Revenue: ptr(2.5e9)},
		},
	}
}

func TestEarningsRoundTripKeepsLatestFigures(t *testing.T) {
	conn := openTestDB(t)
	if err := StoreEarnings(conn, "EQIX.US", testFetchedAt, testEarnings(2.6e9, 1000), true); err != nil {
		t.Fatalf("StoreEarnings returned error: %v", err)
	}
	// A later fetch revises a period and moves the target; a later page of
	// it repeats the target, which is not stored twice.
	later := testFetchedAt.Add(time.Hour)
	StoreEarnings(conn, "EQIX.US", later, testEarnings(2.625e9, 1011.58), true)
	StoreEarnings(conn, "EQIX.US", later, testEarnings(2.625e9, 1011.58), false)

	e, ok, err := EarningsOf(conn, "EQIX.US", "gaap")
	if err != nil || !ok {
		t.Fatalf("EarningsOf = %v, %v", ok, err)
	}
	if !e.FetchedAt.Equal(later) || *e.Target.Price != 1011.58 || e.Target.High != nil || e.TotalCount != 12 {
		t.Errorf("target = %+v fetched %v, want the latest with high unknown", e.Target, e.FetchedAt)
	}
	if len(e.Revenue) != 2 || e.Revenue[0].Start.Format(time.DateOnly) != "2026-01-01" ||
		*e.Revenue[1].Revenue != 2.625e9 || e.Revenue[0].Earnings != nil {
		t.Errorf("revenue = %+v, want oldest first with the revision", e.Revenue)
	}
	if len(e.EPS) != 1 || *e.EPS[0].Actual != 4.83 || e.EPS[0].Length != "quarter" {
		t.Errorf("eps = %+v, want only the gaap figure", e.EPS)
	}
	if len(e.Growth) != 1 || *e.Growth[0].Benchmark != 0.25 || e.Growth[0].BenchmarkSymbol != "SP5" {
		t.Errorf("growth = %+v", e.Growth)
	}

	var targets int
	conn.QueryRow(`SELECT COUNT(*) FROM earnings_target`).Scan(&targets)
	if targets != 2 {
		t.Errorf("stored %d targets, want one per fetch", targets)
	}
	if _, ok, _ := EarningsOf(conn, "XTB.PL", "gaap"); ok {
		t.Error("earnings found for a symbol never fetched")
	}
}
