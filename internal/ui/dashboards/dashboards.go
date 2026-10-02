// Package dashboards holds calculon's dashboards. Each one is a function from a
// portfolio report to a ui.Grid of widgets, and the pieces they share (the
// value chart, the allocation chart, KPI rows) are plain constructors, so a new
// dashboard is mostly a new arrangement of existing parts.
package dashboards

import (
	"github.com/mgierada/calculon/internal/ui"
)

// All lists the dashboards in tab order. fetchNews runs when r is pressed on
// the News tab; nil leaves r a plain reload there.
func All(fetchNews ui.Refresher) []ui.Dashboard {
	return []ui.Dashboard{
		{Title: "Overview", Build: Overview},
		{Title: "Positions", Build: Positions},
		{Title: "History", Build: History},
		{Title: "News", Build: News, Refresh: fetchNews},
	}
}
