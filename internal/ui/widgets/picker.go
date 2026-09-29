package widgets

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	pickerBorder     = lipgloss.Color("62")
	pickerAccent     = lipgloss.Color("116")
	pickerKeyColor   = lipgloss.Color("150")
	pickerTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(pickerAccent)
	pickerMuted      = lipgloss.NewStyle().Foreground(mutedColor)
	pickerGroupStyle = lipgloss.NewStyle().Bold(true).Foreground(headerColor)
	pickerCursorBar  = lipgloss.NewStyle().Foreground(pickerKeyColor).Render("▌")
	pickerThumb      = lipgloss.NewStyle().Foreground(pickerAccent).Render("┃")
	pickerTrack      = lipgloss.NewStyle().Foreground(unfocusedBorder).Render("│")
)

// pickerChrome is every line of the box that is not a list row: border,
// padding, title, subtitle, the blank lines around the list, and the footer.
const pickerChrome = 9

// pickerKeyGap separates the key column from the label column.
const pickerKeyGap = 3

// PickerItem is one row of a Picker. Key is an optional left column, e.g. the
// keys of a binding; Group is an optional heading rows are listed under.
type PickerItem struct {
	Key     string
	Label   string
	Group   string
	Value   any
	Current bool
}

// PickerResult says what a key press did to the picker.
type PickerResult int

const (
	// PickerOpen means the picker stays open.
	PickerOpen PickerResult = iota
	// PickerClosed means the user dismissed it.
	PickerClosed
	// PickerChosen means the user picked the returned item.
	PickerChosen
)

// Picker is a searchable list shown in a centred box: help screens, choosers.
// It sizes itself to its content within the box SetSize allows.
type Picker struct {
	title      string
	subtitle   string
	items      []PickerItem
	selectable bool
	visible    []int
	cursor     int
	offset     int
	query      string
	searching  bool
	maxWidth   int
	maxHeight  int
}

// NewPicker builds a picker. When selectable, enter picks the highlighted item;
// otherwise the list is for reading and enter does nothing.
func NewPicker(title, subtitle string, items []PickerItem, selectable bool) *Picker {
	p := &Picker{title: title, subtitle: subtitle, items: items, selectable: selectable,
		maxWidth: 80, maxHeight: 24}
	p.refilter()
	for i, index := range p.visible {
		if items[index].Current {
			p.cursor = i
		}
	}
	return p
}

// SetSize bounds the box to the area it is drawn over.
func (p *Picker) SetSize(width, height int) {
	p.maxWidth, p.maxHeight = width, height
	p.keepCursorVisible()
}

// Update handles a key press.
func (p *Picker) Update(msg tea.KeyPressMsg) (PickerResult, *PickerItem) {
	key := msg.String()
	if p.searching {
		return p.updateSearch(msg), nil
	}

	switch key {
	case "esc", "q", "?":
		return PickerClosed, nil
	case "/":
		p.searching = true
	case "enter":
		if p.selectable && len(p.visible) > 0 {
			item := p.items[p.visible[p.cursor]]
			return PickerChosen, &item
		}
	case "up", "k":
		p.move(-1)
	case "down", "j":
		p.move(1)
	case "pgup", "ctrl+u":
		p.move(-p.listHeight())
	case "pgdown", "ctrl+d":
		p.move(p.listHeight())
	case "home", "g":
		p.move(-len(p.visible))
	case "end", "G":
		p.move(len(p.visible))
	}
	return PickerOpen, nil
}

// updateSearch edits the query while it is being typed.
func (p *Picker) updateSearch(msg tea.KeyPressMsg) PickerResult {
	switch msg.String() {
	case "esc":
		p.searching, p.query = false, ""
	case "enter":
		p.searching = false
	case "backspace":
		if r := []rune(p.query); len(r) > 0 {
			p.query = string(r[:len(r)-1])
		}
	case "up":
		p.move(-1)
		return PickerOpen
	case "down":
		p.move(1)
		return PickerOpen
	default:
		if msg.Text == "" {
			return PickerOpen
		}
		p.query += msg.Text
	}
	p.refilter()
	return PickerOpen
}

// View renders the box.
func (p *Picker) View() string {
	width := p.boxWidth()
	innerWidth := width - 6 // border and two columns of padding each side

	subtitle := pickerMuted.Render(p.subtitle)
	if p.searching || p.query != "" {
		cursor := ""
		if p.searching {
			cursor = "█"
		}
		subtitle = lipgloss.NewStyle().Foreground(pickerKeyColor).Render("/ " + p.query + cursor)
	}

	lines := []string{pickerTitleStyle.Render(p.title), subtitle, ""}
	lines = append(lines, p.listLines(innerWidth)...)
	lines = append(lines, "", pickerMuted.Render(p.footer()))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(pickerBorder).
		Padding(1, 2).
		Width(width).
		Render(strings.Join(lines, "\n"))
}

// listLines renders the visible window of rows with the scrollbar beside it.
func (p *Picker) listLines(width int) []string {
	rows := p.rows()
	height := p.listHeight()
	if len(rows) == 0 {
		return padLines([]string{pickerMuted.Render("  no matches")}, height)
	}

	keyWidth := 0
	for _, index := range p.visible {
		keyWidth = max(keyWidth, lipgloss.Width(p.items[index].Key))
	}
	rowWidth := width - 2 // scrollbar and its gap

	end := min(p.offset+height, len(rows))
	lines := make([]string, 0, height)
	for i := p.offset; i < end; i++ {
		lines = append(lines, p.renderRow(rows[i], keyWidth, rowWidth))
	}
	lines = padLines(lines, height)

	thumbStart, thumbEnd := scrollThumb(p.offset, height, len(rows))
	for i := range lines {
		bar := " "
		if len(rows) > height {
			bar = pickerTrack
			if i >= thumbStart && i < thumbEnd {
				bar = pickerThumb
			}
		}
		lines[i] = padRight(lines[i], rowWidth) + " " + bar
	}
	return lines
}

