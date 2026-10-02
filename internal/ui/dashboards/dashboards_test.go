package dashboards

import (
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
)

var (
	account = model.Account{Provider: model.ProviderXTB, ID: "50747414", Currency: "PLN"}
	asOf    = time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	opened  = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
)

func testReport() *portfolio.Report {
	lot := func(id string, symbol string, volume, open, current float64) model.Owned[model.OpenLot] {
		return model.Owned[model.OpenLot]{Account: account, Record: model.OpenLot{
			Instrument: model.Instrument{Symbol: symbol, Name: symbol + " SA", Category: "STOCK"},
			PositionID: id, Side: model.SideBuy, Volume: volume, OpenTime: opened,
			OpenPrice: open, CurrentPrice: current, Value: volume * current,
		}}
	}
	in := portfolio.Input{
		User:     "alice",
		Accounts: []model.AccountSnapshot{{Account: account, AsOf: asOf}},
		Lots:     []model.Owned[model.OpenLot]{lot("1", "SNT.PL", 10, 300, 346.4), lot("2", "CDR.PL", 5, 250, 240)},
		Closed: []model.Owned[model.Position]{{Account: account, Record: model.Position{
			Instrument: model.Instrument{Symbol: "SNT.PL", Category: "STOCK"},
			PositionID: "9", Side: model.SideBuy, Volume: 2, OpenTime: opened, OpenPrice: 290,
			CloseTime: opened.AddDate(0, 0, 5), ClosePrice: 310, NetPL: 40,
		}}},
		CashOps: []model.Owned[model.CashOp]{
			{Account: account, Record: model.CashOp{
				ExternalID: "c1", Kind: model.CashOpDeposit, RawType: "Deposit",
				Time: opened.Add(-time.Hour), Amount: 5000,
			}},
			{Account: account, Record: model.CashOp{
				Instrument: model.Instrument{Symbol: "SNT.PL"},
				ExternalID: "c2", Kind: model.CashOpDividend, RawType: "Dividend",
				Time: opened.AddDate(0, 0, 10), Amount: 25,
			}},
		},
		Quotes: []model.Quote{
			{Symbol: "SNT.PL", AsOf: asOf, Price: 346.4},
			{Symbol: "CDR.PL", AsOf: asOf, Price: 240},
		},
	}
	report := portfolio.Build(in, portfolio.Options{FX: portfolio.NewStaticFX("PLN", nil)})
	return &report
}

func render(component ui.Component, width, height int) string {
	component.SetSize(width, height)
	return component.View()
}

func TestDashboardsRenderAtTheirSize(t *testing.T) {
	report := testReport()
	for _, dashboard := range All(nil) {
		for _, size := range [][2]int{{160, 50}, {80, 24}, {40, 10}} {
			view := render(dashboard.Build(report), size[0], size[1])
			if w := lipgloss.Width(view); w > size[0] {
				t.Errorf("%s at %v is %d wide", dashboard.Title, size, w)
			}
		}
	}
}

func TestOverviewShowsTotals(t *testing.T) {
	view := render(Overview(testReport()), 200, 60)

	// 10 * 346.4 + 5 * 240 = 4664 in positions.
	for _, want := range []string{"Total value", "4 664.00 PLN", "SNT.PL", "needs a previous close"} {
		if !strings.Contains(view, want) {
			t.Errorf("overview is missing %q", want)
		}
	}
}

func TestPositionsSelectOpensDetail(t *testing.T) {
	report := testReport()
	table := PositionsTable(report)
	table.SetSize(160, 20)
	table.SetFocused(true)

	cmd := table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("enter on a position returned no command")
	}
	push, ok := cmd().(ui.PushMsg)
	if !ok {
		t.Fatalf("enter produced %T, want ui.PushMsg", cmd())
	}
	if push.Title != report.Holdings[0].Symbol {
		t.Errorf("pushed %q, want the first holding %q", push.Title, report.Holdings[0].Symbol)
	}

	screen, ok := push.Build(report)
	if !ok {
		t.Fatal("detail could not be built from the report it was opened from")
	}
	detail := render(screen, 160, 40)
	if !strings.Contains(detail, "open lots") || !strings.Contains(detail, "closed "+push.Title) {
		t.Errorf("detail screen:\n%s", detail)
	}
}

func TestAllocationChartCyclesGrouping(t *testing.T) {
	chart := NewAllocationChart(testReport())
	chart.SetSize(80, 12)

	if !strings.Contains(chart.View(), "by symbol") {
		t.Fatalf("chart does not start grouped by symbol:\n%s", chart.View())
	}
	chart.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	if view := chart.View(); !strings.Contains(view, "by account") || !strings.Contains(view, "50747414") {
		t.Errorf("chart after g:\n%s", view)
	}
}

