package marketdata

import (
	"context"
	"testing"
	"time"

	"github.com/mgierada/calculon/internal/finimpulse"
)

var (
	newYork, _ = time.LoadLocation("America/New_York")
	warsaw, _  = time.LoadLocation("Europe/Warsaw")
)

func TestDueAt(t *testing.T) {
	// 2026-09-30 is a Wednesday; 2026-10-25 ends summer time in Europe.
	at := func(loc *time.Location, month time.Month, day, hour, minute int) time.Time {
		return time.Date(2026, month, day, hour, minute, 0, 0, loc)
	}
	cases := []struct {
		name   string
		symbol string
		last   time.Time
		want   time.Time
	}{
		{"never fetched", "AMT.US", time.Time{}, time.Time{}},
		{"in session", "AMT.US", at(newYork, 9, 30, 15, 0), at(newYork, 9, 30, 15, 15)},
		{"last poll before close", "AMT.US", at(newYork, 9, 30, 16, 10), at(newYork, 9, 30, 16, 20)},
		{"after close", "AMT.US", at(newYork, 9, 30, 16, 20), at(newYork, 10, 1, 9, 30)},
		{"before open", "AMT.US", at(newYork, 10, 1, 7, 0), at(newYork, 10, 1, 9, 30)},
		{"friday close", "AMT.US", at(newYork, 10, 2, 16, 25), at(newYork, 10, 5, 9, 30)},
		{"weekend", "AMT.US", at(newYork, 10, 3, 12, 0), at(newYork, 10, 5, 9, 30)},
		{"across DST change", "XTB.PL", at(warsaw, 10, 23, 17, 30), at(warsaw, 10, 26, 9, 0)},
		{"unknown exchange", "US500", at(newYork, 10, 3, 12, 0), at(newYork, 10, 3, 12, 15)},
	}
	for _, c := range cases {
		if got := dueAt(c.symbol, c.last, interval); !got.Equal(c.want) {
			t.Errorf("%s: dueAt(%s, %v) = %v, want %v", c.name, c.symbol, c.last, got, c.want)
		}
	}
}

// Outside trading hours a stored closing price is enough: nothing is fetched
// until the next open.
func TestPollWaitsForOpenOnceCloseIsStored(t *testing.T) {
	fetcher := &fakeFetcher{prices: map[string]finimpulse.MarketPrice{
		"AMT": marketPrice("AMT", "USD", 170),
	}}
	poller, now := setup(t, fetcher, "AMT.US")
	*now = time.Date(2026, 10, 2, 16, 20, 0, 0, newYork)
	poller.poll(context.Background())

	*now = time.Date(2026, 10, 3, 12, 0, 0, 0, newYork)
	fetcher.requests = nil
	poller.poll(context.Background())
	if len(fetcher.requests) != 0 {
		t.Errorf("requests = %v, want none on the weekend", fetcher.requests)
	}
}

// Started on a weekend after missing Friday's close, the poller catches up
// with one fetch.
func TestPollCatchesUpOnMissedClose(t *testing.T) {
	fetcher := &fakeFetcher{prices: map[string]finimpulse.MarketPrice{
		"AMT": marketPrice("AMT", "USD", 170),
	}}
	poller, now := setup(t, fetcher, "AMT.US")
	*now = time.Date(2026, 10, 2, 14, 0, 0, 0, newYork)
	poller.poll(context.Background())

	*now = time.Date(2026, 10, 3, 12, 0, 0, 0, newYork)
	fetcher.requests = nil
	poller.poll(context.Background())
	poller.poll(context.Background())
	if len(fetcher.requests) != 1 {
		t.Errorf("requests = %v, want one catch-up fetch", fetcher.requests)
	}
}
