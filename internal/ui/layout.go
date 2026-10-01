package ui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Cell places one component inside a row. Weight is the share of the row's
// width the component gets; a zero weight counts as one.
type Cell struct {
	Component Component
	Weight    int
}

// Row is a horizontal band of cells. A positive Height fixes the row to that
// many lines; the other rows split what is left by Weight, where a zero weight
// counts as one.
type Row struct {
	Cells  []Cell
	Weight int
	Height int
	// AutoHeight makes Height a minimum: the row grows to fit the tallest
	// cell that implements HeightFitter at the width it is given.
	AutoHeight bool
}

// Grid is a vertical stack of rows. Rows split the available height, and the
// cells in a row split that row's width by weight.
//
//	Grid{Rows: []Row{
//	    {Height: 5, Cells: Cells(kpiA, kpiB, kpiC)},
//	    {Weight: 2, Cells: []Cell{{Component: table, Weight: 3}, {Component: chart, Weight: 1}}},
//	    {Weight: 1, Cells: []Cell{{Component: summary}}},
//	}}
//
// renders a five line band of equally wide KPI cards, then the table and chart
// side by side across two thirds of the rest, the table three times as wide as
// the chart, with the summary across the bottom third.
//
// A Grid is itself a Component, so a cell can hold another grid and layouts
// compose. Focus and key routing always target the leaves; see Components.
type Grid struct {
	Rows []Row
	// boxes holds the size each cell was last given, by row then cell.
	boxes [][]box
}

// box is a width by height area.
type box struct{ width, height int }

// Single returns a grid holding one component that fills the screen.
func Single(component Component) *Grid {
	return &Grid{Rows: []Row{{Cells: []Cell{{Component: component}}}}}
}

// Stack returns a grid of equally tall, full-width rows, one per component.
func Stack(components ...Component) *Grid {
	rows := make([]Row, 0, len(components))
	for _, component := range components {
		rows = append(rows, Row{Cells: []Cell{{Component: component}}})
	}
	return &Grid{Rows: rows}
}

// Cells gives each component an equal share of a row.
func Cells(components ...Component) []Cell {
	cells := make([]Cell, 0, len(components))
	for _, component := range components {
		cells = append(cells, Cell{Component: component})
	}
	return cells
}

// Components returns every leaf component in placement order, left to right
// within a row and top to bottom across rows, descending into nested grids.
func (g *Grid) Components() []Component {
	var components []Component
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			switch nested := cell.Component.(type) {
			case nil:
			case *Grid:
				components = append(components, nested.Components()...)
			default:
				components = append(components, nested)
			}
		}
	}
	return components
}

// Init implements Component by starting every cell.
func (g *Grid) Init() tea.Cmd {
	var cmds []tea.Cmd
	g.each(func(c Component) { cmds = append(cmds, c.Init()) })
	return tea.Batch(cmds...)
}

// Update implements Component by forwarding the message to every cell. The app
// routes key presses to the focused leaf directly, so this sees the rest.
func (g *Grid) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd
	g.each(func(c Component) { cmds = append(cmds, c.Update(msg)) })
	return tea.Batch(cmds...)
}

// SetFocused implements Component. A grid is never focused itself; its leaves are.
func (g *Grid) SetFocused(bool) {}

// SetSize implements Component by handing every cell the box it renders inside.
func (g *Grid) SetSize(width, height int) {
	widths := make([][]int, len(g.Rows))
	for i, row := range g.Rows {
		widths[i] = distribute(width, weights(len(row.Cells), func(j int) int { return row.Cells[j].Weight }))
	}
	heights := rowHeights(g.Rows, fixedHeights(g.Rows, widths), height)
	g.boxes = make([][]box, len(g.Rows))
	for i, row := range g.Rows {
		widths := widths[i]
		g.boxes[i] = make([]box, len(row.Cells))
		for j, cell := range row.Cells {
			g.boxes[i][j] = box{widths[j], heights[i]}
			if cell.Component == nil {
				continue
			}
			cell.Component.SetSize(widths[j], heights[i])
		}
	}
}

// View implements Component by rendering the grid as a single string. Each
// cell is clipped to its box, so a widget that cannot shrink far enough, like a
// table with fixed columns on a tiny terminal, never pushes its neighbours out.
func (g *Grid) View() string {
	rowViews := make([]string, 0, len(g.Rows))
	for i, row := range g.Rows {
		cellViews := make([]string, 0, len(row.Cells))
		for j, cell := range row.Cells {
			if cell.Component == nil {
				continue
			}
			cellViews = append(cellViews, g.clip(i, j, cell.Component.View()))
		}
		rowViews = append(rowViews, lipgloss.JoinHorizontal(lipgloss.Top, cellViews...))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rowViews...)
}

// clip cuts a cell's view down to the box it was given, once it has one.
func (g *Grid) clip(row, cell int, view string) string {
	if row >= len(g.boxes) || cell >= len(g.boxes[row]) {
		return view
	}
	b := g.boxes[row][cell]
	return lipgloss.NewStyle().MaxWidth(b.width).MaxHeight(b.height).Render(view)
}

// each calls f on every direct cell component.
func (g *Grid) each(f func(Component)) {
	for _, row := range g.Rows {
		for _, cell := range row.Cells {
			if cell.Component != nil {
				f(cell.Component)
			}
		}
	}
}

// fixedHeights is the height each fixed row wants, zero for weighted rows. An
// AutoHeight row wants at least its Height and as much as its tallest cell
// needs at its width.
func fixedHeights(rows []Row, widths [][]int) []int {
	fixed := make([]int, len(rows))
	for i, row := range rows {
		fixed[i] = max(row.Height, 0)
		if !row.AutoHeight {
			continue
		}
		for j, cell := range row.Cells {
			if fitter, ok := cell.Component.(HeightFitter); ok {
				fixed[i] = max(fixed[i], fitter.HeightFor(widths[i][j]))
			}
		}
	}
	return fixed
}

// rowHeights gives fixed rows their height, shrinking them when they do not
// fit, and splits the remainder among the weighted rows.
func rowHeights(rows []Row, fixed []int, total int) []int {
	heights := make([]int, len(rows))
	remaining := max(total, 0)
	var flexible []int
	for i := range rows {
		if fixed[i] <= 0 {
			flexible = append(flexible, i)
			continue
		}
		heights[i] = min(fixed[i], remaining)
		remaining -= heights[i]
	}
	shares := distribute(remaining, weights(len(flexible), func(k int) int { return rows[flexible[k]].Weight }))
	for k, i := range flexible {
		heights[i] = shares[k]
	}
	return heights
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
