package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// stub is a Component that only records the box it was given.
type stub struct {
	name    string
	width   int
	height  int
	focused bool
}

func (s *stub) Init() tea.Cmd             { return nil }
func (s *stub) Update(tea.Msg) tea.Cmd    { return nil }
func (s *stub) View() string              { return s.name }
func (s *stub) SetSize(width, height int) { s.width, s.height = width, height }
func (s *stub) SetFocused(focused bool)   { s.focused = focused }
func (s *stub) box() string               { return fmt.Sprintf("%dx%d", s.width, s.height) }

func TestDistribute(t *testing.T) {
	tests := []struct {
		total   int
		weights []int
		want    []int
	}{
		{100, []int{1}, []int{100}},
		{100, []int{1, 1}, []int{50, 50}},
		{100, []int{3, 1}, []int{75, 25}},
		{10, []int{1, 1, 1}, []int{4, 3, 3}},
		// A zero weight counts as one.
		{100, []int{0, 0}, []int{50, 50}},
		{0, []int{1, 1}, []int{0, 0}},
		{-5, []int{1}, []int{0}},
		{100, nil, []int{}},
	}

	for _, test := range tests {
		got := distribute(test.total, test.weights)
		if len(got) != len(test.want) {
			t.Errorf("distribute(%d, %v) = %v, want %v", test.total, test.weights, got, test.want)
			continue
		}
		sum := 0
		for i, size := range got {
			if size != test.want[i] {
				t.Errorf("distribute(%d, %v) = %v, want %v", test.total, test.weights, got, test.want)
				break
			}
			sum += size
		}
		if test.total > 0 && len(test.weights) > 0 && sum != test.total {
			t.Errorf("distribute(%d, %v) sums to %d, want %d", test.total, test.weights, sum, test.total)
		}
	}
}

func TestGridResizeSplitsByWeight(t *testing.T) {
	table, chart, summary := &stub{name: "table"}, &stub{name: "chart"}, &stub{name: "summary"}
	grid := &Grid{Rows: []Row{
		{Weight: 2, Cells: []Cell{{Component: table, Weight: 3}, {Component: chart, Weight: 1}}},
		{Weight: 1, Cells: []Cell{{Component: summary}}},
	}}

	grid.SetSize(120, 60)

	if got, want := table.box(), "90x40"; got != want {
		t.Errorf("table box = %s, want %s", got, want)
	}
	if got, want := chart.box(), "30x40"; got != want {
		t.Errorf("chart box = %s, want %s", got, want)
	}
	if got, want := summary.box(), "120x20"; got != want {
		t.Errorf("summary box = %s, want %s", got, want)
	}
}

func TestGridResizeSkipsNilCells(t *testing.T) {
	table := &stub{name: "table"}
	grid := &Grid{Rows: []Row{{Cells: []Cell{{Component: nil}, {Component: table}}}}}

	grid.SetSize(80, 24)

	// The nil cell still claims its share of the width; only its component is skipped.
	if got, want := table.box(), "40x24"; got != want {
		t.Errorf("table box = %s, want %s", got, want)
	}
}

func TestSingleFillsTheScreen(t *testing.T) {
	table := &stub{name: "table"}
	grid := Single(table)

	grid.SetSize(80, 24)

	if got, want := table.box(), "80x24"; got != want {
		t.Errorf("table box = %s, want %s", got, want)
	}
	if components := grid.Components(); len(components) != 1 {
		t.Errorf("Components() returned %d components, want 1", len(components))
	}
}

func TestStackSplitsHeightEvenly(t *testing.T) {
	top, bottom := &stub{name: "top"}, &stub{name: "bottom"}
	grid := Stack(top, bottom)

	grid.SetSize(80, 24)

	if got, want := top.box(), "80x12"; got != want {
		t.Errorf("top box = %s, want %s", got, want)
	}
	if got, want := bottom.box(), "80x12"; got != want {
		t.Errorf("bottom box = %s, want %s", got, want)
	}
}

func TestComponentsAreInPlacementOrder(t *testing.T) {
	a, b, c := &stub{name: "a"}, &stub{name: "b"}, &stub{name: "c"}
	grid := &Grid{Rows: []Row{
		{Cells: []Cell{{Component: a}, {Component: b}}},
		{Cells: []Cell{{Component: c}}},
	}}

	got := grid.Components()
	want := []Component{a, b, c}
	if len(got) != len(want) {
		t.Fatalf("Components() returned %d components, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Components()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestGridFixedRowsKeepTheirHeight(t *testing.T) {
	stats, table := &stub{name: "stats"}, &stub{name: "table"}
	grid := &Grid{Rows: []Row{
		{Height: 5, Cells: Cells(stats)},
		{Cells: Cells(table)},
	}}
	grid.SetSize(80, 30)

	if got, want := stats.box(), "80x5"; got != want {
		t.Errorf("fixed row box = %s, want %s", got, want)
	}
	if got, want := table.box(), "80x25"; got != want {
		t.Errorf("flexible row box = %s, want %s", got, want)
	}
}

// Fixed rows that do not fit shrink rather than overflow the screen.
func TestGridFixedRowsShrinkToFit(t *testing.T) {
	top, bottom := &stub{name: "top"}, &stub{name: "bottom"}
	grid := &Grid{Rows: []Row{{Height: 8, Cells: Cells(top)}, {Height: 8, Cells: Cells(bottom)}}}
	grid.SetSize(80, 10)

	if got, want := bottom.box(), "80x2"; got != want {
		t.Errorf("second fixed row box = %s, want %s", got, want)
	}
}

func TestGridNestsGrids(t *testing.T) {
	left, right, below := &stub{name: "left"}, &stub{name: "right"}, &stub{name: "below"}
	inner := Stack(right, below)
	grid := &Grid{Rows: []Row{{Cells: Cells(left, inner)}}}
	grid.SetSize(100, 40)

	if got, want := right.box(), "50x20"; got != want {
		t.Errorf("nested cell box = %s, want %s", got, want)
	}
	if got := grid.Components(); len(got) != 3 || got[2] != below {
		t.Errorf("Components() = %v, want the nested leaves flattened", got)
	}
}

// fitter is a stub that asks for more height the narrower it gets.
type fitter struct{ stub }

func (f *fitter) HeightFor(width int) int { return 100 / width }

func TestGridAutoHeightRowGrowsToFit(t *testing.T) {
	card, body := &fitter{stub{name: "card"}}, &stub{name: "body"}
	grid := &Grid{Rows: []Row{
		{Height: 5, AutoHeight: true, Cells: Cells(card)},
		{Cells: Cells(body)},
	}}

	grid.SetSize(10, 40)
	if got, want := card.box(), "10x10"; got != want {
		t.Errorf("narrow auto row = %s, want %s", got, want)
	}
	if got, want := body.box(), "10x30"; got != want {
		t.Errorf("body under it = %s, want %s", got, want)
	}

	// Wide enough, the row keeps its Height as a minimum.
	grid.SetSize(50, 40)
	if got, want := card.box(), "50x5"; got != want {
		t.Errorf("wide auto row = %s, want %s", got, want)
	}
}

func TestGridIgnoresFittersWithoutAutoHeight(t *testing.T) {
	card := &fitter{stub{name: "card"}}
	grid := &Grid{Rows: []Row{{Height: 5, Cells: Cells(card)}, {Cells: Cells(&stub{name: "x"})}}}
	grid.SetSize(10, 40)

	if got, want := card.box(), "10x5"; got != want {
		t.Errorf("fixed row = %s, want %s", got, want)
	}
}
