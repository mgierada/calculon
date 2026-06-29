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
	grid := Grid{Rows: []Row{
		{Weight: 2, Cells: []Cell{{Component: table, Weight: 3}, {Component: chart, Weight: 1}}},
		{Weight: 1, Cells: []Cell{{Component: summary}}},
	}}

	grid.Resize(120, 60)

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
	grid := Grid{Rows: []Row{{Cells: []Cell{{Component: nil}, {Component: table}}}}}

	grid.Resize(80, 24)

	// The nil cell still claims its share of the width; only its component is skipped.
	if got, want := table.box(), "40x24"; got != want {
		t.Errorf("table box = %s, want %s", got, want)
	}
}

func TestSingleFillsTheScreen(t *testing.T) {
	table := &stub{name: "table"}
	grid := Single(table)

	grid.Resize(80, 24)

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

	grid.Resize(80, 24)

	if got, want := top.box(), "80x12"; got != want {
		t.Errorf("top box = %s, want %s", got, want)
	}
	if got, want := bottom.box(), "80x12"; got != want {
		t.Errorf("bottom box = %s, want %s", got, want)
	}
}

func TestComponentsAreInPlacementOrder(t *testing.T) {
	a, b, c := &stub{name: "a"}, &stub{name: "b"}, &stub{name: "c"}
	grid := Grid{Rows: []Row{
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
