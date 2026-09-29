// Package widgets holds the terminal widgets a ui.Grid can place. Each widget
// satisfies ui.Component structurally, so this package stays free of any
// layout knowledge.
package widgets

import (
	"fmt"
	"image/color"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	btable "github.com/evertras/bubble-table/table"
)

var (
	focusedBorder   = lipgloss.Color("62")
	unfocusedBorder = lipgloss.Color("240")
	headerColor     = lipgloss.Color("212")
	mutedColor      = lipgloss.Color("245")
	highlightColor  = lipgloss.Color("238")
	positiveColor   = lipgloss.Color("42")
	negativeColor   = lipgloss.Color("203")
)

// selectKey opens the highlighted row.
const selectKey = "enter"

// Column describes a table column. A positive Flex makes the column share the
// leftover width in that proportion; otherwise Width is fixed. Cells align
// right, which suits numbers, unless Left is set.
type Column struct {
	Key   string
	Title string
	Width int
	Flex  int
	Left  bool
}

// Row is one table row, keyed by column key. It may carry extra keys no column
// shows, e.g. an id an OnSelect handler needs.
type Row map[string]any

// Tone says how a styled value should read.
type Tone int

const (
	Neutral Tone = iota
	Positive
	Negative
	Muted
)

// Toned renders a cell value in the colour of its tone.
func Toned(value any, tone Tone) any {
	return btable.NewStyledCell(value, lipgloss.NewStyle().Foreground(toneColor(tone)))
}

// ToneOf is Positive above zero, Negative below it, Neutral at zero.
func ToneOf(value float64) Tone {
	switch {
	case value > 0:
		return Positive
	case value < 0:
		return Negative
	default:
		return Neutral
	}
}

// Table is a scrollable, filterable data table.
type Table struct {
	table    btable.Model
	title    string
	columns  []Column
	rows     []Row
	onSelect func(Row) tea.Cmd
	width    int
	height   int
	focused  bool
}

// NewTable builds a table from neutral column and row descriptions.
func NewTable(title string, columns []Column, rows []Row) *Table {
	model := btable.New(toColumns(columns, false)).
		WithRows(toRows(rows)).
		WithBaseStyle(lipgloss.NewStyle().Align(lipgloss.Right)).
		HeaderStyle(lipgloss.NewStyle().Foreground(headerColor).Bold(true)).
		// bubble-table draws no cursor by default, so the row enter would open
		// is invisible. Coloured cells keep their colour over the background.
		HighlightStyle(lipgloss.NewStyle().Background(highlightColor).Bold(true)).
		Focused(false)

	widget := &Table{table: model, title: title, columns: columns, rows: rows}
	widget.applyStyle()
	return widget
}

// OnSelect makes enter on a row call handle with that row.
func (t *Table) OnSelect(handle func(Row) tea.Cmd) *Table {
	t.onSelect = handle
	return t
}

// Filterable lets the user type / to filter rows by any column.
func (t *Table) Filterable() *Table {
	t.table = t.table.WithColumns(toColumns(t.columns, true)).Filtered(true)
	return t
}

// SetRows replaces the table contents.
func (t *Table) SetRows(rows []Row) {
	t.rows = rows
	t.table = t.table.WithRows(toRows(rows))
}

// Init implements ui.Component.
func (t *Table) Init() tea.Cmd {
	return t.table.Init()
}

// Update implements ui.Component.
func (t *Table) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.String() == selectKey &&
		t.onSelect != nil && !t.CapturingInput() {
		if row, ok := t.highlighted(); ok {
			return t.onSelect(row)
		}
		return nil
	}

	var cmd tea.Cmd
	t.table, cmd = t.table.Update(msg)
	return cmd
}

// CapturingInput implements ui.InputCapturer while the filter is being typed.
func (t *Table) CapturingInput() bool {
	return t.table.GetIsFilterInputFocused()
}

// View implements ui.Component. The footer is set here rather than once at
// construction because the page counter changes as the user scrolls. While a
// filter is in use bubble-table's own footer shows it instead.
func (t *Table) View() string {
	if t.CapturingInput() || t.table.GetCurrentFilter() != "" {
		return t.table.WithStaticFooter("").View()
	}
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

// highlighted returns the row under the cursor, with every key it was built with.
func (t *Table) highlighted() (Row, bool) {
	if len(t.table.GetVisibleRows()) == 0 {
		return nil, false
	}
	return Row(t.table.HighlightedRow().Data), true
}

// applyStyle highlights the border of the focused table.
func (t *Table) applyStyle() {
	border := unfocusedBorder
	if t.focused {
		border = focusedBorder
	}
	t.table = t.table.WithBorderForeground(border)
}

func toColumns(columns []Column, filtered bool) []btable.Column {
	converted := make([]btable.Column, 0, len(columns))
	for _, column := range columns {
		var c btable.Column
		if column.Flex > 0 {
			c = btable.NewFlexColumn(column.Key, column.Title, column.Flex)
		} else {
			c = btable.NewColumn(column.Key, column.Title, column.Width)
		}
		if column.Left {
			c = c.WithStyle(lipgloss.NewStyle().Align(lipgloss.Left))
		}
		converted = append(converted, c.WithFiltered(filtered))
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

func toneColor(tone Tone) color.Color {
	switch tone {
	case Positive:
		return positiveColor
	case Negative:
		return negativeColor
	case Muted:
		return mutedColor
	default:
		return lipgloss.Color("252")
	}
}
