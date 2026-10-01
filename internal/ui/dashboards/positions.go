package dashboards

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// holdingIDKey identifies a row's holding by account and symbol, which stays
// the same across reports, so selecting a row can open that holding and a
// reload keeps the cursor on it. No column shows it.
const holdingIDKey = "_holding"

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
	for _, h := range report.Holdings {
		ccy := h.Account.Currency
		rows = append(rows, widgets.Row{
			holdingIDKey: holdingID(h),
			"symbol":     h.Symbol,
			"name":       h.Name,
			"account":    h.Account.Label(),
			"volume":     volume(h.Volume),
			"avg_open":   price(h.AvgOpenPrice),
			"price":      price(h.Price),
			"value":      money(h.Value, ccy),
			"pl":         deltaCell(h.PL, ""),
			"pl_pct":     deltaPctCell(h.PL),
			"day":        dayCell(h),
			"weight":     percent(h.Weight * 100),
			"days":       strconv.Itoa(daysHeld(h, report)),
		})
		addSortValues(rows[len(rows)-1], h, report)
	}
	title := fmt.Sprintf("open positions — %d holdings · enter details · / filter · s sort", len(rows))
	return widgets.NewTable(title, positionColumns, rows).
		Filterable().
		// Biggest winners first; s and S re-sort from there.
		SortedBy("pl", true).
		KeyedBy(holdingIDKey).
		OnSelect(func(row widgets.Row) tea.Cmd {
			id, ok := row[holdingIDKey].(string)
			if !ok {
				return nil
			}
			h, ok := findHolding(report, id)
			if !ok {
				return nil
			}
			return ui.Push(h.Symbol, func(report *portfolio.Report) (ui.Component, bool) {
				h, ok := findHolding(report, id)
				if !ok {
					return nil, false
				}
				return HoldingDetail(report, h), true
			})
		})
}

// holdingID identifies a holding across reports.
func holdingID(h portfolio.Holding) string {
	key := h.Account.Key()
	return fmt.Sprintf("%s/%s/%s", key.Provider, key.ID, h.Symbol)
}

// findHolding looks a holding up by its holdingID.
func findHolding(report *portfolio.Report, id string) (portfolio.Holding, bool) {
	for _, h := range report.Holdings {
		if holdingID(h) == id {
			return h, true
		}
	}
	return portfolio.Holding{}, false
}

// addSortValues gives each numeric column its raw value to sort by. Money is
// compared in the base currency, so holdings in different accounts rank by what
// they are worth, not by the size of the number in their own currency.
func addSortValues(row widgets.Row, h portfolio.Holding, report *portfolio.Report) {
	toBase := 0.0
	if h.Value != 0 {
		toBase = h.ValueBase / h.Value
	}
	values := map[string]any{
		"volume":   h.Volume,
		"avg_open": h.AvgOpenPrice,
		"price":    h.Price,
		"value":    h.ValueBase,
		"pl":       h.PL.Amount * toBase,
		"pl_pct":   h.PL.Pct,
		"day":      dayOrNil(h),
		"weight":   h.Weight,
		"days":     float64(daysHeld(h, report)),
	}
	for column, value := range values {
		row[widgets.SortBy(column)] = value
	}
}

// knownOrNil is a change's percentage, or nil so an unknown one sorts last.
func knownOrNil(d portfolio.Delta) any {
	if !d.Known {
		return nil
	}
	return d.Pct
}

// dayOrNil is a holding's day change percentage, or nil so one from an
// earlier session sorts last with the unknown ones instead of among today's.
func dayOrNil(h portfolio.Holding) any {
	if h.DayStale {
		return nil
	}
	return knownOrNil(h.Day)
}

// daysHeld is whole days since the holding's first lot opened, as of the report.
func daysHeld(h portfolio.Holding, report *portfolio.Report) int {
	if h.FirstOpen.IsZero() || report.AsOf.IsZero() {
		return 0
	}
	return int(report.AsOf.Sub(h.FirstOpen).Hours() / 24)
}
