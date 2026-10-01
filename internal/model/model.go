// Package model holds the provider-agnostic records that parsers produce and
// the database stores. Provider-specific row shapes live in the parser packages
// and convert into these types.
package model

import (
	"fmt"
	"strconv"
	"time"
)

// Provider identifies the broker or data source a record originated from.
type Provider string

// ProviderXTB is the XTB brokerage.
const ProviderXTB Provider = "xtb"

// Side is the direction of a trade.
type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

// CategoryCFD marks derivative positions. Their notional is not owned, so they
// are valued by profit only and left out of holdings.
const CategoryCFD = "CFD"

// CashOpKind is a normalized cash operation category. Providers spell their
// operation types differently, so parsers map their raw labels onto these.
type CashOpKind string

const (
	CashOpDeposit       CashOpKind = "deposit"
	CashOpWithdrawal    CashOpKind = "withdrawal"
	CashOpTransfer      CashOpKind = "transfer"
	CashOpStockPurchase CashOpKind = "stock_purchase"
	CashOpStockSale     CashOpKind = "stock_sale"
	CashOpCloseTrade    CashOpKind = "close_trade"
	CashOpDividend      CashOpKind = "dividend"
	CashOpWithholdTax   CashOpKind = "withholding_tax"
	CashOpInterest      CashOpKind = "interest"
	CashOpInterestTax   CashOpKind = "interest_tax"
	CashOpFee           CashOpKind = "fee"
	CashOpOther         CashOpKind = "other"
)

// IsContribution reports whether the operation moves money into or out of the
// investor's accounts rather than being earned or spent inside them. Transfers
// between two of the investor's own accounts cancel out once both are imported.
func (k CashOpKind) IsContribution() bool {
	return k == CashOpDeposit || k == CashOpWithdrawal || k == CashOpTransfer
}

// Account is one brokerage account. Every record in a statement belongs to one.
type Account struct {
	Provider Provider
	ID       string
	// Currency is the currency the account is denominated in and every amount
	// and price in its records is expressed in.
	Currency string
	// Name is a human label such as "IKE". It is optional and never part of
	// the account's identity.
	Name string
}

// AccountKey is what identifies an account.
type AccountKey struct {
	Provider Provider
	ID       string
}

// Key identifies the account regardless of its label.
func (a Account) Key() AccountKey {
	return AccountKey{Provider: a.Provider, ID: a.ID}
}

// Label is how the account is shown: its name and id, or just the id.
func (a Account) Label() string {
	if a.Name == "" {
		return a.ID
	}
	return a.Name + " " + a.ID
}

// Validate checks the invariants the database and UI rely on.
func (a Account) Validate() error {
	if a.Provider == "" {
		return fmt.Errorf("account %q: provider is empty", a.ID)
	}
	if a.ID == "" {
		return fmt.Errorf("account: id is empty")
	}
	if a.Currency == "" {
		return fmt.Errorf("account %s: currency is empty", a.ID)
	}
	return nil
}

// AccountSnapshot is an account with the time its open lot snapshot was taken,
// zero when none has been imported.
type AccountSnapshot struct {
	Account
	AsOf time.Time
}

// Owned pairs a record with the account it belongs to, for reads that span
// several accounts.
type Owned[T any] struct {
	Account Account
	Record  T
}

// Statement is everything one provider statement file contains, normalized.
type Statement struct {
	Account   Account
	Positions []Position
	CashOps   []CashOp
	// OpenLots is the set of open positions as of AsOf. Unlike the history it
	// is a snapshot: a lot missing from a newer statement has been closed.
	OpenLots []OpenLot
	AsOf     time.Time
}

// Instrument describes what a record trades.
type Instrument struct {
	Symbol   string
	Name     string
	Category string
}

// Position is one closed trade. Partial closes of the same broker position
// share a PositionID, so Seq disambiguates rows that are otherwise identical.
type Position struct {
	Instrument
	PositionID    string
	Seq           int
	Product       string
	Side          Side
	Volume        float64
	OpenTime      time.Time
	OpenPrice     float64
	CloseTime     time.Time
	ClosePrice    float64
	PurchaseValue float64
	SaleValue     float64
	Commission    float64
	Swap          float64
	Rollover      float64
	GrossPL       float64
	NetPL         float64
	CloseOrigin   string
	Comment       string
}

// Key identifies the closed position across re-imports. It is built from the
// fields that never change once a trade has closed; everything else may be
// corrected by the broker and is updated in place.
func (p Position) Key() string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s#%d",
		p.PositionID, p.OpenTime.UTC().Format(time.RFC3339), p.CloseTime.UTC().Format(time.RFC3339),
		formatFloat(p.Volume), formatFloat(p.OpenPrice), formatFloat(p.ClosePrice), p.Seq)
}

