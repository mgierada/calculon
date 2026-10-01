package marketdata

import (
	"time"
	// Embedded so exchange time zones resolve on hosts without a zoneinfo
	// database, e.g. minimal containers.
	_ "time/tzdata"

	"github.com/mgierada/calculon/internal/symbolmap"
)

// closeGrace keeps a session open past its close for one last poll, so the
// closing price is stored even when the API reports it a few minutes late.
const closeGrace = 20 * time.Minute

// clock is a time of day on an exchange's local clock.
type clock struct{ hour, minute int }

// session is an exchange's regular trading hours, Monday to Friday.
// TODO: exchange holidays are not known, so those days are polled as usual.
type session struct {
	loc         *time.Location
	open, close clock
}

// sessions holds the regular hours of each exchange, by XTB symbol suffix.
var sessions = map[string]session{
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

func newSession(zone string, open, close clock) session {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		// The zones are constants and tzdata is embedded, so this is a typo.
		panic(err)
	}
	return session{loc: loc, open: open, close: close}
}

// sessionOf is the trading session of a symbol's exchange; false for symbols
// whose exchange is unknown, which are polled around the clock.
func sessionOf(symbol string) (session, bool) {
	s, ok := sessions[symbolmap.Suffix(symbol)]
	return s, ok
}

// window is the session on t's local day, from the open until the close plus
// grace, whether or not the exchange trades that day.
func (s session) window(t time.Time) (open, end time.Time) {
	y, m, d := t.In(s.loc).Date()
	open = time.Date(y, m, d, s.open.hour, s.open.minute, 0, 0, s.loc)
	closing := time.Date(y, m, d, s.close.hour, s.close.minute, 0, 0, s.loc)
	return open, closing.Add(closeGrace)
}

// trades reports whether the exchange holds a session on t's local day.
func (s session) trades(t time.Time) bool {
	switch t.In(s.loc).Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	return true
}

// sessionEnd is the end of the session t falls in, or false outside one.
func (s session) sessionEnd(t time.Time) (time.Time, bool) {
	open, end := s.window(t)
	if !s.trades(t) || t.Before(open) || !t.Before(end) {
		return time.Time{}, false
	}
	return end, true
}

// nextOpen is the first session open after t.
func (s session) nextOpen(t time.Time) time.Time {
	local := t.In(s.loc)
	for days := 0; ; days++ {
		day := local.AddDate(0, 0, days)
		if open, _ := s.window(day); s.trades(day) && open.After(t) {
			return open
		}
	}
}

// dueAt is when a symbol last fetched at last needs fetching again: an
// interval on while its session runs, the session's end for the closing price,
// and the next open once it is over. A symbol never fetched is due at once.
func dueAt(symbol string, last time.Time, interval time.Duration) time.Time {
	if last.IsZero() {
		return last
	}
	next := last.Add(interval)
	s, ok := sessionOf(symbol)
	if !ok {
		return next
	}
	if end, open := s.sessionEnd(last); open {
		return earliest(next, end)
	}
	return s.nextOpen(last)
}
