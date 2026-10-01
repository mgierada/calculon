package dashboards

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// unknown stands in for a figure that cannot be computed yet.
const unknown = "—"

// money renders an amount with two decimals and thin grouping, e.g.
// "132 651.90 PLN". An empty currency leaves the code off.
func money(amount float64, currency string) string {
	s := groupThousands(strconv.FormatFloat(math.Abs(amount), 'f', 2, 64))
	if amount < 0 && math.Abs(amount) >= 0.005 {
		s = "-" + s
	}
	if currency != "" {
		s += " " + currency
	}
	return s
}

// signedMoney is money with an explicit plus for gains.
func signedMoney(amount float64, currency string) string {
	if amount >= 0.005 {
		return "+" + money(amount, currency)
	}
	return money(amount, currency)
}

// percentCap is the largest percentage shown in full. Beyond it, e.g. a lot
// received through a spin-off at a near-zero cost basis, the digits carry no
// information and only overflow their column.
const percentCap = 9999

// percent renders a share, e.g. "12.3%".
func percent(value float64) string {
	if math.Abs(value) > percentCap {
		sign := ""
		if value < 0 {
			sign = "-"
		}
		return ">" + sign + strconv.Itoa(percentCap) + "%"
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + "%"
}

// signedPercent is percent with an explicit plus for gains.
func signedPercent(value float64) string {
	if value > percentCap {
		return ">+" + strconv.Itoa(percentCap) + "%"
	}
	if value >= 0.05 {
		return "+" + percent(value)
	}
	return percent(value)
}

// volume prints share counts without trailing zeros, since fractional share
// purchases produce volumes like 1.6757 alongside whole ones like 10.
func volume(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e4)/1e4, 'f', -1, 64)
}

// price prints a unit price with two to four decimals, enough for fractional
// fills of cheap stocks without showing float noise from averaging.
func price(v float64) string {
	s := strconv.FormatFloat(math.Round(v*1e4)/1e4, 'f', -1, 64)
	if dot := strings.IndexByte(s, '.'); dot == -1 {
		s += ".00"
	} else if len(s)-dot-1 < 2 {
		s += strings.Repeat("0", 2-(len(s)-dot-1))
	}
	return s
}

func date(t time.Time) string {
	if t.IsZero() {
		return unknown
	}
	return t.Format("2006-01-02")
}

func dateTime(t time.Time) string {
	if t.IsZero() {
		return unknown
	}
	return t.Format("2006-01-02 15:04")
}

// deltaCell renders a change as a toned table cell.
func deltaCell(d portfolio.Delta, currency string) any {
	if !d.Known {
		return widgets.Toned(unknown, widgets.Muted)
	}
	return widgets.Toned(signedMoney(d.Amount, currency), widgets.ToneOf(d.Amount))
}

// deltaPctCell renders a change's percentage as a toned table cell.
func deltaPctCell(d portfolio.Delta) any {
	if !d.Known {
		return widgets.Toned(unknown, widgets.Muted)
	}
	return widgets.Toned(signedPercent(d.Pct), widgets.ToneOf(d.Amount))
}

// staleDay labels a day change from an earlier session with that session's
// date, e.g. "29 Sep".
func staleDay(day time.Time) string {
	return day.Format("2 Jan")
}

// dayCell renders a holding's day change, muted and dated when it is the move
// of an earlier session rather than today's.
func dayCell(h portfolio.Holding) any {
	if !h.DayStale {
		return deltaPctCell(h.Day)
	}
	return widgets.Toned(signedPercent(h.Day.Pct)+" "+staleDay(h.DayAsOf), widgets.Muted)
}

// dayStat is a holding's day change card, noting the session it is from when
// that is not the latest.
func dayStat(h portfolio.Holding) *widgets.Stat {
	if !h.Day.Known || !h.DayStale {
		return deltaStat("Today", h.Day, h.Account.Currency, dayUnknownNote)
	}
	return widgets.NewStat("Today", signedMoney(h.Day.Amount, h.Account.Currency)).
		WithNote(signedPercent(h.Day.Pct)+" · as of "+staleDay(h.DayAsOf), widgets.Muted)
}

// totalDayStat is the portfolio's day change card, saying how many holdings
// it leaves out for having only an earlier session's price.
func totalDayStat(t portfolio.Totals, base string) *widgets.Stat {
	if t.DayStale == 0 {
		return deltaStat("Today", t.Day, base, dayUnknownNote)
	}
	excluded := fmt.Sprintf("%d stale excluded", t.DayStale)
	if !t.Day.Known {
		return widgets.NewStat("Today", unknown).WithNote(excluded, widgets.Muted)
	}
	return widgets.NewStat("Today", signedMoney(t.Day.Amount, base)).
		WithNote(signedPercent(t.Day.Pct)+" · "+excluded, widgets.ToneOf(t.Day.Amount))
}

// deltaStat is a KPI card for a change that may be unknown.
func deltaStat(label string, d portfolio.Delta, currency, unknownNote string) *widgets.Stat {
	if !d.Known {
		return widgets.NewStat(label, unknown).WithNote(unknownNote, widgets.Muted)
	}
	return widgets.NewStat(label, signedMoney(d.Amount, currency)).
		WithNote(signedPercent(d.Pct), widgets.ToneOf(d.Amount))
}

// groupThousands inserts a space between every three integer digits.
func groupThousands(s string) string {
	integer, fraction, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, digit := range integer {
		if i > 0 && (len(integer)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(digit)
	}
	if fraction != "" {
		b.WriteString("." + fraction)
	}
	return b.String()
}
