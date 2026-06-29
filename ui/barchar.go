package main

import (
	"fmt"
	"math/rand"
	"sort"

	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/barchart"
)

func main() {
	const barCount = 10
	const baseValue = 1.0
	const jitterRange = 1.0 // +/- 0.5 around baseValue

	palette := []string{"52", "88", "124", "160", "196"} // dark -> bright red

	bars := make([]barchart.BarData, 0, barCount)
	for i := 0; i < barCount; i++ {
		jitter := rand.Float64()*jitterRange - jitterRange/2
		value := baseValue + jitter
		color := lipgloss.Color(palette[i%len(palette)])

		bars = append(bars, barchart.BarData{
			Label: fmt.Sprintf("B%d", i),
			Values: []barchart.BarValue{
				{fmt.Sprintf("Item%d", i), value, lipgloss.NewStyle().Foreground(color)},
			},
		})
	}

	// Sort bars by value, descending.
	sort.Slice(bars, func(i, j int) bool {
		return bars[i].Values[0].Value > bars[j].Values[0].Value
	})

	// Horizontal bars: thickness = height/barCount. Gap 1 needs a row per bar
	// plus a row per gap, so height = barCount*2 keeps every value visible.
	// Width 20 keeps it small; bar lengths scale relative to the max value.
	bc := barchart.New(20, barCount*2, barchart.WithHorizontalBars(), barchart.WithBarGap(1))
	bc.PushAll(bars)
	bc.Draw()

	fmt.Println(bc.View())
}
