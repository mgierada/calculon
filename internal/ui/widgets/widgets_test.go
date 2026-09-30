package widgets

import (
	"fmt"
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

// highlightSGR is the escape sequence of the highlight background.
const highlightSGR = "48;5;238"

func TestTableHighlightsCursorRowOnlyWhenFocused(t *testing.T) {
	table := NewTable("holdings", testColumns, []Row{
		{"symbol": "SNT.PL", "volume": Toned("14", Positive)},
		{"symbol": "CDR.PL", "volume": "18"},
	})
	table.SetSize(60, 12)

	if strings.Contains(table.View(), highlightSGR) {
		t.Error("unfocused table draws a highlighted row")
	}

	table.SetFocused(true)
	lines := strings.Split(table.View(), "\n")
	var highlighted []string
	for _, line := range lines {
		if strings.Contains(line, highlightSGR) {
			highlighted = append(highlighted, line)
		}
	}
	if len(highlighted) != 1 || !strings.Contains(highlighted[0], "SNT.PL") {
		t.Errorf("highlighted lines = %q, want just the SNT.PL row", highlighted)
	}

	table.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	for _, line := range strings.Split(table.View(), "\n") {
		if strings.Contains(line, highlightSGR) && !strings.Contains(line, "CDR.PL") {
			t.Errorf("after j the highlight is on %q, want the CDR.PL row", line)
		}
	}
}

func pickerItems(n int) []PickerItem {
	items := make([]PickerItem, n)
	for i := range items {
		items[i] = PickerItem{Key: fmt.Sprintf("k%d", i), Label: fmt.Sprintf("action %d", i), Group: "G"}
	}
	return items
}

func pickerKey(key string) tea.KeyPressMsg {
	switch key {
	case "enter":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
	case "esc":
		return tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
	}
	return tea.KeyPressMsg(tea.Key{Text: key, Code: rune(key[0])})
}

func TestPickerScrollsToKeepCursorVisible(t *testing.T) {
	picker := NewPicker("Keybindings", "Press / to search", pickerItems(50), false)
	picker.SetSize(80, 24)

	view := picker.View()
	if lipgloss.Height(view) > 24 || !strings.Contains(view, "action 0") || strings.Contains(view, "action 49") {
		t.Fatalf("initial view:\n%s", view)
	}
	if !strings.Contains(view, "┃") {
		t.Error("long list has no scrollbar thumb")
	}

	picker.Update(pickerKey("G"))
	if view := picker.View(); !strings.Contains(view, "action 49") {
		t.Errorf("after G the last item is not visible:\n%s", view)
	}
}

func TestPickerSearchAndChoose(t *testing.T) {
	items := []PickerItem{{Label: "All accounts"}, {Label: "IKE 51099570", Value: 2}, {Label: "USD 51727538", Value: 3}}
	picker := NewPicker("Account", "", items, true)

	for _, key := range []string{"/", "u", "s", "d", "enter"} {
		picker.Update(pickerKey(key))
	}
	result, item := picker.Update(pickerKey("enter"))
	if result != PickerChosen || item == nil || item.Value != 3 {
		t.Errorf("chose %v %+v, want the USD account", result, item)
	}
}

func TestPickerStartsOnCurrentItem(t *testing.T) {
	items := []PickerItem{{Label: "a"}, {Label: "b", Current: true}, {Label: "c"}}
	picker := NewPicker("Account", "", items, true)

	if _, item := picker.Update(pickerKey("enter")); item == nil || item.Label != "b" {
		t.Errorf("picked %+v, want the current item b", item)
	}
}

func TestPickerNoMatches(t *testing.T) {
	picker := NewPicker("Keybindings", "", pickerItems(3), false)
	for _, key := range []string{"/", "z", "z", "z"} {
		picker.Update(pickerKey(key))
	}

	if view := picker.View(); !strings.Contains(view, "no matches") {
		t.Errorf("view:\n%s", view)
	}
	if result, _ := picker.Update(pickerKey("enter")); result != PickerOpen {
		t.Error("enter in the search box closed the picker")
	}
	if result, _ := picker.Update(pickerKey("q")); result != PickerClosed {
		t.Error("q after searching did not close the picker")
	}
}

func TestPickerFitsSmallScreens(t *testing.T) {
	picker := NewPicker("Keybindings", "Press / to search", pickerItems(50), false)
	picker.SetSize(40, 14)

	view := picker.View()
	if w, h := lipgloss.Width(view), lipgloss.Height(view); w > 40 || h > 14 {
		t.Errorf("picker is %dx%d, want within 40x14", w, h)
	}
}

func TestPickerChromeDoesNotWrap(t *testing.T) {
	picker := NewPicker("Account", "Every dashboard shows the chosen account", []PickerItem{{Label: "a"}}, true)
	picker.SetSize(120, 30)

	view := picker.View()
	for _, want := range []string{"Every dashboard shows the chosen account", "esc/q close"} {
		if !strings.Contains(view, want) {
			t.Errorf("picker wraps %q:\n%s", want, view)
		}
	}
}

func sortTestTable() *Table {
	columns := []Column{
		{Key: "symbol", Title: "Symbol", Flex: 1, Left: true},
		{Key: "value", Title: "Value", Flex: 1},
		{Key: "day", Title: "Today", Flex: 1},
	}
	rows := []Row{
		{"symbol": "b.PL", "value": "900.00 PLN", SortBy("value"): 900.0, "day": "—", SortBy("day"): nil},
		{"symbol": "A.PL", "value": "1 000.00 PLN", SortBy("value"): 1000.0, "day": Toned("+1%", Positive), SortBy("day"): 1.0},
		{"symbol": "C.PL", "value": "95.00 PLN", SortBy("value"): 95.0, "day": "-2%", SortBy("day"): -2.0},
	}
	table := NewTable("positions", columns, rows).Sortable()
	table.SetSize(80, 12)
	table.SetFocused(true)
	return table
}

// order lists the symbols as the view shows them, top to bottom.
func order(t *testing.T, table *Table) []string {
	t.Helper()
	var symbols []string
	for _, line := range strings.Split(table.View(), "\n") {
		for _, symbol := range []string{"A.PL", "b.PL", "C.PL"} {
			if strings.Contains(line, symbol) {
				symbols = append(symbols, symbol)
			}
		}
	}
	return symbols
}

func sortKey(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: key, Code: rune(key[0])})
}

