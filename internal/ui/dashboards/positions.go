package dashboards

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// holdingIndexKey carries a row's position in report.Holdings, so selecting a
// row can open that holding. No column shows it.
const holdingIndexKey = "_holding"

var positionColumns = []widgets.Column{
	{Key: "symbol", Title: "Symbol", Flex: 2, Left: true},
	{Key: "name", Title: "Name", Flex: 3, Left: true},
	{Key: "account", Title: "Account", Flex: 3, Left: true},
	{Key: "volume", Title: "Volume", Flex: 2},
	{Key: "avg_open", Title: "Avg open", Flex: 2},
	{Key: "price", Title: "Price", Flex: 2},
	{Key: "value", Title: "Value", Flex: 3},
	{Key: "pl", Title: "P/L", Flex: 3},
	{Key: "pl_pct", Title: "P/L %", Flex: 2},
	{Key: "day", Title: "Today", Flex: 2},
	{Key: "weight", Title: "Weight", Flex: 2},
	{Key: "days", Title: "Days", Width: 6},
}

// Positions lists every open holding; enter opens its lots and history.
func Positions(report *portfolio.Report) ui.Component {
	return ui.Single(PositionsTable(report))
}

// PositionsTable renders open holdings, one row per account and symbol.
func PositionsTable(report *portfolio.Report) *widgets.Table {
	rows := make([]widgets.Row, 0, len(report.Holdings))
	for i, h := range report.Holdings {
		ccy := h.Account.Currency
		rows = append(rows, widgets.Row{
			holdingIndexKey: i,
			"symbol":        h.Symbol,
			"name":          h.Name,
			"account":       h.Account.Label(),
			"volume":        volume(h.Volume),
			"avg_open":      price(h.AvgOpenPrice),
			"price":         price(h.Price),
			"value":         money(h.Value, ccy),
			"pl":            deltaCell(h.PL, ""),
			"pl_pct":        deltaPctCell(h.PL),
			"day":           deltaPctCell(h.Day),
			"weight":        percent(h.Weight * 100),
			"days":          strconv.Itoa(daysHeld(h, report)),
		})
	}
	title := fmt.Sprintf("open positions — %d holdings · enter details · / filter", len(rows))
	return widgets.NewTable(title, positionColumns, rows).
		Filterable().
		OnSelect(func(row widgets.Row) tea.Cmd {
			i, ok := row[holdingIndexKey].(int)
			if !ok || i >= len(report.Holdings) {
				return nil
			}
			h := report.Holdings[i]
			return ui.Push(h.Symbol, HoldingDetail(report, h))
		})
}

// daysHeld is whole days since the holding's first lot opened, as of the report.
func daysHeld(h portfolio.Holding, report *portfolio.Report) int {
	if h.FirstOpen.IsZero() || report.AsOf.IsZero() {
		return 0
	}
	return int(report.AsOf.Sub(h.FirstOpen).Hours() / 24)
}
