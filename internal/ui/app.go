package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mgierada/calculon/internal/portfolio"
)

// Keys the app handles itself. Everything else goes to the focused component.
const (
	keyForceQuit = "ctrl+c"
	keyQuit      = "q"
	keyBack      = "esc"
	keyFocusNext = "tab"
	keyFocusPrev = "shift+tab"
	keyTabPrev   = "["
	keyTabNext   = "]"
	keyReload    = "r"
	keyHelp      = "?"
)

// chromeHeight is the header and footer line around the screen body.
const chromeHeight = 2

// The size the app starts at, used for the frame rendered before the terminal
// reports its real size and for terminals that never report one.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

var (
	tabStyle       = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("245"))
	activeTabStyle = lipgloss.NewStyle().Padding(0, 1).Bold(true).
			Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62"))
	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Padding(0, 1)
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

const helpText = `keys

  1..9  [ ]      switch dashboard
  tab shift+tab  move focus between widgets
  ↑ ↓ j k        move within a table
  ← → h l        change table page
  enter          open details of the selected row
  /              filter a table (enter to keep, esc to clear)
  g              change what an allocation chart groups by
  esc            close details
  r              reload data
  ?              toggle this help
  q ctrl+c       quit`

// reportMsg delivers a loaded report.
type reportMsg struct {
	report portfolio.Report
	err    error
}

// screen is one component tree with its own focus: a dashboard tab or a
// drill-down pushed on top of one.
type screen struct {
	title  string
	root   Component
	leaves []Component
	focus  int
}

func newScreen(title string, root Component) *screen {
	s := &screen{title: title, root: root}
	if grid, ok := root.(*Grid); ok {
		s.leaves = grid.Components()
	} else {
		s.leaves = []Component{root}
	}
	s.applyFocus()
	return s
}

func (s *screen) focused() Component {
	if s.focus >= len(s.leaves) {
		return nil
	}
	return s.leaves[s.focus]
}

func (s *screen) cycleFocus(step int) {
	if n := len(s.leaves); n > 0 {
		s.focus = ((s.focus+step)%n + n) % n
	}
	s.applyFocus()
}

func (s *screen) applyFocus() {
	for i, leaf := range s.leaves {
		leaf.SetFocused(i == s.focus)
	}
}

// App is the fullscreen root model: a tab per dashboard, a stack of drill-down
// screens over the active tab, and a header and footer around them.
type App struct {
	dashboards []Dashboard
	load       Loader
	tabs       []*screen
	stack      []*screen
	active     int
	width      int
	height     int
	report     *portfolio.Report
	err        error
	showHelp   bool
}

// NewApp wires dashboards to the loader that feeds them.
func NewApp(dashboards []Dashboard, load Loader) *App {
	return &App{dashboards: dashboards, load: load, width: defaultWidth, height: defaultHeight}
}

// Run takes over the terminal with the app.
func Run(app *App, opts ...tea.ProgramOption) error {
	if _, err := tea.NewProgram(app, opts...).Run(); err != nil {
		return fmt.Errorf("failed to run ui: %w", err)
	}
	return nil
}

// Init loads the first report.
func (a *App) Init() tea.Cmd {
	return a.reload()
}

// Update handles app-level messages and keys, then forwards the rest.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// A terminal that cannot report its size sends zeros; keeping the
		// current size beats collapsing every component to nothing.
		if msg.Width > 0 && msg.Height > 0 {
			a.width, a.height = msg.Width, msg.Height
			a.resize()
		}
		return a, nil
	case reportMsg:
		return a, a.handleReport(msg)
	case PushMsg:
		return a, a.push(newScreen(msg.Title, msg.Screen))
	case tea.KeyPressMsg:
		return a, a.handleKey(msg)
	}
	if current := a.current(); current != nil {
		return a, current.root.Update(msg)
	}
	return a, nil
}

// View renders the header, the current screen and the footer.
func (a *App) View() tea.View {
	body := a.body()
	view := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, a.header(), body, a.footer()))
	view.AltScreen = true
	view.WindowTitle = "calculon"
	return view
}

// handleReport rebuilds every dashboard from a freshly loaded report. Open
// drill-downs are closed because they were built from the old one.
func (a *App) handleReport(msg reportMsg) tea.Cmd {
	a.err = msg.err
	if msg.err != nil {
		return nil
	}
	a.report = &msg.report
	a.stack = nil
	a.tabs = make([]*screen, len(a.dashboards))
	cmds := make([]tea.Cmd, len(a.dashboards))
	for i, dashboard := range a.dashboards {
		a.tabs[i] = newScreen(dashboard.Title, dashboard.Build(a.report))
		cmds[i] = a.tabs[i].root.Init()
	}
	a.resize()
	return tea.Batch(cmds...)
}

