package widgets

import (
	"strings"
	"testing"
)

var testColumns = []Column{
	{Key: "symbol", Title: "Symbol", Flex: 1},
	{Key: "volume", Title: "Volume", Width: 10},
}

func TestTableRendersRows(t *testing.T) {
	table := NewTable("holdings", testColumns, []Row{
		{"symbol": "SNT.PL", "volume": "14"},
		{"symbol": "CDR.PL", "volume": "18"},
	})
	table.SetSize(60, 12)

	view := table.View()

	for _, want := range []string{"Symbol", "Volume", "SNT.PL", "CDR.PL", "holdings"} {
		if !strings.Contains(view, want) {
			t.Errorf("table view is missing %q:\n%s", want, view)
		}
	}
}

func TestTablePaginatesAndReportsPages(t *testing.T) {
	rows := make([]Row, 0, 30)
	for i := range 30 {
		rows = append(rows, Row{"symbol": "SYM", "volume": i})
	}
	table := NewTable("holdings", testColumns, rows)
	table.SetSize(60, 10)

	if got := table.footer(); !strings.Contains(got, "page 1/") {
		t.Errorf("footer = %q, want a page counter", got)
	}
}

func TestTableFooterHasNoPageCounterWhenEverythingFits(t *testing.T) {
	table := NewTable("holdings", testColumns, []Row{{"symbol": "SNT.PL", "volume": "14"}})
	table.SetSize(60, 20)

	if got := table.footer(); got != "holdings" {
		t.Errorf("footer = %q, want %q", got, "holdings")
	}
}

func TestBarChartSortsDescendingAndRenders(t *testing.T) {
	chart := NewBarChart("exposure", []Bar{
		{Label: "small", Value: 1},
		{Label: "big", Value: 10},
		{Label: "mid", Value: 5},
	})
	chart.SetSize(40, 10)

	if got := []string{chart.bars[0].Label, chart.bars[1].Label, chart.bars[2].Label}; got[0] != "big" ||
		got[1] != "mid" || got[2] != "small" {
		t.Errorf("bars ordered %v, want big, mid, small", got)
	}
	if view := chart.View(); !strings.Contains(view, "exposure") {
		t.Errorf("chart view is missing its title:\n%s", view)
	}
}

func TestBarChartHandlesNoData(t *testing.T) {
	chart := NewBarChart("exposure", nil)
	chart.SetSize(40, 10)

	if view := chart.View(); !strings.Contains(view, "exposure") {
		t.Errorf("empty chart view is missing its title:\n%s", view)
	}
}

func TestBarChartHandlesZeroSize(t *testing.T) {
	chart := NewBarChart("exposure", []Bar{{Label: "a", Value: 1}})
	chart.SetSize(0, 0)

	// Must not panic on a degenerate box.
	chart.View()
}
