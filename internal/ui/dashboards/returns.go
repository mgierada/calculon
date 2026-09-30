package dashboards

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// metricKey cycles the return chart through TWR, XIRR and CAGR.
const metricKey = "m"

// ReturnChart plots the return since inception as of every day. It starts on
// the time-weighted return and cycles through the metrics on m.
type ReturnChart struct {
	*widgets.LineChart
	report  *portfolio.Report
	metric  int
	width   int
	height  int
	focused bool
}

// NewReturnChart builds the chart showing the default metric, TWR.
func NewReturnChart(report *portfolio.Report) *ReturnChart {
	chart := &ReturnChart{report: report}
	chart.refresh()
	return chart
}

// Update implements ui.Component, handling the metric toggle.
func (r *ReturnChart) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == metricKey {
		r.metric = (r.metric + 1) % len(portfolio.ReturnMetrics)
		r.refresh()
	}
	return nil
}

// SetSize implements ui.Component, remembering the box for redraws.
func (r *ReturnChart) SetSize(width, height int) {
	r.width, r.height = width, height
	r.LineChart.SetSize(width, height)
}

// SetFocused implements ui.Component, remembering focus for redraws.
func (r *ReturnChart) SetFocused(focused bool) {
	r.focused = focused
	r.LineChart.SetFocused(focused)
}

// refresh redraws the chart for the current metric.
func (r *ReturnChart) refresh() {
	metric := portfolio.ReturnMetrics[r.metric]
	other := portfolio.ReturnMetrics[(r.metric+1)%len(portfolio.ReturnMetrics)]
	series := widgets.Series{Name: metric.String()}
	for _, p := range r.report.Returns.Series {
		if value, ok := p.Value(metric); ok {
			series.Points = append(series.Points, widgets.Point{Time: p.Time, Value: value})
		}
	}

	unit := "% since start"
	if metric.Annualized() {
		unit = "% a year"
	}
	title := fmt.Sprintf("%s over time, %s · m for %s", metric, unit, other)
	r.LineChart = widgets.NewLineChart(title, series).WithFormat(func(v float64) string {
		return fmt.Sprintf("%.0f%%", v)
	})
	r.LineChart.SetSize(r.width, r.height)
	r.LineChart.SetFocused(r.focused)
}

// ReturnStat is the KPI card for returns: the cumulative TWR up front, the
// annual rates in the note once there is enough history for them.
func ReturnStat(report *portfolio.Report) *widgets.Stat {
	r := report.Returns
	if !r.TWR.Known {
		return widgets.NewStat("Return (TWR)", unknown).WithNote("no history yet", widgets.Muted)
	}
	note := fmt.Sprintf("annual after %d days", portfolio.MinReturnDays)
	if r.XIRR.Known || r.CAGR.Known {
		note = fmt.Sprintf("XIRR %s · CAGR %s", annualCell(r.XIRR), annualCell(r.CAGR))
	}
	return widgets.NewStat("Return (TWR)", signedPercent(r.TWR.Pct)).
		WithNote(note, widgets.ToneOf(r.TWR.Pct))
}

// annualCell is an annual rate for the note, or a dash while it is unknown.
func annualCell(d portfolio.Delta) string {
	if !d.Known {
		return unknown
	}
	return signedPercent(d.Pct)
}
