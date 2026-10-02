package widgets

import (
	"fmt"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// pagerChrome is the title and footer lines a pager draws around its text.
const pagerChrome = 2

var mutedStyle = lipgloss.NewStyle().Foreground(mutedColor)

// Pager is a framed, scrollable document read with pager keys (j/k, f/b,
// d/u). Its content is rendered for the width it gets, so text rewraps when
// the terminal is resized.
type Pager struct {
	title    string
	hint     string
	render   func(width int) string
	viewport viewport.Model
	width    int
	height   int
	focused  bool
}

// NewPager builds a pager whose content render draws for a given width.
func NewPager(title string, render func(width int) string) *Pager {
	return &Pager{title: title, render: render, viewport: viewport.New()}
}

// WithHint adds a key hint to the footer, e.g. for a key the owner handles.
func (p *Pager) WithHint(hint string) *Pager {
	p.hint = hint
	return p
}

// Init implements ui.Component.
func (p *Pager) Init() tea.Cmd {
	return nil
}

// Update implements ui.Component, scrolling on pager keys.
func (p *Pager) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return cmd
}

// SetSize implements ui.Component, rewrapping the content to the new width.
func (p *Pager) SetSize(width, height int) {
	p.width, p.height = width, height
	innerWidth, innerHeight := innerSize(width, height)
	p.viewport.SetWidth(innerWidth)
	p.viewport.SetHeight(max(innerHeight-pagerChrome, 0))
	p.viewport.SetContent(p.render(innerWidth))
}

// SetFocused implements ui.Component.
func (p *Pager) SetFocused(focused bool) {
	p.focused = focused
}

// View implements ui.Component.
func (p *Pager) View() string {
	innerWidth, _ := innerSize(p.width, p.height)
	footer := fmt.Sprintf("j/k scroll · f/b page · esc back · %3.0f%%", p.viewport.ScrollPercent()*100)
	if p.hint != "" {
		footer = p.hint + " · " + footer
	}
	footer = lipgloss.PlaceHorizontal(innerWidth, lipgloss.Right, mutedStyle.Render(footer))
	return frame(p.width, p.height, p.focused).
		Render(titleStyle.Render(p.title) + "\n" + p.viewport.View() + "\n" + footer)
}
