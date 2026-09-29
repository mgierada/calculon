package dashboards

import (
	"fmt"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

var lotColumns = []widgets.Column{
	{Key: "position", Title: "Position", Flex: 2, Left: true},
	{Key: "opened", Title: "Opened", Flex: 3, Left: true},
	{Key: "volume", Title: "Volume", Flex: 2},
	{Key: "open", Title: "Open", Flex: 2},
	{Key: "price", Title: "Price", Flex: 2},
	{Key: "value", Title: "Value", Flex: 3},
	{Key: "pl", Title: "P/L", Flex: 3},
	{Key: "pl_pct", Title: "P/L %", Flex: 2},
}

// HoldingDetail drills into one holding: its figures, the lots it is made of,
// and every past trade and cash movement in that symbol and account.
func HoldingDetail(report *portfolio.Report, h portfolio.Holding) ui.Component {
	return &ui.Grid{Rows: []ui.Row{
		{Height: statRowHeight, Cells: ui.Cells(holdingStats(h)...)},
		{Weight: 1, Cells: ui.Cells(lotsTable(h))},
		{Weight: 1, Cells: ui.Cells(
			ClosedTable("closed "+h.Symbol, closedFor(report.Closed, h)),
			CashTable(h.Symbol+" cash", cashFor(report.CashOps, h)),
		)},
	}}
}

func holdingStats(h portfolio.Holding) []ui.Component {
	ccy := h.Account.Currency
	return []ui.Component{
		widgets.NewStat(h.Symbol, h.Name).
			WithNote(fmt.Sprintf("%s · %s %s", h.Category, h.Account.Provider, h.Account.ID), widgets.Muted),
		widgets.NewStat("Value", money(h.Value, ccy)).
			WithNote(percent(h.Weight*100)+" of portfolio", widgets.Muted),
		widgets.NewStat("Volume", volume(h.Volume)).
			WithNote(fmt.Sprintf("%d lots", len(h.Lots)), widgets.Muted),
		widgets.NewStat("Price", price(h.Price)).
			WithNote("avg open "+price(h.AvgOpenPrice), widgets.Muted),
		deltaStat("P/L", h.PL, ccy, ""),
		deltaStat("Today", h.Day, ccy, dayUnknownNote),
	}
}

func lotsTable(h portfolio.Holding) *widgets.Table {
	rows := make([]widgets.Row, 0, len(h.Lots))
	for _, lot := range h.Lots {
		value := lot.Volume * h.Price
		pl := portfolio.Delta{Amount: value - lot.CostBasis(), Known: true}
		if cost := lot.CostBasis(); cost != 0 {
			pl.Pct = pl.Amount / cost * 100
		}
		rows = append(rows, widgets.Row{
			"position": lot.PositionID,
			"opened":   dateTime(lot.OpenTime),
			"volume":   volume(lot.Volume),
			"open":     price(lot.OpenPrice),
			"price":    price(h.Price),
			"value":    money(value, ""),
			"pl":       deltaCell(pl, ""),
			"pl_pct":   deltaPctCell(pl),
		})
	}
	return widgets.NewTable(fmt.Sprintf("open lots — %d", len(rows)), lotColumns, rows)
}

func closedFor(closed []model.Owned[model.Position], h portfolio.Holding) []model.Owned[model.Position] {
	var matching []model.Owned[model.Position]
	for _, p := range closed {
		if p.Account == h.Account && p.Record.Symbol == h.Symbol {
			matching = append(matching, p)
		}
	}
	return matching
}

func cashFor(ops []model.Owned[model.CashOp], h portfolio.Holding) []model.Owned[model.CashOp] {
	var matching []model.Owned[model.CashOp]
	for _, op := range ops {
		if op.Account == h.Account && op.Record.Symbol == h.Symbol {
			matching = append(matching, op)
		}
	}
	return matching
}
