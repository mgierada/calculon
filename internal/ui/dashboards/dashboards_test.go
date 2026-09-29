package dashboards

import (
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
	for _, dashboard := range All() {
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

	detail := render(push.Screen, 160, 40)
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
