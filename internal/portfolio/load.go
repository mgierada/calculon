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
		return Build(in, opts), nil
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
	return report, nil
}
