package widgets

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// segmentPalette colours stacked bar segments in order, cycling when there
// are more segments than colours.
var segmentPalette = []color.Color{
	lipgloss.Color("39"), lipgloss.Color("214"), lipgloss.Color("42"), lipgloss.Color("170"),
	lipgloss.Color("203"), lipgloss.Color("81"), lipgloss.Color("221"), lipgloss.Color("141"),
	lipgloss.Color("245"),
}

// StackBar draws shares of a whole as one full-width bar of coloured segments,
// with a legend underneath.
type StackBar struct {
	title    string
	segments []Bar
	width    int
	height   int
	focused  bool
}

// NewStackBar builds a stacked bar. Segment values are shares in any unit;
// only their proportions matter. Non-positive segments are left out.
func NewStackBar(title string, segments []Bar) *StackBar {
	kept := make([]Bar, 0, len(segments))
	for _, segment := range segments {
		if segment.Value > 0 {
			kept = append(kept, segment)
		}
	}
	return &StackBar{title: title, segments: kept}
}

// Init implements ui.Component.
func (s *StackBar) Init() tea.Cmd {
	return nil
}

// Update implements ui.Component. The bar is read-only, so it ignores input.
func (s *StackBar) Update(tea.Msg) tea.Cmd {
	return nil
}

// SetSize implements ui.Component.
func (s *StackBar) SetSize(width, height int) {
	s.width, s.height = width, height
}

// SetFocused implements ui.Component.
func (s *StackBar) SetFocused(focused bool) {
	s.focused = focused
}

// View implements ui.Component.
func (s *StackBar) View() string {
	innerWidth, _ := innerSize(s.width, s.height)
	content := s.bar(innerWidth) + "\n" + s.legend(innerWidth)
	return frame(s.width, s.height, s.focused).Render(content)
}

// bar splits the width across segments by largest remainder, so segment
// widths always add up to the full width.
func (s *StackBar) bar(width int) string {
	var total float64
	for _, segment := range s.segments {
		total += segment.Value
	}
	if total <= 0 || width <= 0 {
		return ""
	}

	widths := make([]int, len(s.segments))
	remainders := make([]float64, len(s.segments))
	assigned := 0
	for i, segment := range s.segments {
		exact := segment.Value / total * float64(width)
		widths[i] = int(math.Floor(exact))
		remainders[i] = exact - float64(widths[i])
		assigned += widths[i]
	}
	for ; assigned < width; assigned++ {
		best := 0
		for i := range remainders {
			if remainders[i] > remainders[best] {
				best = i
			}
		}
		widths[best]++
		remainders[best] = -1
	}

	var b strings.Builder
	for i, w := range widths {
		style := lipgloss.NewStyle().Foreground(s.color(i))
		b.WriteString(style.Render(strings.Repeat("█", w)))
	}
	return b.String()
}

// legend lists as many "■ label 12.3%" entries as fit on one line.
func (s *StackBar) legend(width int) string {
	var total float64
	for _, segment := range s.segments {
		total += segment.Value
	}
	entries := []string{titleStyle.Render(s.title)}
	used := lipgloss.Width(entries[0])
	for i, segment := range s.segments {
		marker := lipgloss.NewStyle().Foreground(s.color(i)).Render("■")
		entry := fmt.Sprintf("%s %s %.1f%%", marker, segment.Label, segment.Value/total*100)
		if used+2+lipgloss.Width(entry) > width {
			break
		}
		entries = append(entries, entry)
		used += 2 + lipgloss.Width(entry)
	}
	return strings.Join(entries, "  ")
}

// color is segment i's own colour, or its place in the palette.
func (s *StackBar) color(i int) color.Color {
	if c := s.segments[i].Color; c != nil {
		return c
	}
	return segmentPalette[i%len(segmentPalette)]
}
