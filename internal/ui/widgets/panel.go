package widgets

import (
	"charm.land/lipgloss/v2"
)

// frame is the rounded border every panel widget draws, sized so the box,
// border included, fills exactly width by height. The focused panel's border
// is highlighted.
func frame(width, height int, focused bool) lipgloss.Style {
	border := unfocusedBorder
	if focused {
		border = focusedBorder
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Width(max(width, 0)).
		Height(max(height, 0))
}

// innerSize is the content area inside a frame.
func innerSize(width, height int) (int, int) {
	return max(width-2, 0), max(height-2, 0)
}

// titleStyle renders a panel's title line.
var titleStyle = lipgloss.NewStyle().Foreground(headerColor).Bold(true)
