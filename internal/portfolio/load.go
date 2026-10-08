package portfolio

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/model"
)

// Load reads everything stored for a user and builds their report: the summary
// of all accounts when scope is nil, otherwise just that account.
func Load(conn *sql.DB, user db.User, opts Options, scope *model.AccountKey) (Report, error) {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	in := Input{User: user.Name}
	var err error
	if in.Accounts, err = db.Accounts(conn, user.ID); err != nil {
		return Report{}, err
	}
	if in.Lots, err = db.OpenLots(conn, user.ID); err != nil {
		return Report{}, err
	}
	if in.Closed, err = db.ClosedPositions(conn, user.ID); err != nil {
		return Report{}, err
	}
	if in.CashOps, err = db.CashOps(conn, user.ID); err != nil {
		return Report{}, err
	}
	if in.Quotes, err = db.Quotes(conn, user.ID); err != nil {
		return Report{}, err
	}
	if scope == nil {
		report := Build(in, opts)
		report.AccountReturns = AccountReturns(in, opts)
		return withNews(conn, report, opts)
	}

	all := in.Accounts
	scoped, scopedOpts, ok := Scope(in, opts, *scope)
	if !ok {
		return Report{}, fmt.Errorf("no %s account %s", scope.Provider, scope.ID)
	}
	report := Build(scoped, scopedOpts)
	report.Scope = &scoped.Accounts[0].Account
	report.Available = all
	report.FXNote = "values in " + report.Base + ", the account's currency"
	return withNews(conn, report, scopedOpts)
}

// withNews attaches the stored news about the report's biggest holdings, and
// the stored earnings and analyst recommendations of every holding.
func withNews(conn *sql.DB, report Report, opts Options) (Report, error) {
	report.NewsSymbols = TopSymbols(report.Holdings, opts.News.Positions)
	var err error
	if report.News, err = db.News(conn, report.NewsSymbols, opts.News.PerSymbol); err != nil {
		return report, err
	}
	report.Earnings = map[string]model.Earnings{}
	report.Recommendations = map[string]model.Recommendations{}
	for _, symbol := range TopSymbols(report.Holdings, len(report.Holdings)) {
		earnings, ok, err := db.EarningsOf(conn, symbol, opts.EarningsMethodology)
		if err != nil {
			return report, err
		}
		if ok {
			report.Earnings[symbol] = earnings
		}
		recs, ok, err := db.RecommendationsOf(conn, symbol)
		if err != nil {
			return report, err
		}
		if ok {
			report.Recommendations[symbol] = recs
		}
	}
	return report, nil
}
