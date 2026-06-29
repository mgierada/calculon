package ui

import (
	"charm.land/lipgloss/v2"
)

// Cell places one component inside a row. Weight is the share of the row's
// width the component gets; a zero weight counts as one.
type Cell struct {
	Component Component
	Weight    int
}

// Row is a horizontal band of cells. Weight is the share of the grid's height
// the row gets; a zero weight counts as one.
type Row struct {
	Cells  []Cell
	Weight int
}

// Grid is a vertical stack of rows. Rows split the available height by weight,
// and the cells in a row split that row's width by weight.
//
//	Grid{Rows: []Row{
//	    {Weight: 2, Cells: []Cell{{Component: table, Weight: 3}, {Component: chart, Weight: 1}}},
//	    {Weight: 1, Cells: []Cell{{Component: summary}}},
//	}}
//
// renders the table and chart side by side across the top two thirds, the table
// three times as wide as the chart, with the summary across the bottom third.
type Grid struct {
	Rows []Row
}

// Single returns a grid holding one component that fills the screen.
func Single(component Component) Grid {
	return Grid{Rows: []Row{{Cells: []Cell{{Component: component}}}}}
}

// Stack returns a grid of equally tall, full-width rows, one per component.
func Stack(components ...Component) Grid {
	rows := make([]Row, 0, len(components))
	for _, component := range components {
		rows = append(rows, Row{Cells: []Cell{{Component: component}}})
	}
	return Grid{Rows: rows}
}

// Components returns every component in placement order, left to right within a
// row and top to bottom across rows.
func (g Grid) Components() []Component {
	var components []Component
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			if cell.Component != nil {
				components = append(components, cell.Component)
			}
		}
	}
	return components
}

// Resize hands every component the box it should render inside.
func (g Grid) Resize(width, height int) {
	heights := distribute(height, weights(len(g.Rows), func(i int) int { return g.Rows[i].Weight }))

	for i, row := range g.Rows {
		widths := distribute(width, weights(len(row.Cells), func(j int) int { return row.Cells[j].Weight }))
		for j, cell := range row.Cells {
			if cell.Component == nil {
				continue
			}
			cell.Component.SetSize(widths[j], heights[i])
		}
	}
}

// View renders the grid as a single string.
func (g Grid) View() string {
	rowViews := make([]string, 0, len(g.Rows))
	for _, row := range g.Rows {
		cellViews := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			if cell.Component == nil {
				continue
			}
			cellViews = append(cellViews, cell.Component.View())
		}
		rowViews = append(rowViews, lipgloss.JoinHorizontal(lipgloss.Top, cellViews...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rowViews...)
}

// weights collects n weights from an accessor.
func weights(n int, weightAt func(int) int) []int {
	collected := make([]int, n)
	for i := range collected {
		collected[i] = weightAt(i)
	}
	return collected
}

// distribute splits total proportionally to weights, handing the rounding
// remainder to the earliest entries so the parts always add up to total.
func distribute(total int, weights []int) []int {
	sizes := make([]int, len(weights))
	if len(weights) == 0 || total <= 0 {
		return sizes
	}

	sum := 0
	normalized := make([]int, len(weights))
	for i, weight := range weights {
		if weight <= 0 {
			weight = 1
		}
		normalized[i] = weight
		sum += weight
	}

	assigned := 0
	for i, weight := range normalized {
		sizes[i] = total * weight / sum
		assigned += sizes[i]
	}
	for i := 0; assigned < total; i, assigned = i+1, assigned+1 {
		sizes[i%len(sizes)]++
	}

	return sizes
}
