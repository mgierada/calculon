package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// quitKeys close the app. Components never see them.
var quitKeys = map[string]bool{"q": true, "ctrl+c": true, "esc": true}

// focusKey cycles which component receives key presses.
const focusKey = "tab"

// The size the grid starts at, used for the frame rendered before the terminal
// reports its real size and for terminals that never report one.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

// App is the fullscreen root model. It resizes the grid when the terminal
// changes size and routes key presses to the focused component only, while
// every other message goes to all of them.
type App struct {
	grid       Grid
	components []Component
	focus      int
}

// NewApp wires a grid into a runnable app, focusing the first component.
func NewApp(grid Grid) *App {
	app := &App{grid: grid, components: grid.Components()}
	app.grid.Resize(defaultWidth, defaultHeight)
	app.applyFocus()
	return app
}

// Run takes over the terminal with a fullscreen app rendering the grid.
func Run(grid Grid) error {
	if _, err := tea.NewProgram(NewApp(grid)).Run(); err != nil {
		return fmt.Errorf("failed to run ui: %w", err)
	}
	return nil
}

// Init starts every component.
func (a *App) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(a.components))
	for _, component := range a.components {
		cmds = append(cmds, component.Init())
	}
	return tea.Batch(cmds...)
}

// Update handles resize, quit, and focus keys, then forwards the message on.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// A terminal that cannot report its size sends zeros; keeping the
		// current size beats collapsing every component to nothing.
		if msg.Width > 0 && msg.Height > 0 {
			a.grid.Resize(msg.Width, msg.Height)
		}
		return a, nil

	case tea.KeyPressMsg:
		switch key := msg.String(); {
		case quitKeys[key]:
			return a, tea.Quit
		case key == focusKey:
			a.cycleFocus()
			return a, nil
		}
		return a, a.updateFocused(msg)
	}

	return a, a.updateAll(msg)
}

// View renders the grid fullscreen.
func (a *App) View() tea.View {
	view := tea.NewView(a.grid.View())
	view.AltScreen = true
	view.WindowTitle = "calculon"
	return view
}

// updateFocused sends a message to the focused component only.
func (a *App) updateFocused(msg tea.Msg) tea.Cmd {
	if a.focus >= len(a.components) {
		return nil
	}
	return a.components[a.focus].Update(msg)
}

// updateAll sends a message to every component.
func (a *App) updateAll(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(a.components))
	for _, component := range a.components {
		cmds = append(cmds, component.Update(msg))
	}
	return tea.Batch(cmds...)
}

// cycleFocus moves focus to the next component.
func (a *App) cycleFocus() {
	if len(a.components) > 0 {
		a.focus = (a.focus + 1) % len(a.components)
	}
	a.applyFocus()
}

// applyFocus tells each component whether it is the focused one.
func (a *App) applyFocus() {
	for i, component := range a.components {
		component.SetFocused(i == a.focus)
	}
}
