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

// Bar is one labelled value in a BarChart.
type Bar struct {
	Label string
	Value float64
}

// BarChart draws horizontal bars sorted largest first, with the value printed
// to the right of each bar.
type BarChart struct {
	bars    []Bar
	title   string
	width   int
	height  int
	focused bool
}

// NewBarChart builds a bar chart from labelled values.
func NewBarChart(title string, bars []Bar) *BarChart {
	return &BarChart{title: title, bars: sortedDescending(bars)}
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
	border := unfocusedBorder
	if b.focused {
		border = focusedBorder
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(max(b.width-2, 0)).
		Height(max(b.height-2, 0))

	if len(b.bars) == 0 || b.width <= 2 || b.height <= 2 {
		return box.Render(b.title)
	}

	return box.Render(b.render())
}

// render draws the chart and appends each bar's value on the right.
func (b *BarChart) render() string {
	// One row per bar with no gap, minus the title line.
	chartHeight := max(b.height-3, len(b.bars))
	chart := barchart.New(max(b.width-12, 1), chartHeight)
	chart.PushAll(b.barData())
	// SetHorizontal must run after PushAll so the axis reserves label width.
	chart.SetHorizontal(true)
	chart.SetBarGap(0)
	chart.Draw()

	valueByRow := make(map[int]float64, len(b.bars))
	for i, bar := range b.bars {
		valueByRow[i] = bar.Value
	}

	chartWidth := chart.Width()
	lines := strings.Split(chart.View(), "\n")
	rendered := make([]string, 0, len(lines)+1)
	rendered = append(rendered, lipgloss.NewStyle().Foreground(headerColor).Render(b.title))
	for row, line := range lines {
		value, isBar := valueByRow[row]
		if !isBar {
			rendered = append(rendered, line)
			continue
		}
		pad := max(chartWidth-lipgloss.Width(line), 0)
		rendered = append(rendered, fmt.Sprintf("%s%s %.2f", line, strings.Repeat(" ", pad), value))
	}

	return strings.Join(rendered, "\n")
}

// barData converts the bars to chart input, coloring by rank so the chart reads
// as a gradient from largest to smallest.
func (b *BarChart) barData() []barchart.BarData {
	data := make([]barchart.BarData, 0, len(b.bars))
	for i, bar := range b.bars {
		shade := len(gradient) - 1 - i*len(gradient)/len(b.bars)
		style := lipgloss.NewStyle().Foreground(gradient[shade])
		data = append(data, barchart.BarData{
			Label:  bar.Label,
			Values: []barchart.BarValue{{Name: bar.Label, Value: bar.Value, Style: style}},
		})
	}
	return data
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
