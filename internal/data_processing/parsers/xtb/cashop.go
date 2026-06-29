package xtb

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// cashOpHeader are the labels that identify the cash operation table.
var cashOpHeader = []string{"ID", "Type", "Time", "Amount"}

// cashOpKinds maps XTB's lowercased operation labels onto normalized kinds.
// Anything unmapped becomes model.CashOpOther, keeping its label in RawType.
var cashOpKinds = map[string]model.CashOpKind{
	"deposit":                 model.CashOpDeposit,
	"withdrawal":              model.CashOpWithdrawal,
	"stock purchase":          model.CashOpStockPurchase,
	"stock sale":              model.CashOpStockSale,
	"close trade":             model.CashOpCloseTrade,
	"divident":                model.CashOpDividend, // XTB's own spelling
	"dividend":                model.CashOpDividend,
	"withholding tax":         model.CashOpWithholdTax,
	"free-funds interest":     model.CashOpInterest,
	"free-funds interest tax": model.CashOpInterestTax,
}

// stockTradeComment matches the comment XTB attaches to stock purchases and
// sales, which is where the traded volume and price live:
//
//	OPEN BUY 4 @ 273.60             whole order filled at once
//	OPEN BUY 1/2 @ 206.00           this fill is 1 of a 2 share order
//	CLOSE BUY 25/70 @ 29.600        partial close of a 70 share position
//
// The leading number is this row's volume; the number after the slash is the
// order it belongs to and is not needed once rows are summed per symbol.
var stockTradeComment = regexp.MustCompile(
	`^(?:OPEN|CLOSE)\s+(?:BUY|SELL)\s+([0-9]*\.?[0-9]+)(?:/[0-9]*\.?[0-9]+)?\s*@\s*([0-9]*\.?[0-9]+)$`,
)

// XTBCashOp is one row of the XTB cash operation sheet, in XTB's own shape.
type XTBCashOp struct {
	ID      string
	Type    string
	Time    time.Time
	Comment string
	Symbol  string
	Amount  float64
}

// ToCashOp converts the XTB row into the canonical cash operation record,
// destructuring volume and price out of the comment for stock trades.
func (c XTBCashOp) ToCashOp(accountID string) (model.CashOp, error) {
	op := model.CashOp{
		Provider:   model.ProviderXTB,
		AccountID:  accountID,
		ExternalID: c.ID,
		Kind:       cashOpKind(c.Type),
		RawType:    c.Type,
		Time:       c.Time,
		Comment:    c.Comment,
		Symbol:     c.Symbol,
		Amount:     c.Amount,
	}

	if !op.MovesStock() {
		return op, nil
	}

	volume, price, err := parseStockTradeComment(c.Comment)
	if err != nil {
		return model.CashOp{}, fmt.Errorf("cash op %s: %w", c.ID, err)
	}
	op.Volume = volume
	op.Price = price

	return op, nil
}

// parseCashOps reads the cash operation sheet into validated canonical records.
// It returns nil when the sheet holds no cash operation table.
func parseCashOps(rows [][]string, accountID string) ([]model.CashOp, error) {
	data, cols, ok := dataRows(rows, cashOpHeader)
	if !ok {
		return nil, nil
	}

	cashOps := make([]model.CashOp, 0, len(data))
	for _, row := range data {
		raw, err := parseCashOpRow(row, cols)
		if err != nil {
			return nil, err
		}
		op, err := raw.ToCashOp(accountID)
		if err != nil {
			return nil, err
		}
		if err := op.Validate(); err != nil {
			return nil, err
		}
		cashOps = append(cashOps, op)
	}

	return cashOps, nil
}

// parseCashOpRow maps one sheet row onto an XTBCashOp.
func parseCashOpRow(row []string, cols columns) (XTBCashOp, error) {
	op := XTBCashOp{
		ID:      cols.get(row, "ID"),
		Type:    cols.get(row, "Type"),
		Comment: cols.get(row, "Comment"),
		Symbol:  cols.get(row, "Symbol"),
	}

	amount, err := parseFloat(cols.get(row, "Amount"))
	if err != nil {
		return XTBCashOp{}, fmt.Errorf("cash op %s: field %q: %w", op.ID, "Amount", err)
	}
	op.Amount = amount

	opTime, err := parseTime(cols.get(row, "Time"))
	if err != nil {
		return XTBCashOp{}, fmt.Errorf("cash op %s: field %q: %w", op.ID, "Time", err)
	}
	op.Time = opTime

	return op, nil
}

// cashOpKind normalizes an XTB operation label.
func cashOpKind(rawType string) model.CashOpKind {
	if kind, ok := cashOpKinds[strings.ToLower(strings.TrimSpace(rawType))]; ok {
		return kind
	}
	return model.CashOpOther
}

// parseStockTradeComment destructures the volume and price of a stock trade out
// of its comment.
func parseStockTradeComment(comment string) (volume, price float64, err error) {
	match := stockTradeComment.FindStringSubmatch(strings.TrimSpace(comment))
	if match == nil {
		return 0, 0, fmt.Errorf("unrecognized stock trade comment %q", comment)
	}

	if volume, err = parseFloat(match[1]); err != nil {
		return 0, 0, fmt.Errorf("stock trade comment %q: volume: %w", comment, err)
	}
	if price, err = parseFloat(match[2]); err != nil {
		return 0, 0, fmt.Errorf("stock trade comment %q: price: %w", comment, err)
	}

	return volume, price, nil
}
