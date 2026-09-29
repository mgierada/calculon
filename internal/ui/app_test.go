package ui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/portfolio"
)

// keyStub records the keys it was sent.
type keyStub struct {
	stub
	keys []string
}

func (k *keyStub) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		k.keys = append(k.keys, key.String())
	}
	return nil
}

// capturingStub takes free text, like a table with its filter focused.
type capturingStub struct {
	keyStub
}

func (c *capturingStub) CapturingInput() bool { return true }

func press(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: key, Code: rune(key[0])})
}

var (
	tabKey = tea.KeyPressMsg(tea.Key{Code: tea.KeyTab})
	escKey = tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})
)

func testReport() portfolio.Report {
	return portfolio.Report{User: "alice", FXNote: "static FX"}
}

// loadedApp is an app whose report has arrived, with one dashboard per root.
func loadedApp(t *testing.T, roots ...Component) *App {
	t.Helper()

	dashboards := make([]Dashboard, len(roots))
	for i, root := range roots {
		dashboards[i] = Dashboard{Title: root.View(), Build: func(*portfolio.Report) Component { return root }}
	}
	app := NewApp(dashboards, func() (portfolio.Report, error) { return testReport(), nil })
	app.Update(reportMsg{report: testReport()})
	return app
}

func TestAppLoadsReportOnInit(t *testing.T) {
	app := NewApp(nil, func() (portfolio.Report, error) { return testReport(), nil })

	msg := app.Init()()
	if got, ok := msg.(reportMsg); !ok || got.report.User != "alice" {
		t.Fatalf("Init produced %#v, want the loaded report", msg)
	}
}

func TestAppShowsLoadErrors(t *testing.T) {
	app := NewApp(nil, nil)
	app.Update(reportMsg{err: errors.New("database is locked")})

	if view := app.View(); !strings.Contains(view.Content, "database is locked") {
		t.Errorf("view does not show the load error:\n%s", view.Content)
	}
}

func TestAppFocusesFirstComponent(t *testing.T) {
	first, second := &stub{name: "first"}, &stub{name: "second"}
	loadedApp(t, Stack(first, second))

	if !first.focused || second.focused {
		t.Errorf("focus = first %v, second %v; want only first", first.focused, second.focused)
	}
}

func TestAppRoutesKeysToFocusedComponentOnly(t *testing.T) {
	first, second := &keyStub{stub: stub{name: "first"}}, &keyStub{stub: stub{name: "second"}}
	app := loadedApp(t, Stack(first, second))

	app.Update(press("j"))

	if len(first.keys) != 1 || first.keys[0] != "j" {
		t.Errorf("focused component received %v, want [j]", first.keys)
	}
	if len(second.keys) != 0 {
		t.Errorf("unfocused component received %v, want nothing", second.keys)
	}
}

func TestAppCyclesFocusOnTab(t *testing.T) {
	first, second := &keyStub{stub: stub{name: "first"}}, &keyStub{stub: stub{name: "second"}}
	app := loadedApp(t, Stack(first, second))

	app.Update(tabKey)
	if first.focused || !second.focused {
		t.Fatal("tab did not move focus to the second component")
	}
	app.Update(press("k"))
	if len(second.keys) != 1 || second.keys[0] != "k" {
		t.Errorf("second component received %v, want [k]", second.keys)
	}

	app.Update(tabKey)
	if !first.focused || second.focused {
		t.Error("tab did not wrap focus back to the first component")
	}
}

func TestAppFocusReachesNestedGrids(t *testing.T) {
	outer, inner := &keyStub{stub: stub{name: "outer"}}, &keyStub{stub: stub{name: "inner"}}
	app := loadedApp(t, &Grid{Rows: []Row{{Cells: Cells(outer, Stack(inner))}}})

	app.Update(tabKey)
	app.Update(press("x"))

	if len(inner.keys) != 1 {
		t.Errorf("nested component received %v, want [x]", inner.keys)
	}
}

func TestAppSwitchesDashboardsWithNumberKeys(t *testing.T) {
	overview, positions := &stub{name: "overview"}, &stub{name: "positions"}
	app := loadedApp(t, overview, positions)

	app.Update(press("2"))
	if view := app.View().Content; !strings.Contains(view, "positions") {
		t.Errorf("after 2 the body does not show positions:\n%s", view)
	}

	app.Update(press("]"))
	if app.active != 0 {
		t.Errorf("] from the last dashboard went to %d, want wrap to 0", app.active)
	}
}

func TestAppPushesAndPopsDrillDowns(t *testing.T) {
	app := loadedApp(t, &stub{name: "table"})

	app.Update(PushMsg{Title: "SNT.PL", Screen: &stub{name: "detail"}})
	if view := app.View().Content; !strings.Contains(view, "detail") || !strings.Contains(view, "SNT.PL") {
		t.Errorf("pushed screen not shown:\n%s", view)
	}

	app.Update(escKey)
	if len(app.stack) != 0 {
		t.Error("esc did not close the drill-down")
	}
}

func TestAppForwardsKeysToCapturingComponent(t *testing.T) {
	filter := &capturingStub{keyStub: keyStub{stub: stub{name: "table"}}}
	app := loadedApp(t, filter)

	_, cmd := app.Update(press("q"))
	app.Update(press("2"))

	if cmd != nil {
		t.Error("q quit while a component was taking text")
	}
	if len(filter.keys) != 2 {
		t.Errorf("capturing component received %v, want [q 2]", filter.keys)
	}
}

func TestAppQuits(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{press("q"), tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl})} {
		app := loadedApp(t, &stub{name: "table"})
		if _, cmd := app.Update(key); cmd == nil {
			t.Errorf("key %q returned no command, want quit", key.String())
		}
	}
}

func TestAppResizesBelowChrome(t *testing.T) {
	table := &stub{name: "table"}
	app := loadedApp(t, table)

	app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	if got, want := table.box(), "100x38"; got != want {
		t.Errorf("table box = %s, want %s", got, want)
	}
}

func TestAppStartsAtADefaultSize(t *testing.T) {
	table := &stub{name: "table"}
	loadedApp(t, table)

	if got, want := table.box(), "80x22"; got != want {
		t.Errorf("table box before any resize = %s, want %s", got, want)
	}
}

// A terminal that cannot report its size must not collapse the layout.
func TestAppIgnoresZeroSize(t *testing.T) {
	table := &stub{name: "table"}
	app := loadedApp(t, table)

	app.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	app.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	if got, want := table.box(), "100x38"; got != want {
		t.Errorf("table box after a zero resize = %s, want %s", got, want)
	}
}

func TestAppViewIsFullscreen(t *testing.T) {
	app := loadedApp(t, &stub{name: "table"})

	view := app.View()
	if !view.AltScreen {
		t.Error("view does not request the alternate screen")
	}
	if !strings.Contains(view.Content, "alice") {
		t.Errorf("header does not name the user:\n%s", view.Content)
	}
}
