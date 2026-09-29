package ui

import (
	"github.com/mgierada/calculon/internal/portfolio"
)

// Dashboard is one tab of the app. Build turns a report into a fresh component
// tree; it runs again whenever the report is reloaded, so dashboards hold no
// data of their own and adding one is a matter of writing a Build function.
type Dashboard struct {
	Title string
	Build func(report *portfolio.Report) Component
}

// Loader produces the report the dashboards render. It runs off the UI loop.
type Loader func() (portfolio.Report, error)
