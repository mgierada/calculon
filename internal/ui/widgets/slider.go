package widgets

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	sliderTrackStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sliderLowStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	sliderAvgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
	sliderHighStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	sliderTargetStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
)

// RangeMark is one value placed on a range slider. Nil values are left off.
type RangeMark struct {
	Label string
	Value *float64
	Glyph string
	Style lipgloss.Style
}

// LowMark, AverageMark, HighMark and TargetMark are the marks a price target
// slider shows, styled to tell them apart.
func LowMark(v *float64) RangeMark     { return RangeMark{"low", v, "├", sliderLowStyle} }
func AverageMark(v *float64) RangeMark { return RangeMark{"avg", v, "●", sliderAvgStyle} }
func HighMark(v *float64) RangeMark    { return RangeMark{"high", v, "┤", sliderHighStyle} }
func TargetMark(v *float64) RangeMark  { return RangeMark{"target", v, "◆", sliderTargetStyle} }

// RangeSlider places values on one track spanning the lowest to the highest
// of them, so a value outside the others' range still shows. Where two marks
// share a cell, the later one wins.
type RangeSlider struct {
	title   string
	marks   []RangeMark
	note    string
	format  func(float64) string
	width   int
	height  int
	focused bool
}

// NewRangeSlider builds a slider of marks.
func NewRangeSlider(title string, marks ...RangeMark) *RangeSlider {
	return &RangeSlider{title: title, marks: marks, format: func(v float64) string {
		return fmt.Sprintf("%.2f", v)
	}}
}

// WithFormat renders values with format.
func (s *RangeSlider) WithFormat(format func(float64) string) *RangeSlider {
	s.format = format
	return s
}

// WithNote adds a line under the labels.
func (s *RangeSlider) WithNote(note string) *RangeSlider {
	s.note = note
	return s
}

// Init implements ui.Component.
func (s *RangeSlider) Init() tea.Cmd { return nil }

// Update implements ui.Component. The slider is read-only.
func (s *RangeSlider) Update(tea.Msg) tea.Cmd { return nil }

// SetSize implements ui.Component.
func (s *RangeSlider) SetSize(width, height int) { s.width, s.height = width, height }

// SetFocused implements ui.Component.
func (s *RangeSlider) SetFocused(focused bool) { s.focused = focused }

// View implements ui.Component.
func (s *RangeSlider) View() string {
	innerWidth, _ := innerSize(s.width, s.height)
	lines := []string{titleStyle.Render(s.title), s.Track(innerWidth), s.labels()}
	if s.note != "" {
		lines = append(lines, mutedStyle.Render(s.note))
	}
	return frame(s.width, s.height, s.focused).Render(strings.Join(lines, "\n"))
}

// Track draws the marks on a track width cells wide.
func (s *RangeSlider) Track(width int) string {
	width = max(width, 3)
	low, high := math.Inf(1), math.Inf(-1)
	for _, m := range s.marks {
		if m.Value != nil {
			low, high = math.Min(low, *m.Value), math.Max(high, *m.Value)
		}
	}
	cells := make([]string, width)
	for i := range cells {
		cells[i] = sliderTrackStyle.Render("─")
	}
	if math.IsInf(low, 1) {
		return strings.Join(cells, "")
	}
	for _, m := range s.marks {
		if m.Value == nil {
			continue
		}
		pos := 0
		if high > low {
			pos = int(math.Round((*m.Value - low) / (high - low) * float64(width-1)))
		}
		cells[pos] = m.Style.Render(m.Glyph)
	}
	return strings.Join(cells, "")
}

// labels names each known mark with its value, in its colour.
func (s *RangeSlider) labels() string {
	parts := make([]string, 0, len(s.marks))
	for _, m := range s.marks {
		if m.Value != nil {
			parts = append(parts, m.Style.Render(fmt.Sprintf("%s %s %s", m.Glyph, m.Label, s.format(*m.Value))))
		}
	}
	return strings.Join(parts, "  ")
}
