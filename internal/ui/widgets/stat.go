package widgets

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var statValueStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))

// Stat is a KPI card: a label, a prominent value, and an optional toned note
// underneath such as a change or a percentage.
type Stat struct {
	label   string
	value   string
	note    string
	tone    Tone
	width   int
	height  int
	focused bool
}

// NewStat builds a KPI card.
func NewStat(label, value string) *Stat {
	return &Stat{label: label, value: value}
}

// WithNote adds a line under the value in the colour of its tone.
func (s *Stat) WithNote(note string, tone Tone) *Stat {
	s.note, s.tone = note, tone
	return s
}

// Init implements ui.Component.
func (s *Stat) Init() tea.Cmd {
	return nil
}

// Update implements ui.Component. A card is read-only, so it ignores input.
func (s *Stat) Update(tea.Msg) tea.Cmd {
	return nil
}

// SetSize implements ui.Component.
func (s *Stat) SetSize(width, height int) {
	s.width, s.height = width, height
}

// SetFocused implements ui.Component.
func (s *Stat) SetFocused(focused bool) {
	s.focused = focused
}

// View implements ui.Component. Text wraps to the card's width rather than
// being cut off; a grid row with AutoHeight makes room for the extra lines.
func (s *Stat) View() string {
	innerWidth, _ := innerSize(s.width, s.height)
	return frame(s.width, s.height, s.focused).Render(s.content(innerWidth))
}

// HeightFor implements ui.HeightFitter: the card's height with its text
// wrapped to width, borders included.
func (s *Stat) HeightFor(width int) int {
	innerWidth, _ := innerSize(width, 0)
	return lipgloss.Height(s.content(innerWidth)) + 2
}

// content is the label, value and note, each wrapped to width.
func (s *Stat) content(width int) string {
	wrap := lipgloss.NewStyle().Width(max(width, 1))
	parts := []string{
		wrap.Inherit(titleStyle).Render(s.label),
		wrap.Inherit(statValueStyle).Render(s.value),
	}
	if s.note != "" {
		parts = append(parts, wrap.Foreground(toneColor(s.tone)).Render(s.note))
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
