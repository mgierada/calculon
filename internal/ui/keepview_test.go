package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/marketdata"
	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
)

// statefulStub carries a piece of view state across rebuilds.
type statefulStub struct {
	stub
	state string
}

func (s *statefulStub) State() any { return s.state }

func (s *statefulStub) Restore(state any) {
	if text, ok := state.(string); ok {
		s.state = text
	}
}

// typingStub takes text until enter.
type typingStub struct {
	stub
	typing bool
}

func (t *typingStub) CapturingInput() bool { return t.typing }

func (t *typingStub) Update(msg tea.Msg) tea.Cmd {
	if key, ok := msg.(tea.KeyPressMsg); ok && key.Code == tea.KeyEnter {
		t.typing = false
	}
	return nil
}

// statefulApp has one dashboard of two stateful leaves, built afresh each load.
func statefulApp(t *testing.T) *App {
	t.Helper()

	dashboards := []Dashboard{{Title: "x", Build: func(*portfolio.Report) Component {
		return Stack(&statefulStub{stub: stub{name: "a"}}, &statefulStub{stub: stub{name: "b"}})
	}}}
	app := NewApp(dashboards, func(*model.AccountKey) (portfolio.Report, error) { return testReport(), nil })
	app.Update(reportMsg{report: testReport()})
	return app
}

func leafState(app *App, i int) string {
	return app.tabs[0].leaves[i].(*statefulStub).state
}

func TestAppKeepsViewAcrossRefresh(t *testing.T) {
	app := statefulApp(t)
	app.Update(tabKey)
	app.tabs[0].leaves[1].(*statefulStub).state = "sorted by P/L"
	builds := 0
	app.Update(PushMsg{Title: "SNT.PL", Build: func(*portfolio.Report) (Component, bool) {
		builds++
		return &stub{name: "detail"}, true
	}})

	app.Update(reportMsg{report: testReport(), keepView: true})

	if got := leafState(app, 1); got != "sorted by P/L" {
		t.Errorf("leaf state = %q, want it carried over", got)
	}
	if app.tabs[0].focus != 1 || !app.tabs[0].leaves[1].(*statefulStub).focused {
		t.Errorf("focus = %d, want the second leaf kept focused", app.tabs[0].focus)
	}
	if len(app.stack) != 1 || builds != 2 {
		t.Errorf("stack = %d screens after %d builds, want the drill-down rebuilt", len(app.stack), builds)
	}
}

func TestAppClosesDrillDownWhoseSubjectIsGone(t *testing.T) {
	app := statefulApp(t)
	held := true
	app.Update(PushMsg{Title: "SNT.PL", Build: func(*portfolio.Report) (Component, bool) {
		return &stub{name: "detail"}, held
	}})

	held = false
	app.Update(reportMsg{report: testReport(), keepView: true})

	if len(app.stack) != 0 {
		t.Error("drill-down kept open for a holding no longer in the report")
	}
}

func TestAppResetsViewWhenSwitchingData(t *testing.T) {
	app := statefulApp(t)
	app.tabs[0].leaves[0].(*statefulStub).state = "filtered"
	app.Update(PushMsg{Title: "SNT.PL", Build: fixedScreen(&stub{name: "detail"})})

	app.Update(reportMsg{report: testReport()})

	if got := leafState(app, 0); got != "" || len(app.stack) != 0 {
		t.Errorf("state = %q with %d drill-downs, want a fresh view", got, len(app.stack))
	}
}

// Regression guard: a refresh while typing a filter would drop the filter
// input's focus, turning the next letters into shortcuts such as q.
func TestAppHoldsRefreshWhileTyping(t *testing.T) {
	typing := &typingStub{stub: stub{name: "table"}, typing: true}
	app := loadedApp(t, typing).WithPrices(make(chan marketdata.Update))

	if cmd := app.handlePrices(pricesMsg{}); cmd == nil || !app.staleView {
		t.Fatal("refresh not held back while typing")
	}

	_, cmd := app.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil || app.staleView {
		t.Error("held refresh not applied once typing ended")
	}
}
