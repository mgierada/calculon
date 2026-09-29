package portfolio

import (
	"github.com/mgierada/calculon/internal/model"
)

// Scope narrows the input to one account and values it in that account's own
// currency, so its figures read like the broker's statement for it. It reports
// false when the user has no such account.
func Scope(in Input, opts Options, key model.AccountKey) (Input, Options, bool) {
	var account *model.AccountSnapshot
	for i := range in.Accounts {
		if in.Accounts[i].Key() == key {
			account = &in.Accounts[i]
			break
		}
	}
	if account == nil {
		return Input{}, Options{}, false
	}

	scoped := Input{
		User:     in.User,
		Accounts: []model.AccountSnapshot{*account},
		Lots:     ownedBy(in.Lots, key),
		Closed:   ownedBy(in.Closed, key),
		CashOps:  ownedBy(in.CashOps, key),
		Quotes:   in.Quotes,
	}
	opts.FX = NewStaticFX(account.Currency, nil)
	return scoped, opts, true
}

// ownedBy keeps the records of one account.
func ownedBy[T any](records []model.Owned[T], key model.AccountKey) []model.Owned[T] {
	var kept []model.Owned[T]
	for _, record := range records {
		if record.Account.Key() == key {
			kept = append(kept, record)
		}
	}
	return kept
}