// Validate checks the invariants the database and UI rely on.
func (p Position) Validate() error {
	if p.PositionID == "" {
		return fmt.Errorf("position: id is empty")
	}
	if err := validateTrade(p.PositionID, p.Symbol, p.Side, p.Volume, p.OpenTime, p.OpenPrice); err != nil {
		return err
	}
	if p.CloseTime.IsZero() {
		return fmt.Errorf("position %s: close time is missing", p.PositionID)
	}
	if p.CloseTime.Before(p.OpenTime) {
		return fmt.Errorf("position %s: close time %s precedes open time %s",
			p.PositionID, p.CloseTime.Format(time.RFC3339), p.OpenTime.Format(time.RFC3339))
	}
	if p.ClosePrice <= 0 {
		return fmt.Errorf("position %s: close price must be positive, got %v",
			p.PositionID, p.ClosePrice)
	}
	return nil
}

// OpenLot is one still-open position as of a statement's snapshot time.
// CurrentPrice and Value are the broker's valuation at that time.
type OpenLot struct {
	Instrument
	PositionID   string
	Seq          int
	Product      string
	Side         Side
	Volume       float64
	OpenTime     time.Time
	OpenPrice    float64
	CurrentPrice float64
	Value        float64
	GrossPL      float64
	NetPL        float64
	Commission   float64
	Swap         float64
}

// Key identifies the lot within its snapshot.
func (l OpenLot) Key() string {
	return fmt.Sprintf("%s#%d", l.PositionID, l.Seq)
}

// CostBasis is what the lot cost to open.
func (l OpenLot) CostBasis() float64 {
	return l.Volume * l.OpenPrice
}

// Validate checks the invariants the database and UI rely on.
func (l OpenLot) Validate() error {
	if l.PositionID == "" {
		return fmt.Errorf("open lot: id is empty")
	}
	if err := validateTrade(l.PositionID, l.Symbol, l.Side, l.Volume, l.OpenTime, l.OpenPrice); err != nil {
		return err
	}
	if l.CurrentPrice < 0 {
		return fmt.Errorf("open lot %s: current price is negative", l.PositionID)
	}
	return nil
}

// CashOp is a single cash ledger entry. Volume and Price are set only for
// operations that carry them (stock purchases and sales), where they are
// destructured out of the provider's free-text comment. Volume is always
// positive; Kind carries the direction.
type CashOp struct {
	Instrument
	ExternalID string
	Kind       CashOpKind
	RawType    string
	Time       time.Time
	Product    string
	PositionID string
	Comment    string
	Amount     float64
	Volume     float64
	Price      float64
}

// MovesStock reports whether the operation changes the held volume of a symbol.
func (c CashOp) MovesStock() bool {
	return c.Kind == CashOpStockPurchase || c.Kind == CashOpStockSale
}

// Validate checks the invariants the database and UI rely on.
func (c CashOp) Validate() error {
	if c.ExternalID == "" {
		return fmt.Errorf("cash op: id is empty")
	}
	if c.Kind == "" {
		return fmt.Errorf("cash op %s: kind is empty", c.ExternalID)
	}
	if c.Time.IsZero() {
		return fmt.Errorf("cash op %s: time is missing", c.ExternalID)
	}
	if !c.MovesStock() {
		return nil
	}
	if c.Symbol == "" {
		return fmt.Errorf("cash op %s: %s needs a symbol", c.ExternalID, c.Kind)
	}
	if c.Volume <= 0 {
		return fmt.Errorf("cash op %s: %s needs a positive volume, got %v",
			c.ExternalID, c.Kind, c.Volume)
	}
	if c.Price <= 0 {
		return fmt.Errorf("cash op %s: %s needs a positive price, got %v",
			c.ExternalID, c.Kind, c.Price)
	}
	return nil
}

// Quote is one observed price of a symbol, in the currency the symbol trades
// in on its account. Statements contribute one quote per held symbol; a market
// data source can contribute more.
type Quote struct {
	Symbol string
	AsOf   time.Time
	Price  float64
	Source string
	// PrevClose is the previous session's close as the source reported it,
	// zero when it reports none.
	PrevClose float64
}

// validateTrade checks the fields every opened trade carries.
func validateTrade(id, symbol string, side Side, volume float64, openTime time.Time, openPrice float64) error {
	if symbol == "" {
		return fmt.Errorf("position %s: symbol is empty", id)
	}
	if side != SideBuy && side != SideSell {
		return fmt.Errorf("position %s: unknown side %q", id, side)
	}
	if volume <= 0 {
		return fmt.Errorf("position %s: volume must be positive, got %v", id, volume)
	}
	if openTime.IsZero() {
		return fmt.Errorf("position %s: open time is missing", id)
	}
	if openPrice <= 0 {
		return fmt.Errorf("position %s: open price must be positive, got %v", id, openPrice)
	}
	return nil
}

// formatFloat renders a float with the fewest digits that round-trip.
func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
