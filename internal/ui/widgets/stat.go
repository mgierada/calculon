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

// View implements ui.Component.
func (s *Stat) View() string {
	lines := titleStyle.Render(s.label) + "\n" + statValueStyle.Render(s.value)
	if s.note != "" {
		lines += "\n" + lipgloss.NewStyle().Foreground(toneColor(s.tone)).Render(s.note)
	}
	innerWidth, _ := innerSize(s.width, s.height)
	return frame(s.width, s.height, s.focused).Render(
		lipgloss.NewStyle().MaxWidth(innerWidth).Render(lines))
}
