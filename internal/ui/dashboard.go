package ui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/ui/widgets"
)

// portfolioColumns is the open-holdings table layout.
var portfolioColumns = []widgets.Column{
	{Key: "symbol", Title: "Symbol", Flex: 2},
	{Key: "volume", Title: "Volume", Flex: 2},
	{Key: "avg_open", Title: "Avg Open Price", Flex: 3},
	{Key: "days", Title: "Days Held", Flex: 2},
}

// PortfolioGrid lays out the dashboard. Right now that is one full-screen table
// of open holdings; adding a chart beside it is a matter of adding a cell here.
func PortfolioGrid(holdings []model.Holding, now time.Time) Grid {
	return Single(PortfolioTable(holdings, now))
}

// PortfolioTable renders open holdings, one row per symbol.
func PortfolioTable(holdings []model.Holding, now time.Time) *widgets.Table {
	rows := make([]widgets.Row, 0, len(holdings))
	for _, holding := range holdings {
		rows = append(rows, widgets.Row{
			"symbol":   holding.Symbol,
			"volume":   formatVolume(holding.Volume),
			"avg_open": fmt.Sprintf("%.2f", holding.AvgOpenPrice),
			"days":     holding.DaysHeld(now),
		})
	}

	title := fmt.Sprintf("open positions by symbol — %d symbols", len(holdings))
	return widgets.NewTable(title, portfolioColumns, rows)
}

// formatVolume prints share counts without trailing zeros, since fractional
// share purchases produce volumes like 1.6757 alongside whole ones like 10.
func formatVolume(volume float64) string {
	return strconv.FormatFloat(volume, 'f', -1, 64)
}
