package widgets

import (
	tea "charm.land/bubbletea/v2"
)

// Text is a framed block of static text, for notes and empty states.
type Text struct {
	title   string
	body    string
	width   int
	height  int
	focused bool
}

// NewText builds a text panel.
func NewText(title, body string) *Text {
	return &Text{title: title, body: body}
}

// Init implements ui.Component.
func (t *Text) Init() tea.Cmd {
	return nil
}

// Update implements ui.Component. The panel is read-only, so it ignores input.
func (t *Text) Update(tea.Msg) tea.Cmd {
	return nil
}

// SetSize implements ui.Component.
func (t *Text) SetSize(width, height int) {
	t.width, t.height = width, height
}

// SetFocused implements ui.Component.
func (t *Text) SetFocused(focused bool) {
	t.focused = focused
}

// View implements ui.Component.
func (t *Text) View() string {
	return frame(t.width, t.height, t.focused).Render(titleStyle.Render(t.title) + "\n" + t.body)
}
