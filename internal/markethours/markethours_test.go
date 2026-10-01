package markethours

import (
	"testing"
	"time"
)

var (
	newYork, _ = time.LoadLocation("America/New_York")
	warsaw, _  = time.LoadLocation("Europe/Warsaw")
)

func at(loc *time.Location, month time.Month, day, hour, minute int) time.Time {
	return time.Date(2026, month, day, hour, minute, 0, 0, loc)
}

func TestLatestDay(t *testing.T) {
	us, _ := For("AMT.US")
	// 2026-10-01 is a Thursday.
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"during session", at(newYork, 10, 1, 11, 0), at(newYork, 10, 1, 0, 0)},
		{"after close", at(newYork, 10, 1, 20, 0), at(newYork, 10, 1, 0, 0)},
		{"before open", at(newYork, 10, 1, 8, 0), at(newYork, 9, 30, 0, 0)},
		{"open, no price yet", at(newYork, 10, 1, 9, 40), at(newYork, 9, 30, 0, 0)},
		{"saturday", at(newYork, 10, 3, 12, 0), at(newYork, 10, 2, 0, 0)},
		{"monday before open", at(newYork, 10, 5, 7, 0), at(newYork, 10, 2, 0, 0)},
	}
	for _, c := range cases {
		if got := us.LatestDay(c.now); !got.Equal(c.want) {
			t.Errorf("%s: LatestDay(%v) = %v, want %v", c.name, c.now, got, c.want)
		}
	}
}

func TestDayIsExchangeLocal(t *testing.T) {
	pl, _ := For("XTB.PL")
	// 22:30 UTC on Sep 30 is past midnight in Warsaw.
	got := pl.Day(time.Date(2026, 9, 30, 22, 30, 0, 0, time.UTC))
	if want := at(warsaw, 10, 1, 0, 0); !got.Equal(want) {
		t.Errorf("Day = %v, want %v", got, want)
	}
}

func TestForUnknownExchange(t *testing.T) {
	if _, ok := For("US500"); ok {
		t.Error("a CFD without an exchange suffix got a session")
	}
}