func TestMoney(t *testing.T) {
	tests := []struct {
		amount   float64
		currency string
		want     string
	}{
		{132651.9, "PLN", "132 651.90 PLN"},
		{-1234.5, "", "-1 234.50"},
		{0.001, "USD", "0.00 USD"},
		{-0.001, "USD", "0.00 USD"},
		{999, "", "999.00"},
	}
	for _, test := range tests {
		if got := money(test.amount, test.currency); got != test.want {
			t.Errorf("money(%v, %q) = %q, want %q", test.amount, test.currency, got, test.want)
		}
	}
}

func TestPriceAndVolume(t *testing.T) {
	if got := price(21580); got != "21580.00" {
		t.Errorf("price(21580) = %q", got)
	}
	if got := price(0.0083); got != "0.0083" {
		t.Errorf("price(0.0083) = %q", got)
	}
	if got := price(21566.737337); got != "21566.7373" {
		t.Errorf("price(21566.737337) = %q, want four decimals at most", got)
	}
	if got := volume(42.70450000001); got != "42.7045" {
		t.Errorf("volume = %q, want float noise trimmed", got)
	}
}

func TestPercentCapsHugeValues(t *testing.T) {
	if got := signedPercent(233099900); got != ">+9999%" {
		t.Errorf("signedPercent(huge) = %q", got)
	}
	if got := percent(-50000); got != ">-9999%" {
		t.Errorf("percent(-huge) = %q", got)
	}
	if got := signedPercent(12.34); got != "+12.3%" {
		t.Errorf("signedPercent(12.34) = %q", got)
	}
}

func reportWithReturns() *portfolio.Report {
	report := testReport()
	for i := range 30 {
		report.Returns.Series = append(report.Returns.Series, portfolio.ReturnPoint{
			Time: asOf.AddDate(0, 0, i-30), TWR: 20, XIRR: 12, CAGR: 9, HasXIRR: true, HasCAGR: true,
		})
	}
	report.Returns.TWR = portfolio.Delta{Amount: 20, Pct: 20, Known: true}
	report.Returns.XIRR = portfolio.Delta{Amount: 12, Pct: 12, Known: true}
	report.Returns.CAGR = portfolio.Delta{Amount: 9, Pct: 9, Known: true}
	return report
}

func TestReturnChartCyclesMetricsFromTWR(t *testing.T) {
	chart := NewReturnChart(reportWithReturns())
	chart.SetSize(80, 16)

	for _, want := range []string{"TWR over time, % since start", "XIRR over time, % a year",
		"CAGR over time, % a year", "TWR over time"} {
		view := chart.View()
		if !strings.Contains(view, want) || strings.Contains(view, "no data") {
			t.Errorf("chart does not show %q:\n%s", want, view)
		}
		if w, h := lipgloss.Width(view), lipgloss.Height(view); w != 80 || h != 16 {
			t.Errorf("chart is %dx%d, want it to keep its 80x16 box", w, h)
		}
		chart.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))
	}
}

func TestReturnStat(t *testing.T) {
	view := render(ReturnStat(reportWithReturns()), 34, 5)
	for _, want := range []string{"Return (TWR)", "+20.0%", "XIRR +12.0% · CAGR +9.0%"} {
		if !strings.Contains(view, want) {
			t.Errorf("return card is missing %q:\n%s", want, view)
		}
	}

	// The test report spans under MinReturnDays: TWR is known, annual rates not.
	view = render(ReturnStat(testReport()), 34, 5)
	if !strings.Contains(view, "annual after 90 days") {
		t.Errorf("return card without enough history:\n%s", view)
	}
}

func TestPositionsStartSortedByPLDescending(t *testing.T) {
	report := testReport()
	table := PositionsTable(report)
	table.SetSize(200, 20)
	table.SetFocused(true)

	// SNT.PL gains 464 while CDR.PL loses 50, so SNT.PL leads.
	view := table.View()
	if !strings.Contains(view, "P/L ▼") {
		t.Errorf("P/L column is not marked as the sort:\n%s", view)
	}
	cmd := table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if push, ok := cmd().(ui.PushMsg); !ok || push.Title != "SNT.PL" {
		t.Errorf("top row opens %+v, want SNT.PL, the biggest P/L", cmd())
	}

	// Reversed, the loss comes first, which ordering by value never gives.
	table.Update(tea.KeyPressMsg(tea.Key{Text: "S", Code: 'S'}))
	cmd = table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if push, ok := cmd().(ui.PushMsg); !ok || push.Title != "CDR.PL" {
		t.Errorf("after S the top row opens %+v, want CDR.PL, the loss", cmd())
	}
}

