package ui

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
)

// Render draws a horizontal bar chart of random values to stdout, colored as a
// red gradient from largest (brightest) to smallest (darkest).
func Render() {
	const barCount = 10
	const baseValue = 1.0
	const jitterRange = 1.0 // +/- 0.5 around baseValue

	palette := []string{"52", "88", "124", "160", "196"} // dark -> bright red

	bars := make([]barchart.BarData, 0, barCount)
	for i := 0; i < barCount; i++ {
		jitter := rand.Float64()*jitterRange - jitterRange/2
		value := baseValue + jitter

		bars = append(bars, barchart.BarData{
			Label: fmt.Sprintf("name_%d", i+1),
			Values: []barchart.BarValue{
				{fmt.Sprintf("Item%d", i), value, lipgloss.NewStyle()},
			},
		})
	}

	// Sort bars by value, descending.
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Values[0].Value > bars[j].Values[0].Value
	})

	// Color by sorted position so the chart reads as a gradient: largest value
	// gets the brightest red, smallest the darkest.
	for i := range bars {
		idx := len(palette) - 1 - i*len(palette)/barCount
		color := lipgloss.Color(palette[idx])
		bars[i].Values[0].Style = lipgloss.NewStyle().Foreground(color)
	}

	// Horizontal bars: thickness = height/barCount. Gap 1 needs a row per bar
	// plus a row per gap, so height = barCount*2 keeps every value visible.
	// Width 20 keeps it small; bar lengths scale relative to the max value.
	bc := barchart.New(20, barCount*2)
	bc.PushAll(bars)
	// SetHorizontal must run after PushAll so the left axis reserves space for
	// the label width (names on the left axis).
	bc.SetHorizontal(true)
	bc.SetBarGap(0)
	bc.Draw()

	// With barWidth 1 and barGap 1, bar k renders on row k*2; gap rows are odd.
	// Append each bar's value on the right, aligned past the chart width.
	valueByRow := make(map[int]float64, len(bars))
	for k, b := range bars {
		valueByRow[k*2] = b.Values[0].Value
	}

	chartWidth := bc.Width()
	for row, line := range strings.Split(bc.View(), "\n") {
		value, isBar := valueByRow[row]
		if !isBar {
			fmt.Println(line)
			continue
		}
		pad := chartWidth - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		fmt.Printf("%s%s %.2f\n", line, strings.Repeat(" ", pad), value)
	}
}
