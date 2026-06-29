// Package model holds the provider-agnostic records that parsers produce and
// the database stores. Provider-specific row shapes live in the parser packages
// and convert into these types.
package model

import (
	"fmt"
	"math"
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

// CashOpKind is a normalized cash operation category. Providers spell their
// operation types differently, so parsers map their raw labels onto these.
type CashOpKind string

const (
	CashOpDeposit       CashOpKind = "deposit"
	CashOpWithdrawal    CashOpKind = "withdrawal"
	CashOpStockPurchase CashOpKind = "stock_purchase"
	CashOpStockSale     CashOpKind = "stock_sale"
	CashOpCloseTrade    CashOpKind = "close_trade"
	CashOpDividend      CashOpKind = "dividend"
	CashOpWithholdTax   CashOpKind = "withholding_tax"
	CashOpInterest      CashOpKind = "interest"
	CashOpInterestTax   CashOpKind = "interest_tax"
	CashOpOther         CashOpKind = "other"
)

// Statement is everything one provider statement file contains, normalized.
type Statement struct {
	Provider  Provider
	AccountID string
	Positions []Position
	CashOps   []CashOp
}

// Position is a single trade position. CloseTime and ClosePrice are zero while
// the position is still open.
type Position struct {
	Provider      Provider
	AccountID     string
	ExternalID    string
	Symbol        string
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
	Comment       string
}

// IsOpen reports whether the position has not been closed yet.
func (p Position) IsOpen() bool {
	return p.CloseTime.IsZero()
}

// Validate checks the invariants the database and UI rely on.
func (p Position) Validate() error {
	if err := validateIdentity(p.Provider, p.AccountID, p.ExternalID); err != nil {
		return err
	}
	if p.Symbol == "" {
		return fmt.Errorf("position %s: symbol is empty", p.ExternalID)
	}
	if p.Side != SideBuy && p.Side != SideSell {
		return fmt.Errorf("position %s: unknown side %q", p.ExternalID, p.Side)
	}
	if p.Volume <= 0 {
		return fmt.Errorf("position %s: volume must be positive, got %v", p.ExternalID, p.Volume)
	}
	if p.OpenTime.IsZero() {
		return fmt.Errorf("position %s: open time is missing", p.ExternalID)
	}
	if p.OpenPrice <= 0 {
		return fmt.Errorf("position %s: open price must be positive, got %v", p.ExternalID, p.OpenPrice)
	}
	if p.IsOpen() {
		return nil
	}
	if p.CloseTime.Before(p.OpenTime) {
		return fmt.Errorf("position %s: close time %s precedes open time %s",
			p.ExternalID, p.CloseTime.Format(time.RFC3339), p.OpenTime.Format(time.RFC3339))
	}
	if p.ClosePrice <= 0 {
		return fmt.Errorf("position %s: closed position needs a close price", p.ExternalID)
	}
	return nil
}

// CashOp is a single cash ledger entry. Volume and Price are set only for
// operations that carry them (stock purchases and sales), where they are
// destructured out of the provider's free-text comment. Volume is always
// positive; Kind carries the direction.
type CashOp struct {
	Provider   Provider
	AccountID  string
	ExternalID string
	Kind       CashOpKind
	RawType    string
	Time       time.Time
	Comment    string
	Symbol     string
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
	if err := validateIdentity(c.Provider, c.AccountID, c.ExternalID); err != nil {
		return err
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

// Holding is the aggregated open exposure to one symbol.
type Holding struct {
	Symbol string
	// Volume is purchased volume minus sold volume.
	Volume float64
	// AvgOpenPrice is the volume-weighted average price of all purchases,
	// i.e. average cost basis rather than FIFO lot cost.
	AvgOpenPrice float64
	// FirstOpen is the timestamp of the earliest purchase.
	FirstOpen time.Time
}

// DaysHeld is whole days between the first purchase and now.
func (h Holding) DaysHeld(now time.Time) int {
	if h.FirstOpen.IsZero() {
		return 0
	}
	return int(math.Floor(now.Sub(h.FirstOpen).Hours() / 24))
}

// validateIdentity checks the fields that make up a record's natural key.
func validateIdentity(provider Provider, accountID, externalID string) error {
	if provider == "" {
		return fmt.Errorf("record %q: provider is empty", externalID)
	}
	if accountID == "" {
		return fmt.Errorf("record %q: account id is empty", externalID)
	}
	if externalID == "" {
		return fmt.Errorf("record: external id is empty")
	}
	return nil
}
