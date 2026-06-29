// Package widgets holds the terminal widgets a ui.Grid can place. Each widget
// satisfies ui.Component structurally, so this package stays free of any
// layout knowledge.
package widgets

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	btable "github.com/evertras/bubble-table/table"
)

var (
	focusedBorder   = lipgloss.Color("62")
	unfocusedBorder = lipgloss.Color("240")
	headerColor     = lipgloss.Color("212")
)

// Column describes a table column. A positive Flex makes the column share the
// leftover width in that proportion; otherwise Width is fixed.
type Column struct {
	Key   string
	Title string
	Width int
	Flex  int
}

// Row is one table row, keyed by column key.
type Row map[string]any

// Table is a scrollable, sortable data table.
type Table struct {
	table   btable.Model
	title   string
	width   int
	height  int
	focused bool
}

// NewTable builds a table from neutral column and row descriptions.
func NewTable(title string, columns []Column, rows []Row) *Table {
	model := btable.New(toColumns(columns)).
		WithRows(toRows(rows)).
		WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Right)).
		HeaderStyle(lipgloss.NewStyle().Foreground(headerColor).Bold(true)).
		Focused(false)

	widget := &Table{table: model, title: title}
	widget.applyStyle()
	return widget
}

// SetRows replaces the table contents.
func (t *Table) SetRows(rows []Row) {
	t.table = t.table.WithRows(toRows(rows))
}

// Init implements ui.Component.
func (t *Table) Init() tea.Cmd {
	return t.table.Init()
}

// Update implements ui.Component.
func (t *Table) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// View implements ui.Component. The footer is set here rather than once at
// construction because the page counter changes as the user scrolls.
func (t *Table) View() string {
	return t.table.WithStaticFooter(t.footer()).View()
}

// footer names the table and, when the rows do not all fit, says which page is
// showing. bubble-table renders either a static footer or its own page counter,
// never both, so the two are combined here.
func (t *Table) footer() string {
	if pages := t.table.MaxPages(); pages > 1 {
		return fmt.Sprintf("%s — page %d/%d", t.title, t.table.CurrentPage(), pages)
	}
	return t.title
}

// SetSize implements ui.Component. bubble-table sizes itself from the totals it
// is given, borders and header included, and paginates the rows that do not fit.
// The minimum matches the target so the table fills its cell instead of
// shrinking to its contents.
func (t *Table) SetSize(width, height int) {
	t.width, t.height = width, height
	t.table = t.table.
		WithTargetWidth(width).
		WithTargetHeight(height).
		WithMinimumHeight(height)
}

// SetFocused implements ui.Component.
func (t *Table) SetFocused(focused bool) {
	t.focused = focused
	t.table = t.table.Focused(focused)
	t.applyStyle()
}

// applyStyle highlights the border of the focused table.
func (t *Table) applyStyle() {
	border := unfocusedBorder
	if t.focused {
		border = focusedBorder
	}
	t.table = t.table.WithBorderForeground(border)
}

func toColumns(columns []Column) []btable.Column {
	converted := make([]btable.Column, 0, len(columns))
	for _, column := range columns {
		if column.Flex > 0 {
			converted = append(converted, btable.NewFlexColumn(column.Key, column.Title, column.Flex))
			continue
		}
		converted = append(converted, btable.NewColumn(column.Key, column.Title, column.Width))
	}
	return converted
}

func toRows(rows []Row) []btable.Row {
	converted := make([]btable.Row, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, btable.NewRow(btable.RowData(row)))
	}
	return converted
}
