package widgets

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
)

// gradient runs dark red to bright red; the largest bar gets the brightest.
var gradient = []color.Color{
	lipgloss.Color("52"),
	lipgloss.Color("88"),
	lipgloss.Color("124"),
	lipgloss.Color("160"),
	lipgloss.Color("196"),
}

// Bar is one labelled value in a BarChart or a StackBar segment. Color sets
// a segment's colour; nil takes the next of the palette.
type Bar struct {
	Label string
	Value float64
	Color color.Color
}

// BarChart draws horizontal bars sorted largest first, with the value printed
// to the right of each bar.
type BarChart struct {
	bars    []Bar
	title   string
	format  func(float64) string
	width   int
	height  int
	focused bool
}

// NewBarChart builds a bar chart from labelled values.
func NewBarChart(title string, bars []Bar) *BarChart {
	return &BarChart{title: title, bars: sortedDescending(bars), format: formatTwoDecimals}
}

// WithFormat sets how the value printed beside each bar reads.
func (b *BarChart) WithFormat(format func(float64) string) *BarChart {
	b.format = format
	return b
}

// SetTitle replaces the chart title.
func (b *BarChart) SetTitle(title string) {
	b.title = title
}

// SetBars replaces the chart contents.
func (b *BarChart) SetBars(bars []Bar) {
	b.bars = sortedDescending(bars)
}

// Init implements ui.Component.
func (b *BarChart) Init() tea.Cmd {
	return nil
}

// Update implements ui.Component. The chart is read-only, so it ignores input.
func (b *BarChart) Update(tea.Msg) tea.Cmd {
	return nil
}

// SetSize implements ui.Component.
func (b *BarChart) SetSize(width, height int) {
	b.width, b.height = width, height
}

// SetFocused implements ui.Component.
func (b *BarChart) SetFocused(focused bool) {
	b.focused = focused
}

// View implements ui.Component.
func (b *BarChart) View() string {
	box := frame(b.width, b.height, b.focused)
	if len(b.bars) == 0 || b.width <= 2 || b.height <= 3 {
		return box.Render(titleStyle.Render(b.title))
	}
	return box.Render(b.render())
}

// render draws the chart and appends each bar's value on the right.
func (b *BarChart) render() string {
	// One row per bar with no gap, below the title line. Bars that do not fit
	// are dropped from the bottom, which holds the smallest.
	innerWidth, innerHeight := innerSize(b.width, b.height)
	bars := b.bars[:min(len(b.bars), innerHeight-1)]
	valueWidth := 0
	for _, bar := range bars {
		valueWidth = max(valueWidth, lipgloss.Width(b.format(bar.Value)))
	}
	chart := barchart.New(max(innerWidth-valueWidth-1, 1), len(bars))
	chart.PushAll(barData(bars))
	// SetHorizontal must run after PushAll so the axis reserves label width.
	chart.SetHorizontal(true)
	chart.SetBarGap(0)
	chart.Draw()

	valueByRow := make(map[int]float64, len(bars))
	for i, bar := range bars {
		valueByRow[i] = bar.Value
	}

	chartWidth := chart.Width()
	lines := strings.Split(chart.View(), "\n")
	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, titleStyle.Render(b.title))
	for row, line := range lines {
		value, isBar := valueByRow[row]
		if !isBar {
			rendered = append(rendered, line)
			continue
		}
		pad := max(chartWidth-lipgloss.Width(line), 0)
		rendered = append(rendered, fmt.Sprintf("%s%s %s", line, strings.Repeat(" ", pad), b.format(value)))
	}

	return strings.Join(rendered, "\n")
}

// barData converts the bars to chart input, coloring by rank so the chart reads
// as a gradient from largest to smallest.
func barData(bars []Bar) []barchart.BarData {
	data := make([]barchart.BarData, 0, len(bars))
	for i, bar := range bars {
		shade := len(gradient) - 1 - i*len(gradient)/len(bars)
		style := lipgloss.NewStyle().Foreground(gradient[shade])
		data = append(data, barchart.BarData{
			Label:  bar.Label,
			Values: []barchart.BarValue{{Name: bar.Label, Value: bar.Value, Style: style}},
		})
	}
	return data
}

func formatTwoDecimals(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

// sortedDescending copies the bars ordered largest value first.
func sortedDescending(bars []Bar) []Bar {
	sorted := make([]Bar, len(bars))
	copy(sorted, bars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})
	return sorted
}