// handleKey applies app shortcuts, deferring to a component taking free text.
func (a *App) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	if key == keyForceQuit {
		return tea.Quit
	}
	current := a.current()
	if capturer, ok := focusedOf(current).(InputCapturer); ok && capturer.CapturingInput() {
		return current.focused().Update(msg)
	}

	switch key {
	case keyQuit:
		return tea.Quit
	case keyHelp:
		a.showHelp = !a.showHelp
		return nil
	case keyReload:
		return a.reload()
	case keyTabPrev, keyTabNext:
		step := 1
		if key == keyTabPrev {
			step = -1
		}
		a.selectTab(a.active + step)
		return nil
	case keyBack:
		if a.showHelp {
			a.showHelp = false
			return nil
		}
		if len(a.stack) > 0 {
			a.stack = a.stack[:len(a.stack)-1]
			return nil
		}
	case keyFocusNext, keyFocusPrev:
		if current != nil {
			step := 1
			if key == keyFocusPrev {
				step = -1
			}
			current.cycleFocus(step)
		}
		return nil
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		a.selectTab(int(key[0] - '1'))
		return nil
	}

	if focused := focusedOf(current); focused != nil {
		return focused.Update(msg)
	}
	return nil
}

// reload fetches a new report off the UI loop.
func (a *App) reload() tea.Cmd {
	load := a.load
	return func() tea.Msg {
		report, err := load()
		return reportMsg{report: report, err: err}
	}
}

// push opens a drill-down screen.
func (a *App) push(s *screen) tea.Cmd {
	a.stack = append(a.stack, s)
	s.root.SetSize(a.bodySize())
	return s.root.Init()
}

// selectTab switches dashboards, wrapping around and closing drill-downs.
func (a *App) selectTab(i int) {
	if n := len(a.tabs); n > 0 {
		a.active = (i%n + n) % n
		a.stack = nil
	}
}

// current is the screen on top: the last drill-down or the active tab.
func (a *App) current() *screen {
	if n := len(a.stack); n > 0 {
		return a.stack[n-1]
	}
	if a.active < len(a.tabs) {
		return a.tabs[a.active]
	}
	return nil
}

func focusedOf(s *screen) Component {
	if s == nil {
		return nil
	}
	return s.focused()
}

func (a *App) resize() {
	width, height := a.bodySize()
	for _, tab := range a.tabs {
		tab.root.SetSize(width, height)
	}
	for _, s := range a.stack {
		s.root.SetSize(width, height)
	}
}

func (a *App) bodySize() (int, int) {
	return a.width, max(a.height-chromeHeight, 0)
}

func (a *App) body() string {
	width, height := a.bodySize()
	place := func(content string) string {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
	}
	switch {
	case a.showHelp:
		return place(helpText)
	case a.err != nil:
		return place(errorStyle.Render(a.err.Error()) + "\n\npress r to retry, q to quit")
	case a.report == nil:
		return place("loading…")
	}
	return a.current().root.View()
}

// header shows the brand, one tab per dashboard, and the user on the right.
func (a *App) header() string {
	parts := []string{brandStyle.Render("calculon")}
	for i, dashboard := range a.dashboards {
		label := fmt.Sprintf("%d %s", i+1, dashboard.Title)
		style := tabStyle
		if i == a.active {
			style = activeTabStyle
		}
		parts = append(parts, style.Render(label))
	}
	for _, s := range a.stack {
		parts = append(parts, tabStyle.Render("› "+s.title))
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	var right string
	if a.report != nil {
		right = footerStyle.Render(a.report.User + " ")
	}
	return fill(left, right, a.width)
}

// footer shows data freshness, FX provenance, warnings and a key hint.
func (a *App) footer() string {
	hint := footerStyle.Render("? help · q quit ")
	if a.report == nil {
		return fill("", hint, a.width)
	}
	info := []string{}
	if !a.report.AsOf.IsZero() {
		info = append(info, "as of "+a.report.AsOf.Format("2006-01-02 15:04 MST"))
	}
	info = append(info, a.report.FXNote)
	left := footerStyle.Render(" " + strings.Join(info, " · "))
	if len(a.report.Warnings) > 0 {
		left += warningStyle.Render(" · ⚠ " + a.report.Warnings[0])
	}
	return fill(left, hint, a.width)
}

// fill lays out left and right parts on one line of the given width, truncating
// the left part first when both do not fit.
func fill(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		left = lipgloss.NewStyle().MaxWidth(max(width-lipgloss.Width(right), 0)).Render(left)
		gap = 0
	}
	return left + strings.Repeat(" ", gap) + right
}
