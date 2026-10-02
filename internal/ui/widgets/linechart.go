package widgets

import (
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"
)

// seriesPalette colours datasets in the order they are added.
var seriesPalette = []color.Color{
	lipgloss.Color("39"),
	lipgloss.Color("214"),
	lipgloss.Color("42"),
	lipgloss.Color("170"),
	lipgloss.Color("203"),
	lipgloss.Color("116"),
}

// Point is one value of a time series.
type Point struct {
	Time  time.Time
	Value float64
}

// Series is a named line.
type Series struct {
	Name   string
	Points []Point
}

// LineChart plots one or more time series against a shared time axis, with a
// legend on the title line.
type LineChart struct {
	title   string
	series  []Series
	format  func(float64) string
	width   int
	height  int
	focused bool
	// rendered caches the drawn chart, which only changes with size.
	rendered string
}

// NewLineChart builds a chart over the given series.
func NewLineChart(title string, series ...Series) *LineChart {
	return &LineChart{title: title, series: series, format: formatCompact}
}

// WithFormat sets how Y axis labels read.
func (c *LineChart) WithFormat(format func(float64) string) *LineChart {
	c.format = format
	return c
}

// Init implements ui.Component.
func (c *LineChart) Init() tea.Cmd {
	return nil
}

// Update implements ui.Component. The chart is read-only, so it ignores input.
func (c *LineChart) Update(tea.Msg) tea.Cmd {
	return nil
}

// SetFocused implements ui.Component.
func (c *LineChart) SetFocused(focused bool) {
	c.focused = focused
}

// SetSize implements ui.Component, redrawing the chart for the new box.
func (c *LineChart) SetSize(width, height int) {
	c.width, c.height = width, height
	c.rendered = c.render()
}

// View implements ui.Component.
func (c *LineChart) View() string {
	return frame(c.width, c.height, c.focused).Render(c.header() + "\n" + c.rendered)
}

// header is the title followed by a coloured legend entry per series.
func (c *LineChart) header() string {
	parts := []string{titleStyle.Render(c.title)}
	for i, s := range c.series {
		marker := lipgloss.NewStyle().Foreground(seriesPalette[i%len(seriesPalette)]).Render("━━")
		parts = append(parts, marker+" "+s.Name)
	}
	return strings.Join(parts, "   ")
}

// render draws every series with braille dots for a smooth line.
func (c *LineChart) render() string {
	innerWidth, innerHeight := innerSize(c.width, c.height)
	chartHeight := innerHeight - 1
	minT, maxT, minV, maxV, ok := c.bounds()
	if !ok || innerWidth < 10 || chartHeight < 3 {
		return lipgloss.NewStyle().Foreground(mutedColor).Render("no data")
	}

	pad := (maxV - minV) * 0.05
	if pad == 0 {
		pad = math.Max(math.Abs(maxV)*0.05, 1)
	}
	lower := minV - pad
	if minV >= 0 {
		// Padding must not invent negative values for a series that has none.
		lower = math.Max(lower, 0)
	}
	chart := timeserieslinechart.New(innerWidth, chartHeight,
		timeserieslinechart.WithTimeRange(minT, maxT),
		timeserieslinechart.WithYRange(lower, maxV+pad),
		timeserieslinechart.WithYLabelFormatter(func(_ int, v float64) string { return c.format(v) }),
		timeserieslinechart.WithAxesStyles(
			lipgloss.NewStyle().Foreground(unfocusedBorder),
			lipgloss.NewStyle().Foreground(mutedColor),
		),
	)
	for i, s := range c.series {
		chart.SetDataSetStyle(s.Name, lipgloss.NewStyle().Foreground(seriesPalette[i%len(seriesPalette)]))
		for _, p := range s.Points {
			chart.PushDataSet(s.Name, timeserieslinechart.TimePoint{Time: p.Time, Value: p.Value})
		}
	}
	chart.DrawBrailleAll()
	return chart.View()
}

// bounds spans every point of every series.
func (c *LineChart) bounds() (minT, maxT time.Time, minV, maxV float64, ok bool) {
	for _, s := range c.series {
		for _, p := range s.Points {
			if !ok {
				minT, maxT, minV, maxV, ok = p.Time, p.Time, p.Value, p.Value, true
				continue
			}
			if p.Time.Before(minT) {
				minT = p.Time
			}
			if p.Time.After(maxT) {
				maxT = p.Time
			}
			minV, maxV = math.Min(minV, p.Value), math.Max(maxV, p.Value)
		}
	}
	if ok && !maxT.After(minT) {
		maxT = minT.Add(24 * time.Hour)
	}
	return minT, maxT, minV, maxV, ok
}

// formatCompact shortens large axis values, e.g. 132651 to 132.7k.
func formatCompact(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1e6:
		return trimFloat(value/1e6) + "M"
	case abs >= 1e3:
		return trimFloat(value/1e3) + "k"
	default:
		return trimFloat(value)
	}
}

func trimFloat(value float64) string {
	s := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(value, 'f', 1, 64), "0"), ".")
	if s == "-0" {
		return "0"
	}
	return s
}
