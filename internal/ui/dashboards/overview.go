package dashboards

import (
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// Fixed heights of the bands whose content does not grow with the screen.
const (
	statRowHeight  = 5
	stackBarHeight = 4
	// tableChrome is a table's borders, header and footer.
	tableChrome = 6
)

// dayUnknownNote explains a missing day change.
const dayUnknownNote = "needs a previous close"

var accountColumns = []widgets.Column{
	{Key: "account", Title: "Account", Flex: 2, Left: true},
	{Key: "currency", Title: "Ccy", Width: 5, Left: true},
	{Key: "positions", Title: "Positions", Flex: 3},
	{Key: "cash", Title: "Cash", Flex: 3},
	{Key: "total", Title: "Total", Flex: 3},
	{Key: "total_base", Title: "Total (base)", Flex: 3},
	{Key: "as_of", Title: "Snapshot", Flex: 3},
}

// Overview is the landing dashboard: headline figures, where the money sits,
// and how the portfolio got here.
func Overview(report *portfolio.Report) ui.Component {
	return &ui.Grid{Rows: []ui.Row{
		{Height: statRowHeight, Cells: ui.Cells(TotalsStats(report)...)},
		{Height: stackBarHeight, Cells: ui.Cells(AllocationBar(report))},
		{Weight: 1, Cells: []ui.Cell{
			{Component: NewAllocationChart(report), Weight: 2},
			{Component: ValueChart(report), Weight: 3},
		}},
		{Height: len(report.Accounts) + tableChrome, Cells: ui.Cells(AccountsTable(report))},
	}}
}

// TotalsStats are the portfolio-wide KPI cards.
func TotalsStats(report *portfolio.Report) []ui.Component {
	t, base := report.Totals, report.Base
	return []ui.Component{
		widgets.NewStat("Total value", money(t.Total, base)).
			WithNote("net deposits "+money(t.Contributions, base), widgets.Muted),
		widgets.NewStat("Positions", money(t.PositionsValue, base)).
			WithNote("cost "+money(t.CostBasis, base), widgets.Muted),
		widgets.NewStat("Cash", money(t.Cash, base)),
		deltaStat("Unrealized P/L", t.Unrealized, base, ""),
		deltaStat("Today", t.Day, base, dayUnknownNote),
		widgets.NewStat("Realized P/L", signedMoney(t.RealizedPL, base)).
			WithNote("dividends "+money(t.Dividends, base), widgets.Muted),
		deltaStat("Total P/L", t.TotalPL, base, ""),
	}
}

// AccountsTable totals each account in its own currency and in the base one.
func AccountsTable(report *portfolio.Report) *widgets.Table {
	rows := make([]widgets.Row, 0, len(report.Accounts))
	for _, a := range report.Accounts {
		rows = append(rows, widgets.Row{
			"account":    string(a.Provider) + " " + a.Label(),
			"currency":   a.Currency,
			"positions":  money(a.PositionsValue, ""),
			"cash":       money(a.Cash, ""),
			"total":      money(a.Total, ""),
			"total_base": money(a.TotalBase, report.Base),
			"as_of":      dateTime(a.AsOf),
		})
	}
	return widgets.NewTable("accounts", accountColumns, rows)
}

// ValueChart plots the portfolio's worth over time next to the money put in.
func ValueChart(report *portfolio.Report) *widgets.LineChart {
	value := widgets.Series{Name: "value"}
	contributions := widgets.Series{Name: "net deposits"}
	for _, p := range report.History {
		value.Points = append(value.Points, widgets.Point{Time: p.Time, Value: p.Value})
		contributions.Points = append(contributions.Points,
			widgets.Point{Time: p.Time, Value: p.Contributions})
	}
	return widgets.NewLineChart("value over time ("+report.Base+")", value, contributions)
}
