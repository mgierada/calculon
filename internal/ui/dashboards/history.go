package dashboards

import (
	"fmt"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

var closedColumns = []widgets.Column{
	{Key: "closed", Title: "Closed", Width: 12, Left: true},
	{Key: "symbol", Title: "Symbol", Flex: 2, Left: true},
	{Key: "account", Title: "Account", Flex: 2, Left: true},
	{Key: "volume", Title: "Volume", Flex: 2},
	{Key: "open", Title: "Open", Flex: 2},
	{Key: "close", Title: "Close", Flex: 2},
	{Key: "pl", Title: "Net P/L", Flex: 3},
	{Key: "origin", Title: "Origin", Flex: 2, Left: true},
}

var cashColumns = []widgets.Column{
	{Key: "time", Title: "Time", Width: 12, Left: true},
	{Key: "account", Title: "Account", Flex: 2, Left: true},
	{Key: "kind", Title: "Kind", Flex: 3, Left: true},
	{Key: "symbol", Title: "Symbol", Flex: 2, Left: true},
	{Key: "amount", Title: "Amount", Flex: 3},
	{Key: "comment", Title: "Comment", Flex: 5, Left: true},
}

// History shows how the portfolio got here: its value over time, every closed
// trade and every cash movement, each table filterable with /.
func History(report *portfolio.Report) ui.Component {
	return &ui.Grid{Rows: []ui.Row{
		{Weight: 2, Cells: ui.Cells(ValueChart(report))},
		{Weight: 3, Cells: ui.Cells(
			ClosedTable("closed positions", report.Closed),
			CashTable("cash operations", report.CashOps),
		)},
	}}
}

// ClosedTable lists closed trades, most recent first.
func ClosedTable(title string, closed []model.Owned[model.Position]) *widgets.Table {
	rows := make([]widgets.Row, 0, len(closed))
	for _, owned := range closed {
		p := owned.Record
		rows = append(rows, widgets.Row{
			"closed":  date(p.CloseTime),
			"symbol":  p.Symbol,
			"account": owned.Account.Label(),
			"volume":  volume(p.Volume),
			"open":    price(p.OpenPrice),
			"close":   price(p.ClosePrice),
			"pl":      widgets.Toned(signedMoney(p.NetPL, owned.Account.Currency), widgets.ToneOf(p.NetPL)),
			"origin":  p.CloseOrigin,
		})
	}
	return widgets.NewTable(fmt.Sprintf("%s — %d · / filter", title, len(rows)), closedColumns, rows).
		Filterable()
}

// CashTable lists cash movements, newest first.
func CashTable(title string, ops []model.Owned[model.CashOp]) *widgets.Table {
	rows := make([]widgets.Row, 0, len(ops))
	for _, owned := range ops {
		op := owned.Record
		rows = append(rows, widgets.Row{
			"time":    date(op.Time),
			"account": owned.Account.Label(),
			"kind":    op.RawType,
			"symbol":  op.Symbol,
			"amount":  widgets.Toned(signedMoney(op.Amount, owned.Account.Currency), widgets.ToneOf(op.Amount)),
			"comment": op.Comment,
		})
	}
	return widgets.NewTable(fmt.Sprintf("%s — %d · / filter", title, len(rows)), cashColumns, rows).
		Filterable()
}