// Regression: at a width like a laptop terminal the KPI notes were cut off.
func TestOverviewStatsShowEveryNoteWhenNarrow(t *testing.T) {
	grid := Overview(testReport()).(*ui.Grid)
	grid.SetSize(160, 50)

	var cards []string
	for _, cell := range grid.Rows[0].Cells {
		view := regexp.MustCompile(`\x1b\[[0-9;:]*[a-zA-Z]`).ReplaceAllString(cell.Component.View(), "")
		view = strings.NewReplacer("│", " ", "╭", " ", "╮", " ", "╰", " ", "╯", " ", "─", " ").Replace(view)
		text := strings.Join(strings.Fields(view), " ")
		cards = append(cards, text)
	}
	for card, want := range map[string]string{
		"Total":     "net deposits 5 000.00 PLN",
		"Positions": "cost 4 250.00 PLN",
		"Today":     "needs a previous close",
	} {
		found := false
		for _, text := range cards {
			if strings.HasPrefix(text, card) && strings.Contains(text, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s card at 160 columns lost %q; cards: %q", card, want, cards)
		}
	}
}

// A price refresh that reorders positions must leave the cursor on the same
// holding, or enter opens a different one than the user picked.
func TestPositionsCursorFollowsHoldingAcrossRefresh(t *testing.T) {
	old := PositionsTable(testReport())
	old.SetSize(200, 20)
	old.SetFocused(true)
	old.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))

	refreshed := testReport()
	for i := range refreshed.Holdings {
		if refreshed.Holdings[i].Symbol == "CDR.PL" {
			refreshed.Holdings[i].PL.Amount = 10_000
		}
	}
	table := PositionsTable(refreshed)
	table.SetSize(200, 20)
	table.SetFocused(true)
	table.Restore(old.State())

	cmd := table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if push, ok := cmd().(ui.PushMsg); !ok || push.Title != "CDR.PL" {
		t.Errorf("enter opens %+v, want CDR.PL, the row the cursor was on", cmd())
	}
}

func TestHoldingDetailClosesOnceHoldingIsGone(t *testing.T) {
	table := PositionsTable(testReport())
	table.SetSize(160, 20)
	table.SetFocused(true)
	push := table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))().(ui.PushMsg)

	sold := testReport()
	sold.Holdings = nil
	if _, ok := push.Build(sold); ok {
		t.Error("detail rebuilt for a holding no longer held")
	}
}

// staleReport is testReport with SNT.PL priced only from an earlier session.
func staleReport() *portfolio.Report {
	report := testReport()
	for i := range report.Holdings {
		if h := &report.Holdings[i]; h.Symbol == "SNT.PL" {
			h.Day = portfolio.Delta{Amount: 20, Pct: 0.58, Known: true}
			h.DayStale, h.DayAsOf = true, time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
		}
	}
	report.Totals.Day, report.Totals.DayStale = portfolio.Delta{}, 1
	return report
}

func TestStaleDayChangeIsLabelledWithItsSession(t *testing.T) {
	report := staleReport()

	if view := render(PositionsTable(report), 200, 20); !strings.Contains(view, "+0.6% 29 Sep") {
		t.Errorf("positions do not date the stale move:\n%s", view)
	}
	for _, h := range report.Holdings {
		if h.Symbol == "SNT.PL" {
			if view := render(dayStat(h), 30, 5); !strings.Contains(view, "as of 29 Sep") {
				t.Errorf("detail card does not date the stale move:\n%s", view)
			}
		}
	}
	if view := render(totalDayStat(report.Totals, report.Base), 30, 5); !strings.Contains(view, "1 stale excluded") {
		t.Errorf("total card does not say what it leaves out:\n%s", view)
	}
}

func TestReturnChartShowsEveryAccountBesideAll(t *testing.T) {
	report := testReport()
	ike := model.Account{Provider: model.ProviderXTB, ID: "51099570", Currency: "PLN", Name: "IKE"}
	report.AccountReturns = []portfolio.AccountReturn{
		{Account: account, Returns: report.Returns},
		{Account: ike, Returns: report.Returns},
	}

	view := render(NewReturnChart(report), 120, 20)
	for _, want := range []string{"all", account.ID, "IKE"} {
		if !strings.Contains(view, want) {
			t.Errorf("legend is missing %q:\n%s", want, view)
		}
	}
	if single := render(NewReturnChart(testReport()), 120, 20); strings.Contains(single, "all") {
		t.Errorf("a single account's chart is labelled all:\n%s", single)
	}
}

