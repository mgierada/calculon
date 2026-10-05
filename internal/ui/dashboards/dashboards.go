// Package dashboards holds calculon's dashboards. Each one is a function from a
// portfolio report to a ui.Grid of widgets, and the pieces they share (the
// value chart, the allocation chart, KPI rows) are plain constructors, so a new
// dashboard is mostly a new arrangement of existing parts.
package dashboards

import (
	"github.com/mgierada/calculon/internal/ui"
)

// Refreshers fetch the data of the dashboards that call an API, run when r
// is pressed on their tab; a nil one leaves r a plain reload there.
type Refreshers struct {
	News     ui.Refresher
	Earnings ui.Refresher
}

// All lists the dashboards in tab order.
func All(refresh Refreshers) []ui.Dashboard {
	return []ui.Dashboard{
		{Title: "Overview", Build: Overview},
		{Title: "Positions", Build: Positions},
		{Title: "History", Build: History},
		{Title: "News", Build: News, Refresh: refresh.News},
		{Title: "Earnings", Build: Earnings, Refresh: refresh.Earnings},
	}
}
