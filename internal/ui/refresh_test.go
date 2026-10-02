package ui

import (
	"context"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
)

// refreshApp has a plain tab and one whose r runs refresh, counting loads.
func refreshApp(t *testing.T, refresh Refresher) (*App, *int) {
	t.Helper()

	loads := 0
	build := func(*portfolio.Report) Component { return &stub{name: "x"} }
	app := NewApp([]Dashboard{
		{Title: "Overview", Build: build},
		{Title: "News", Build: build, Refresh: refresh},
	}, func(*model.AccountKey) (portfolio.Report, error) {
		loads++
		return testReport(), nil
	})
	app.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	app.Update(reportMsg{report: testReport()})
	return app, &loads
}

// plain strips terminal styling and links, leaving the text.
func plain(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;:]*[a-zA-Z]|\x1b\]8;[^\a]*\a`).ReplaceAllString(s, "")
}

// drain feeds the refresh's messages to the app until it finishes.
func drain(t *testing.T, app *App) {
	t.Helper()

	for app.refresh != nil {
		cmd, _ := app.handleRefresh(app.waitRefresh()())
		if cmd != nil && app.refresh == nil {
			if msg, ok := cmd().(reportMsg); ok {
				app.Update(msg)
			}
		}
	}
}

func TestRefreshRunsOnlyOnItsTab(t *testing.T) {
	calls := 0
	app, loads := refreshApp(t, func(context.Context, *portfolio.Report, func(Progress)) (string, error) {
		calls++
		return "", nil
	})

	app.Update(press("r"))
	if calls != 0 || app.refresh != nil {
		t.Error("r on a tab without a refresh started one")
	}

	app.Update(press("2"))
	app.Update(press("r"))
	if app.refresh == nil {
		t.Fatal("r on the news tab did not start its refresh")
	}
	drain(t, app)
	if calls != 1 || *loads != 1 {
		t.Errorf("refresh ran %d times then loaded %d times, want once each", calls, *loads)
	}
}

func TestRefreshShowsProgressThenNotice(t *testing.T) {
	step := make(chan struct{})
	app, _ := refreshApp(t, func(_ context.Context, _ *portfolio.Report, progress func(Progress)) (string, error) {
		progress(Progress{Done: 0, Total: 2, Current: "XTB.PL"})
		progress(Progress{Done: 1, Total: 2, Finished: "XTB.PL", Detail: "3 new"})
		<-step
		return "news: 3 new", nil
	})
	app.Update(press("2"))
	app.Update(press("r"))

	app.handleRefresh(app.waitRefresh()())
	app.handleRefresh(app.waitRefresh()())
	view := plain(app.View().Content)
	for _, want := range []string{"Refreshing News", "✓ XTB.PL", "3 new", "1/2", "esc to cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("progress box is missing %q:\n%s", want, view)
		}
	}

	close(step)
	drain(t, app)
	if footer := app.footer(); !strings.Contains(footer, "news: 3 new") {
		t.Errorf("footer does not report the refresh:\n%s", footer)
	}
}

func TestRefreshCancelsOnEsc(t *testing.T) {
	app, _ := refreshApp(t, func(ctx context.Context, _ *portfolio.Report, _ func(Progress)) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	app.Update(press("2"))
	app.Update(press("r"))

	app.Update(escKey)
	drain(t, app)

	if footer := app.footer(); !strings.Contains(footer, "news refresh cancelled") {
		t.Errorf("footer does not say the refresh was cancelled:\n%s", footer)
	}
}

func TestNoticeShowsThenExpires(t *testing.T) {
	app, _ := refreshApp(t, nil)

	_, cmd := app.Update(NoticeMsg("link copied to system clipboard"))
	if !strings.Contains(app.footer(), "link copied to system clipboard") {
		t.Errorf("footer does not show the notice:\n%s", app.footer())
	}
	if cmd == nil {
		t.Fatal("notice has no expiry")
	}

	// A newer notice is not cleared by the older one expiring.
	app.Update(NoticeMsg("newer"))
	app.Update(noticeExpiredMsg("link copied to system clipboard"))
	if !strings.Contains(app.footer(), "newer") {
		t.Errorf("older notice's expiry cleared the newer one:\n%s", app.footer())
	}
	app.Update(noticeExpiredMsg("newer"))
	if strings.Contains(app.footer(), "newer") {
		t.Errorf("notice still shown after expiring:\n%s", app.footer())
	}
}
