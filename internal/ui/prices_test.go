package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/marketdata"
	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
)

func TestAppReloadsOnNewPrices(t *testing.T) {
	loads := 0
	dashboards := []Dashboard{{Title: "x", Build: func(*portfolio.Report) Component { return &stub{name: "x"} }}}
	updates := make(chan marketdata.Update, 1)
	app := NewApp(dashboards, func(*model.AccountKey) (portfolio.Report, error) {
		loads++
		return testReport(), nil
	}).WithPrices(updates)
	app.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	app.Update(reportMsg{report: testReport()})

	updates <- marketdata.Update{At: time.Now(), Fetched: 2, Failed: []string{"AMT.US"}}
	msg := app.waitPrices()()
	// Closed so the batch's next wait returns instead of blocking the test.
	close(updates)
	_, cmd := app.Update(msg)
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("new prices produced %T, want a reload and a new wait", cmd())
	}
	for _, c := range batch {
		if c != nil {
			if next := c(); next != nil {
				app.Update(next)
			}
		}
	}

	if loads != 1 {
		t.Errorf("loads = %d, want a reload on new prices", loads)
	}
	footer := app.footer()
	if !strings.Contains(footer, "prices ") || !strings.Contains(footer, "1 prices failed") {
		t.Errorf("footer does not report the poll:\n%s", footer)
	}
}

func TestAppStopsWaitingWhenFeedCloses(t *testing.T) {
	updates := make(chan marketdata.Update)
	close(updates)
	app := NewApp(nil, nil).WithPrices(updates)

	if msg := app.waitPrices()(); msg != nil {
		t.Errorf("closed feed produced %#v, want nothing", msg)
	}
}