// g and m reach their charts from anywhere on the overview, e.g. with the
// accounts table focused.
func TestOverviewChartKeysWorkWithoutFocus(t *testing.T) {
	report := testReport()
	app := ui.NewApp([]ui.Dashboard{{Title: "Overview", Build: Overview}},
		func(*model.AccountKey) (portfolio.Report, error) { return *report, nil })
	app.Update(app.Init()())
	app.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	app.Update(tea.KeyPressMsg(tea.Key{Text: "g", Code: 'g'}))
	app.Update(tea.KeyPressMsg(tea.Key{Text: "m", Code: 'm'}))

	view := app.View().Content
	if !strings.Contains(view, "% of portfolio by account") {
		t.Errorf("g did not regroup the allocation chart:\n%s", view)
	}
	if !strings.Contains(view, "XIRR over time") {
		t.Errorf("m did not switch the return chart:\n%s", view)
	}
}

func newsReport() *portfolio.Report {
	report := testReport()
	report.NewsSymbols = []string{"SNT.PL", "CDR.PL"}
	report.News = []model.NewsItem{
		{
			ID: "a", Title: "Synektik wins tender", Description: "A long story about the tender.",
			Published: time.Date(2026, 9, 30, 10, 15, 0, 0, time.UTC), Source: "PAP",
			URL: "https://example.test/a", Symbols: []string{"SNT.PL"},
			RelatedTickers: []string{"SNT.WA"},
		},
		{
			ID: "b", Title: "CD Projekt delays a game", Published: time.Date(2026, 9, 29, 8, 0, 0, 0, time.UTC),
			Source: "Reuters", URL: "https://example.test/b", Symbols: []string{"CDR.PL"},
		},
	}
	return report
}

func TestNewsWithoutArticlesSaysHowToFetch(t *testing.T) {
	report := testReport()
	report.NewsSymbols = []string{"SNT.PL"}

	if view := render(News(report), 120, 10); !strings.Contains(view, "Press r to fetch") ||
		!strings.Contains(view, "SNT.PL") {
		t.Errorf("empty news does not explain itself:\n%s", view)
	}
}

func TestNewsListsArticlesAndOpensThemInAPager(t *testing.T) {
	report := newsReport()
	table := NewsTable(report)
	table.SetSize(160, 12)
	table.SetFocused(true)

	view := table.View()
	for _, want := range []string{"Synektik wins tender", "PAP", "2026-09-30 10:15", "Reuters"} {
		if !strings.Contains(view, want) {
			t.Errorf("news table is missing %q:\n%s", want, view)
		}
	}

	push := table.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))().(ui.PushMsg)
	article, ok := push.Build(report)
	if !ok {
		t.Fatal("article could not be built")
	}
	styled := render(article, 60, 20)
	if !strings.Contains(styled, "\x1b]8;;https://example.test/a") {
		t.Errorf("article link is not a terminal hyperlink:\n%q", styled)
	}
	text := regexp.MustCompile(`\x1b\[[0-9;:]*[a-zA-Z]|\x1b\]8;[^\a]*\a`).ReplaceAllString(styled, "")
	for _, want := range []string{"Synektik wins tender", "PAP · 2026-09-30 10:15 UTC", "A long story",
		"open article ↗ https://example.test/a", "mentions SNT.WA", "esc back"} {
		if !strings.Contains(text, want) {
			t.Errorf("article is missing %q:\n%s", want, text)
		}
	}

	report.News = report.News[1:]
	if _, ok := push.Build(report); ok {
		t.Error("article kept open once it is no longer listed")
	}
}

// copied runs a command and reports the notice it shows, if any.
func copied(t *testing.T, cmd tea.Cmd) string {
	t.Helper()

	if cmd == nil {
		t.Fatal("y returned no command")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("y produced %T, want the clipboard write and a notice", cmd())
	}
	for _, c := range batch {
		if notice, ok := c().(ui.NoticeMsg); ok {
			return string(notice)
		}
	}
	return ""
}

func TestNewsCopiesLinkFromTableAndArticle(t *testing.T) {
	report := newsReport()
	table := NewsTable(report)
	table.SetSize(160, 12)
	table.SetFocused(true)
	y := tea.KeyPressMsg(tea.Key{Text: "y", Code: 'y'})

	if got := copied(t, table.Update(y)); got != copiedNote {
		t.Errorf("table notice = %q, want %q", got, copiedNote)
	}

	article := NewsArticle(report.News[0])
	article.SetSize(80, 20)
	if got := copied(t, article.Update(y)); got != copiedNote {
		t.Errorf("article notice = %q, want %q", got, copiedNote)
	}
	if view := article.View(); !strings.Contains(view, "y copy link") {
		t.Errorf("article footer does not hint at y:\n%s", view)
	}
}
