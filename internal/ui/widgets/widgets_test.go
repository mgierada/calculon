package widgets

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func enter() tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
}

func TestTableOnSelectReceivesHiddenKeys(t *testing.T) {
	var selected Row
	table := NewTable("holdings", testColumns, []Row{
		{"symbol": "SNT.PL", "volume": "14", "_id": 7},
	}).OnSelect(func(row Row) tea.Cmd {
		selected = row
		return nil
	})
	table.SetSize(60, 12)
	table.SetFocused(true)

	table.Update(enter())

	if selected["_id"] != 7 {
		t.Errorf("selected row = %v, want the row with its hidden id", selected)
	}
}

func TestTableFilterCapturesInput(t *testing.T) {
	table := NewTable("holdings", testColumns, []Row{
		{"symbol": "SNT.PL", "volume": "14"},
		{"symbol": "CDR.PL", "volume": "18"},
	}).Filterable()
	table.SetSize(60, 12)
	table.SetFocused(true)

	table.Update(tea.KeyPressMsg(tea.Key{Text: "/", Code: '/'}))
	if !table.CapturingInput() {
		t.Fatal("table does not capture input after /")
	}
	for _, r := range "CDR" {
		table.Update(tea.KeyPressMsg(tea.Key{Text: string(r), Code: r}))
	}
	if view := table.View(); strings.Contains(view, "SNT.PL") || !strings.Contains(view, "CDR.PL") {
		t.Errorf("filtered view should show only CDR.PL:\n%s", view)
	}
}

func TestStackBarFillsWidthExactly(t *testing.T) {
	bar := NewStackBar("allocation", []Bar{{"a", 1}, {"b", 1}, {"c", 1}, {"cash", -5}})

	got := bar.bar(10)
	if width := lipgloss.Width(got); width != 10 {
		t.Errorf("bar width = %d, want 10", width)
	}
	if len(bar.segments) != 3 {
		t.Errorf("kept %d segments, want the negative one dropped", len(bar.segments))
	}
}

func TestStackBarLegendFitsWidth(t *testing.T) {
	bar := NewStackBar("allocation", []Bar{{"AAAAAAAA", 5}, {"BBBBBBBB", 3}, {"CCCCCCCC", 2}})

	if width := lipgloss.Width(bar.legend(40)); width > 40 {
		t.Errorf("legend width = %d, want at most 40", width)
	}
}

func TestStatRendersLabelValueAndNote(t *testing.T) {
	stat := NewStat("Total value", "1 234.00 PLN").WithNote("+5.0%", Positive)
	stat.SetSize(30, 5)

	view := stat.View()
	for _, want := range []string{"Total value", "1 234.00 PLN", "+5.0%"} {
		if !strings.Contains(view, want) {
			t.Errorf("stat view is missing %q:\n%s", want, view)
		}
	}
	if w, h := lipgloss.Width(view), lipgloss.Height(view); w != 30 || h != 5 {
		t.Errorf("stat is %dx%d, want 30x5", w, h)
	}
}

func TestLineChartRendersSeries(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	series := Series{Name: "value"}
	for i := range 30 {
		series.Points = append(series.Points, Point{Time: start.AddDate(0, 0, i), Value: float64(1000 + i*10)})
	}
	chart := NewLineChart("value over time", series)
	chart.SetSize(60, 15)

	view := chart.View()
	if !strings.Contains(view, "value over time") || strings.Contains(view, "no data") {
		t.Errorf("line chart view:\n%s", view)
	}
	if w, h := lipgloss.Width(view), lipgloss.Height(view); w != 60 || h != 15 {
		t.Errorf("line chart is %dx%d, want 60x15", w, h)
	}
}

func TestLineChartHandlesNoData(t *testing.T) {
	chart := NewLineChart("value over time")
	chart.SetSize(60, 15)

	if view := chart.View(); !strings.Contains(view, "no data") {
		t.Errorf("empty chart does not say so:\n%s", view)
	}
}

func TestFormatCompact(t *testing.T) {
	tests := map[float64]string{132651: "132.7k", 950: "950", 2_500_000: "2.5M", -1200: "-1.2k", 0: "0"}
	for value, want := range tests {
		if got := formatCompact(value); got != want {
			t.Errorf("formatCompact(%v) = %q, want %q", value, got, want)
		}
	}
}
