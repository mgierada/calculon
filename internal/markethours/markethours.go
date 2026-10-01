// Package markethours knows when each exchange holds its regular session, so
// prices are polled only while they can change and a price can be told apart
// from one left over from an earlier session.
package markethours

import (
	"time"
	// Embedded so exchange time zones resolve on hosts without a zoneinfo
	// database, e.g. minimal containers.
	_ "time/tzdata"

	"github.com/mgierada/calculon/internal/symbolmap"
)

// DataDelay is how late market data may report a price. A session is polled
// that long past its close, so the closing price is stored, and a session
// only counts as started that long after its open, when its first price
// can have arrived.
const DataDelay = 20 * time.Minute

// clock is a time of day on an exchange's local clock.
type clock struct{ hour, minute int }

// Session is an exchange's regular trading hours, Monday to Friday.
// TODO: exchange holidays are not known, so those days count as trading days.
type Session struct {
	loc         *time.Location
	open, close clock
}

// sessions holds the regular hours of each exchange, by XTB symbol suffix.
var sessions = map[string]Session{
	".PL": newSession("Europe/Warsaw", clock{9, 0}, clock{17, 5}),
	".US": newSession("America/New_York", clock{9, 30}, clock{16, 0}),
	".UK": newSession("Europe/London", clock{8, 0}, clock{16, 35}),
	".DE": newSession("Europe/Berlin", clock{9, 0}, clock{17, 35}),
	".FR": newSession("Europe/Paris", clock{9, 0}, clock{17, 35}),
	".NL": newSession("Europe/Amsterdam", clock{9, 0}, clock{17, 35}),
	".BE": newSession("Europe/Brussels", clock{9, 0}, clock{17, 35}),
	".ES": newSession("Europe/Madrid", clock{9, 0}, clock{17, 35}),
	".IT": newSession("Europe/Rome", clock{9, 0}, clock{17, 35}),
	".PT": newSession("Europe/Lisbon", clock{8, 0}, clock{16, 35}),
	".CH": newSession("Europe/Zurich", clock{9, 0}, clock{17, 30}),
	".DK": newSession("Europe/Copenhagen", clock{9, 0}, clock{17, 0}),
	".NO": newSession("Europe/Oslo", clock{9, 0}, clock{16, 25}),
	".SE": newSession("Europe/Stockholm", clock{9, 0}, clock{17, 30}),
	".FI": newSession("Europe/Helsinki", clock{10, 0}, clock{18, 30}),
}

func newSession(zone string, open, close clock) Session {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		// The zones are constants and tzdata is embedded, so this is a typo.
		panic(err)
	}
	return Session{loc: loc, open: open, close: close}
}

// For is the session of a symbol's exchange; false for symbols whose exchange
// is unknown.
func For(symbol string) (Session, bool) {
	s, ok := sessions[symbolmap.Suffix(symbol)]
	return s, ok
}

// Day is the exchange-local trading day t falls on, as midnight in the
// exchange's time zone.
func (s Session) Day(t time.Time) time.Time {
	y, m, d := t.In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, s.loc)
}

// End is the end of the session t falls in, close plus DataDelay, or false
// outside one.
func (s Session) End(t time.Time) (time.Time, bool) {
	open, end := s.window(t)
	if !s.trades(t) || t.Before(open) || !t.Before(end) {
		return time.Time{}, false
	}
	return end, true
}

// NextOpen is the first session open after t.
func (s Session) NextOpen(t time.Time) time.Time {
	local := t.In(s.loc)
	for days := 0; ; days++ {
		day := local.AddDate(0, 0, days)
		if open, _ := s.window(day); s.trades(day) && open.After(t) {
			return open
		}
	}
}

// LatestDay is the most recent trading day whose session had started by now,
// counting from DataDelay after the open. A price from an earlier day is stale.
func (s Session) LatestDay(now time.Time) time.Time {
	local := now.In(s.loc)
	for days := 0; ; days-- {
		day := local.AddDate(0, 0, days)
		if open, _ := s.window(day); s.trades(day) && !open.Add(DataDelay).After(now) {
			return s.Day(day)
		}
	}
}

// window is the session on t's local day, from the open until the close plus
// DataDelay, whether or not the exchange trades that day.
func (s Session) window(t time.Time) (open, end time.Time) {
	y, m, d := t.In(s.loc).Date()
	open = time.Date(y, m, d, s.open.hour, s.open.minute, 0, 0, s.loc)
	closing := time.Date(y, m, d, s.close.hour, s.close.minute, 0, 0, s.loc)
	return open, closing.Add(DataDelay)
}

// trades reports whether the exchange holds a session on t's local day.
func (s Session) trades(t time.Time) bool {
	switch t.In(s.loc).Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return true
}