// pickerRow is one line of the list: a group heading or an item.
type pickerRow struct {
	heading string
	// visibleIndex is the item's position in p.visible, -1 for a heading.
	visibleIndex int
}

// rows interleaves group headings with the visible items.
func (p *Picker) rows() []pickerRow {
	var rows []pickerRow
	group := ""
	for i, index := range p.visible {
		item := p.items[index]
		if item.Group != "" && item.Group != group {
			if len(rows) > 0 {
				rows = append(rows, pickerRow{visibleIndex: -1})
			}
			rows = append(rows, pickerRow{heading: item.Group, visibleIndex: -1})
			group = item.Group
		}
		rows = append(rows, pickerRow{visibleIndex: i})
	}
	return rows
}

func (p *Picker) renderRow(row pickerRow, keyWidth, width int) string {
	if row.visibleIndex < 0 {
		return " " + pickerGroupStyle.Render(row.heading)
	}
	item := p.items[p.visible[row.visibleIndex]]
	highlighted := row.visibleIndex == p.cursor

	marker := " "
	if item.Current {
		marker = "●"
	}
	label := item.Label
	if keyWidth > 0 {
		key := padRight(item.Key, keyWidth+pickerKeyGap)
		if highlighted {
			key = lipgloss.NewStyle().Bold(true).Foreground(pickerKeyColor).Render(key)
		} else {
			key = lipgloss.NewStyle().Foreground(pickerKeyColor).Render(key)
		}
		label = key + p.styleLabel(label, highlighted)
	} else {
		label = p.styleLabel(label, highlighted)
	}

	bar := " "
	if highlighted {
		bar = pickerCursorBar
	}
	line := bar + marker + label
	return lipgloss.NewStyle().MaxWidth(width).Render(line)
}

func (p *Picker) styleLabel(label string, highlighted bool) string {
	if highlighted {
		return lipgloss.NewStyle().Bold(true).Foreground(pickerAccent).Render(label)
	}
	return label
}

func (p *Picker) footer() string {
	if p.searching {
		return "enter keep   esc clear"
	}
	if p.selectable {
		return "/ search   enter select   esc/q close"
	}
	return "/ search   esc/q close"
}

// refilter keeps the items matching the query, case-insensitively, in any of
// their text, and resets the cursor.
func (p *Picker) refilter() {
	query := strings.ToLower(p.query)
	p.visible = p.visible[:0]
	for i, item := range p.items {
		text := strings.ToLower(item.Key + " " + item.Label + " " + item.Group)
		if strings.Contains(text, query) {
			p.visible = append(p.visible, i)
		}
	}
	p.cursor, p.offset = 0, 0
}

func (p *Picker) move(step int) {
	if len(p.visible) == 0 {
		return
	}
	p.cursor = min(max(p.cursor+step, 0), len(p.visible)-1)
	p.keepCursorVisible()
}

// keepCursorVisible scrolls so the cursor row, and its heading when it is the
// first item of a group, stay in the window.
func (p *Picker) keepCursorVisible() {
	rows := p.rows()
	line := 0
	for i, row := range rows {
		if row.visibleIndex == p.cursor {
			line = i
			break
		}
	}
	top := line
	if line > 0 && rows[line-1].visibleIndex < 0 && rows[line-1].heading != "" {
		top = line - 1
	}
	height := p.listHeight()
	if top < p.offset {
		p.offset = top
	}
	if line >= p.offset+height {
		p.offset = line - height + 1
	}
	p.offset = max(min(p.offset, len(rows)-height), 0)
}

// boxWidth fits the longest line, rows and chrome alike, within the allowed
// width.
func (p *Picker) boxWidth() int {
	widest := max(lipgloss.Width(p.title), lipgloss.Width(p.subtitle), lipgloss.Width(p.footer()))
	keyWidth := 0
	for _, item := range p.items {
		keyWidth = max(keyWidth, lipgloss.Width(item.Key))
	}
	for _, item := range p.items {
		row := 2 + lipgloss.Width(item.Label)
		if keyWidth > 0 {
			row += keyWidth + pickerKeyGap
		}
		widest = max(widest, row+2, lipgloss.Width(item.Group)+1)
	}
	return max(min(widest+6, p.maxWidth-4), 20)
}

// listHeight is how many rows fit, never more than there are.
func (p *Picker) listHeight() int {
	return max(min(len(p.allRows()), p.maxHeight-2-pickerChrome), 1)
}

// allRows counts rows as if nothing were filtered, so the box keeps its height
// while the user types a search.
func (p *Picker) allRows() []pickerRow {
	saved := p.visible
	p.visible = make([]int, len(p.items))
	for i := range p.items {
		p.visible[i] = i
	}
	rows := p.rows()
	p.visible = saved
	return rows
}

// scrollThumb is the span of the scrollbar thumb for a window over total rows.
func scrollThumb(offset, height, total int) (int, int) {
	if total <= height {
		return 0, height
	}
	size := max(height*height/total, 1)
	start := offset * (height - size) / max(total-height, 1)
	return start, start + size
}

func padLines(lines []string, height int) []string {
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func padRight(s string, width int) string {
	return s + strings.Repeat(" ", max(width-lipgloss.Width(s), 0))
}
