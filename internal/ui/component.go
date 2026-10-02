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

	"github.com/mgierada/calculon/internal/portfolio"
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

// ShortcutHandler is implemented by components with keys that work wherever
// focus is on their screen, like a chart's toggle. Such a key goes to the
// component claiming it ahead of the focused one, unless that one is taking
// text.
type ShortcutHandler interface {
	HandlesShortcut(key string) bool
}

// HeightFitter is implemented by components whose content needs more lines
// when they are narrow, like a card whose text wraps. A grid row with
// AutoHeight grows to the tallest height its cells ask for.
type HeightFitter interface {
	// HeightFor is the height the component needs to show everything at width.
	HeightFor(width int) int
}

// Stateful is implemented by components holding view state worth keeping when
// a reload rebuilds them, like a table's sort and cursor. Restore receives what
// State returned on the component being replaced, and ignores state it does
// not recognise.
type Stateful interface {
	State() any
	Restore(state any)
}

// ScreenBuilder builds a drill-down screen from a report. It runs again on
// every reload that keeps the view, and reports false once what the screen
// shows is gone from the report, which closes it.
type ScreenBuilder func(report *portfolio.Report) (Component, bool)

// PushMsg asks the app to open a drill-down screen on top of the current one.
type PushMsg struct {
	Title string
	Build ScreenBuilder
}

// Push returns a command that opens a drill-down screen. Widgets return it from
// Update, e.g. when a table row is selected.
func Push(title string, build ScreenBuilder) tea.Cmd {
	return func() tea.Msg { return PushMsg{Title: title, Build: build} }
}
