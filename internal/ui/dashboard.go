package ui

import (
	"context"

	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/portfolio"
)

// Dashboard is one tab of the app. Build turns a report into a fresh component
// tree; it runs again whenever the report is reloaded, so dashboards hold no
// data of their own and adding one is a matter of writing a Build function.
type Dashboard struct {
	Title string
	Build func(report *portfolio.Report) Component
	// Refresh, when set, fetches the dashboard's data from outside before r
	// reloads it, e.g. calling an API only when the user asks for it.
	Refresh Refresher
}

// Refresher fetches data off the UI loop, reporting progress as it goes, and
// returns a summary of what it did for the footer. It stops when ctx is
// cancelled. target is what the dashboard shows, e.g. a chosen symbol, empty
// for dashboards without a choice; see RefreshTargeter.
type Refresher func(ctx context.Context, report *portfolio.Report, target string,
	progress func(Progress)) (string, error)

// RefreshTargeter is implemented by a component whose dashboard refreshes
// one thing the user chose, like an earnings view's symbol.
type RefreshTargeter interface {
	RefreshTarget() string
}

// Progress is how far a refresh has got: Current is the item being worked
// on, empty once all are done; Finished is the item just completed, with
// Detail saying how it went and Failed whether it did not.
type Progress struct {
	Done, Total int
	Current     string
	Finished    string
	Detail      string
	Failed      bool
}

// Loader produces the report the dashboards render: the summary of every
// account when scope is nil, otherwise that one account. It runs off the UI
// loop.
type Loader func(scope *model.AccountKey) (portfolio.Report, error)
