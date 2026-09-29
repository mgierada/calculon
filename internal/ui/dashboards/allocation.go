package dashboards

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// groupingKey cycles what the allocation chart groups by.
const groupingKey = "g"

// stackSegments is how many slices the one-line allocation bar shows before
// folding the rest into "other".
const stackSegments = 9

// AllocationChart shows each group's share of the portfolio as bars, and
// cycles the grouping (symbol, account, category, currency) on g.
type AllocationChart struct {
	*widgets.BarChart
	report   *portfolio.Report
	grouping int
}

// NewAllocationChart builds the chart grouped by symbol.
func NewAllocationChart(report *portfolio.Report) *AllocationChart {
	chart := &AllocationChart{
		BarChart: widgets.NewBarChart("", nil).WithFormat(percent),
		report:   report,
	}
	chart.refresh()
	return chart
}

// Update implements ui.Component, handling the grouping toggle.
func (a *AllocationChart) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == groupingKey {
		a.grouping = (a.grouping + 1) % len(portfolio.Groupings)
		a.refresh()
	}
	return nil
}

func (a *AllocationChart) refresh() {
	grouping := portfolio.Groupings[a.grouping]
	a.SetTitle(fmt.Sprintf("%% of portfolio by %s · g to change", grouping))
	a.SetBars(slicesToBars(portfolio.Allocation(*a.report, grouping)))
}

// AllocationBar is the whole portfolio as one stacked bar, by symbol.
func AllocationBar(report *portfolio.Report) *widgets.StackBar {
	slices := portfolio.TopSlices(portfolio.Allocation(*report, portfolio.BySymbol), stackSegments)
	return widgets.NewStackBar("allocation", slicesToBars(slices))
}

// slicesToBars keeps positive slices; a slightly negative cash balance has no
// meaningful share to draw.
func slicesToBars(slices []portfolio.Slice) []widgets.Bar {
	bars := make([]widgets.Bar, 0, len(slices))
	for _, s := range slices {
		if s.Weight > 0 {
			bars = append(bars, widgets.Bar{Label: s.Label, Value: s.Weight})
		}
	}
	return bars
}
