package portfolio

import (
	"database/sql"

	"github.com/mgierada/calculon/internal/db"
)

// Load reads everything stored for a user and builds their report.
func Load(conn *sql.DB, user db.User, opts Options) (Report, error) {
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
	return Build(in, opts), nil
}
