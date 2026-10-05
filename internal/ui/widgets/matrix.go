package widgets

import (
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Keys that scroll a matrix's periods.
const (
	matrixOlderKey = "left"
	matrixNewerKey = "right"
)

// sparkBlocks are the bar heights of a sparkline, lowest first.
var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkWidth caps how many periods a row's sparkline shows.
const sparkWidth = 16

var (
	matrixHeaderStyle = lipgloss.NewStyle().Foreground(headerColor).Bold(true).Padding(0, 1)
	matrixCellStyle   = lipgloss.NewStyle().Padding(0, 1).Align(lipgloss.Right)
	matrixLabelStyle  = lipgloss.NewStyle().Padding(0, 1)
	matrixLatestStyle = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	sparkStyle        = lipgloss.NewStyle().Foreground(positiveColor)
	sparkNegStyle     = lipgloss.NewStyle().Foreground(negativeColor)
)

// MatrixRow is one figure across every period, oldest first; nil is unknown.
type MatrixRow struct {
	Label  string
	Values []*float64
	Format func(float64) string
	// Bold marks a headline figure; Toned colours values by sign.
	Bold, Toned bool
}

// Matrix shows figures by period, like a financial statement: a column per
// period, the latest highlighted, and each row's trend as a sparkline. When
// the periods do not all fit, the newest show and ← → scroll to older ones.
type Matrix struct {
	title   string
	note    string
	periods []string
	rows    []MatrixRow
	// offset is how many of the newest periods are scrolled out on the right.
	offset  int
	width   int
	height  int
	focused bool
}

// NewMatrix builds a matrix over periods, oldest first.
func NewMatrix(title, note string, periods []string, rows ...MatrixRow) *Matrix {
	return &Matrix{title: title, note: note, periods: periods, rows: rows}
}

// Init implements ui.Component.
func (m *Matrix) Init() tea.Cmd { return nil }

// Update implements ui.Component, scrolling periods on ← and →.
func (m *Matrix) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		m.Scroll(key.String())
	}
	return nil
}

// Scroll moves the visible periods for key, reporting whether it was a
// scroll key.
func (m *Matrix) Scroll(key string) bool {
	switch key {
	case matrixOlderKey:
		m.offset = min(m.offset+1, max(len(m.periods)-1, 0))
	case matrixNewerKey:
		m.offset = max(m.offset-1, 0)
	default:
		return false
	}
	return true
}

// matrixChrome is the frame, title, header and header rule around the rows.
const matrixChrome = 5

// Height is how many lines the matrix needs to show every row.
func (m *Matrix) Height() int { return len(m.rows) + matrixChrome }

// Offset is the scroll position, for keeping it across rebuilds.
func (m *Matrix) Offset() int { return m.offset }

// SetOffset restores a scroll position.
func (m *Matrix) SetOffset(offset int) { m.offset = max(min(offset, len(m.periods)-1), 0) }

// SetSize implements ui.Component.
func (m *Matrix) SetSize(width, height int) { m.width, m.height = width, height }

// SetFocused implements ui.Component.
func (m *Matrix) SetFocused(focused bool) { m.focused = focused }

// View implements ui.Component.
func (m *Matrix) View() string {
	innerWidth, _ := innerSize(m.width, m.height)
	header := titleStyle.Render(m.title)
	if m.note != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header,
			lipgloss.PlaceHorizontal(max(innerWidth-lipgloss.Width(header), 0), lipgloss.Right,
				mutedStyle.Render(m.note)))
	}
	return frame(m.width, m.height, m.focused).Render(header + "\n" + m.table(innerWidth))
}

// table lays out the periods that fit in width, newest last.
func (m *Matrix) table(width int) string {
	cells := m.formatted()
	labelWidth, cellWidth := 0, 0
	for i, row := range m.rows {
		labelWidth = max(labelWidth, lipgloss.Width(row.Label)+2)
		for _, cell := range cells[i] {
			cellWidth = max(cellWidth, lipgloss.Width(cell)+2)
		}
	}
	for _, p := range m.periods {
		cellWidth = max(cellWidth, lipgloss.Width(p)+2)
	}
	spark := min(len(m.periods), sparkWidth) + 2
	fit := max((width-labelWidth-spark)/max(cellWidth, 1), 1)
	end := len(m.periods) - m.offset
	start := max(end-fit, 0)

	headers := append([]string{""}, m.periods[start:end]...)
	headers = append(headers, "trend")
	t := table.New().Headers(headers...).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderColumn(false).BorderHeader(true).BorderStyle(sliderTrackStyle)
	for i, row := range m.rows {
		line := append([]string{row.Label}, cells[i][start:end]...)
		t.Row(append(line, sparkline(row.Values))...)
	}
	latest := end - start
	t.StyleFunc(func(r, c int) lipgloss.Style {
		var style lipgloss.Style
		switch {
		case r == table.HeaderRow:
			style = matrixHeaderStyle
		case c == 0:
			style = matrixLabelStyle
		default:
			style = matrixCellStyle
		}
		if r >= 0 && r < len(m.rows) && m.rows[r].Bold {
			style = style.Bold(true)
		}
		if c == latest && m.offset == 0 {
			style = style.Inherit(matrixLatestStyle)
		}
		return style
	})
	return t.String()
}

// formatted renders every value, toned by sign where the row asks.
func (m *Matrix) formatted() [][]string {
	cells := make([][]string, len(m.rows))
	for i, row := range m.rows {
		cells[i] = make([]string, len(m.periods))
		for j := range m.periods {
			cells[i][j] = "—"
			if j >= len(row.Values) || row.Values[j] == nil {
				continue
			}
			v := *row.Values[j]
			cells[i][j] = row.Format(v)
			if row.Toned {
				cells[i][j] = lipgloss.NewStyle().Foreground(toneColor(ToneOf(v))).Render(cells[i][j])
			}
		}
	}
	return cells
}

// sparkline draws the last sparkWidth values as bars of their size against
// the largest, negatives in red and unknown values as gaps.
func sparkline(values []*float64) string {
	values = values[max(len(values)-sparkWidth, 0):]
	largest := 0.0
	for _, v := range values {
		if v != nil {
			largest = math.Max(largest, math.Abs(*v))
		}
	}
	var b strings.Builder
	for _, v := range values {
		if v == nil || largest == 0 {
			b.WriteString(" ")
			continue
		}
		level := int(math.Round(math.Abs(*v) / largest * float64(len(sparkBlocks)-1)))
		bar := string(sparkBlocks[max(min(level, len(sparkBlocks)-1), 0)])
		if *v < 0 {
			b.WriteString(sparkNegStyle.Render(bar))
		} else {
			b.WriteString(sparkStyle.Render(bar))
		}
	}
	return b.String()
}
