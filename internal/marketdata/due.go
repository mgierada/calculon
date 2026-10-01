package marketdata

import (
	"time"

	"github.com/mgierada/calculon/internal/markethours"
)

// dueAt is when a symbol last fetched at last needs fetching again: an
// interval on while its session runs, the session's end for the closing price,
// and the next open once it is over. A symbol never fetched is due at once,
// and one whose exchange is unknown every interval.
func dueAt(symbol string, last time.Time, interval time.Duration) time.Time {
	if last.IsZero() {
		return last
	}
	next := last.Add(interval)
	s, ok := markethours.For(symbol)
	if !ok {
		return next
	}
	if end, open := s.End(last); open {
		return earliest(next, end)
	}
	return s.NextOpen(last)
}
