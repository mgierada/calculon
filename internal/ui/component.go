// Package ui composes terminal widgets into fullscreen dashboards.
//
// A widget implements Component and knows nothing about where it sits. A Grid
// says where each one goes and how much room it gets, and is a Component too,
// so layouts nest. A Dashboard builds its component tree from a portfolio
// report; the App shows dashboards as tabs, routes keys to the focused widget,
// and keeps a stack of drill-down screens a widget can push onto.
package ui

import (
	tea "charm.land/bubbletea/v2"
)

// Component is one widget in a layout. The grid owns placement and sizing and
// calls SetSize whenever the terminal is resized; the component renders itself
// inside exactly that box, borders included.
//
// Components are mutable, so implementations are pointer types.
type Component interface {
	// Init returns an optional startup command.
	Init() tea.Cmd
	// Update handles a message, mutating the component.
	Update(msg tea.Msg) tea.Cmd
	// View renders the component at its current size.
	View() string
	// SetSize tells the component the box it must render inside.
	SetSize(width, height int)
	// SetFocused tells the component whether it currently receives key presses.
	SetFocused(focused bool)
}

// InputCapturer is implemented by components that sometimes take free text,
// such as a table while its filter is being typed. While CapturingInput is
// true the app forwards every key to the component instead of treating keys
// like q or 1..9 as app shortcuts.
type InputCapturer interface {
	CapturingInput() bool
}

// PushMsg asks the app to open a drill-down screen on top of the current one.
type PushMsg struct {
	Title  string
	Screen Component
}

// Push returns a command that opens a drill-down screen. Widgets return it from
// Update, e.g. when a table row is selected.
func Push(title string, screen Component) tea.Cmd {
	return func() tea.Msg { return PushMsg{Title: title, Screen: screen} }
}