func TestTableSortsByEveryColumn(t *testing.T) {
	table := sortTestTable()
	if got := strings.Join(order(t, table), ","); got != "b.PL,A.PL,C.PL" {
		t.Fatalf("initial order = %s, want the given order", got)
	}

	// Text sorts alphabetically, ignoring case.
	table.Update(sortKey("s"))
	if got := strings.Join(order(t, table), ","); got != "A.PL,b.PL,C.PL" {
		t.Errorf("by symbol = %s", got)
	}
	if !strings.Contains(table.View(), "Symbol ▲") {
		t.Error("sorted column is not marked")
	}

	// Numbers sort by value, not by their formatted text, largest first.
	table.Update(sortKey("s"))
	if got := strings.Join(order(t, table), ","); got != "A.PL,b.PL,C.PL" {
		t.Errorf("by value = %s, want 1000, 900, 95", got)
	}
	table.Update(sortKey("S"))
	if got := strings.Join(order(t, table), ","); got != "C.PL,b.PL,A.PL" {
		t.Errorf("by value reversed = %s", got)
	}
	if !strings.Contains(table.View(), "Value ▲") {
		t.Error("reversed column is not marked ascending")
	}

	// Unknown values sort last in both directions.
	table.Update(sortKey("s"))
	if got := strings.Join(order(t, table), ","); got != "A.PL,C.PL,b.PL" {
		t.Errorf("by day = %s, want the unknown b.PL last", got)
	}
	table.Update(sortKey("S"))
	if got := strings.Join(order(t, table), ","); got != "C.PL,A.PL,b.PL" {
		t.Errorf("by day reversed = %s, want the unknown b.PL still last", got)
	}

	// Past the last column the given order comes back.
	table.Update(sortKey("s"))
	if got := strings.Join(order(t, table), ","); got != "b.PL,A.PL,C.PL" {
		t.Errorf("after cycling = %s, want the given order", got)
	}
}

func TestTableSelectsTheSortedRow(t *testing.T) {
	var selected Row
	table := sortTestTable().OnSelect(func(row Row) tea.Cmd {
		selected = row
		return nil
	})
	table.Update(sortKey("s"))
	table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	if selected["symbol"] != "A.PL" {
		t.Errorf("enter selected %v, want A.PL, the top row after sorting", selected["symbol"])
	}
}

// Tables that are not sortable ignore s, so it stays free for other widgets.
func TestUnsortableTableIgnoresSortKeys(t *testing.T) {
	table := NewTable("holdings", testColumns, []Row{{"symbol": "b"}, {"symbol": "a"}})
	table.SetSize(60, 10)
	table.SetFocused(true)
	table.Update(sortKey("s"))
	table.SetRows([]Row{{"symbol": "b"}, {"symbol": "a"}})

	if row, ok := table.highlighted(); !ok || row["symbol"] != "b" {
		t.Errorf("top row = %v, want b: an unsortable table keeps the given order", row)
	}
}

func TestTableStartsSortedBy(t *testing.T) {
	table := sortTestTable().SortedBy("value", true)

	if got := strings.Join(order(t, table), ","); got != "A.PL,b.PL,C.PL" {
		t.Errorf("initial order = %s, want by value, largest first", got)
	}
	if !strings.Contains(table.View(), "Value ▼") {
		t.Error("initial sort column is not marked")
	}
	table.Update(sortKey("S"))
	if got := strings.Join(order(t, table), ","); got != "C.PL,b.PL,A.PL" {
		t.Errorf("after S = %s, want the initial sort reversed", got)
	}
}
