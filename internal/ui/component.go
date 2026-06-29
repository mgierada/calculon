// Package ui composes terminal widgets into a fullscreen layout.
//
// A widget implements Component and knows nothing about where it sits. A Grid
// says where each one goes and how much room it gets, so putting a chart beside
// or below a table is a change to the grid, not to either widget.
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
