package dashboards

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// metricKey cycles the return chart through TWR, XIRR and CAGR.
const metricKey = "m"

// ReturnChart plots the return since inception as of every day: the combined
// portfolio, and on the summary of several accounts each account beside it.
// It starts on the time-weighted return and cycles through the metrics on m.
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

// HandlesShortcut implements ui.ShortcutHandler: m works from anywhere on the
// screen.
func (r *ReturnChart) HandlesShortcut(key string) bool {
	return key == metricKey
}

// State implements ui.Stateful, so a refresh keeps the metric.
func (r *ReturnChart) State() any {
	return returnState(r.metric)
}

// Restore implements ui.Stateful.
func (r *ReturnChart) Restore(state any) {
	if metric, ok := state.(returnState); ok && int(metric) < len(portfolio.ReturnMetrics) {
		r.metric = int(metric)
		r.refresh()
	}
}

// returnState is the metric a return chart shows.
type returnState int

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
	series := []widgets.Series{returnSeries(metric.String(), r.report.Returns, metric)}
	if len(r.report.AccountReturns) > 0 {
		series[0].Name = "all"
	}
	for _, account := range r.report.AccountReturns {
		series = append(series, returnSeries(accountName(account.Account), account.Returns, metric))
	}

	unit := "% since start"
	if metric.Annualized() {
		unit = "% a year"
	}
	title := fmt.Sprintf("%s over time, %s · m for %s", metric, unit, other)
	r.LineChart = widgets.NewLineChart(title, series...).WithFormat(func(v float64) string {
		return fmt.Sprintf("%.0f%%", v)
	})
	r.LineChart.SetSize(r.width, r.height)
	r.LineChart.SetFocused(r.focused)
}

// returnSeries is one line of a metric over time.
func returnSeries(name string, returns portfolio.Returns, metric portfolio.ReturnMetric) widgets.Series {
	series := widgets.Series{Name: name}
	for _, p := range returns.Series {
		if value, ok := p.Value(metric); ok {
			series.Points = append(series.Points, widgets.Point{Time: p.Time, Value: value})
		}
	}
	return series
}

// accountName is the short name a chart legend gives an account: its label
// when it has one, otherwise its id.
func accountName(account model.Account) string {
	if account.Name != "" {
		return account.Name
	}
	return account.ID
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
