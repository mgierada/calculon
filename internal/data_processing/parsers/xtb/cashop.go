package xtb

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mgierada/calculon/internal/model"
)

// cashOpHeader are the labels that identify the cash operation table.
var cashOpHeader = []string{"Type", "Ticker", "Time", "Amount", "ID"}

// cashOpKinds maps XTB's lowercased operation labels onto normalized kinds.
// Anything unmapped becomes model.CashOpOther, keeping its label in RawType.
var cashOpKinds = map[string]model.CashOpKind{
	"deposit":                 model.CashOpDeposit,
	"withdrawal":              model.CashOpWithdrawal,
	"transfer":                model.CashOpTransfer,
	"subaccount transfer":     model.CashOpTransfer,
	"ike deposit":             model.CashOpTransfer,
	"stock purchase":          model.CashOpStockPurchase,
	"stock sell":              model.CashOpStockSale,
	"stock sale":              model.CashOpStockSale,
	"close trade":             model.CashOpCloseTrade,
	"dividend":                model.CashOpDividend,
	"divident":                model.CashOpDividend, // XTB's older spelling
	"withholding tax":         model.CashOpWithholdTax,
	"free funds interest":     model.CashOpInterest,
	"free funds interest tax": model.CashOpInterestTax,
	"sec fee":                 model.CashOpFee,
	"swap":                    model.CashOpFee,
	"rollover":                model.CashOpFee,
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

// parseCashOps reads the cash operation sheet into validated canonical records.
// It returns nil when the sheet holds no cash operation table.
func parseCashOps(rows [][]string) ([]model.CashOp, error) {
	data, cols, ok := dataRows(rows, cashOpHeader)
	if !ok {
		return nil, nil
	}

	cashOps := make([]model.CashOp, 0, len(data))
	for _, row := range data {
		op, err := parseCashOpRow(row, cols)
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

// parseCashOpRow maps one sheet row onto a cash operation, destructuring volume
// and price out of the comment for stock trades.
func parseCashOpRow(row []string, cols columns) (model.CashOp, error) {
	p := fieldParser{row: row, cols: cols}
	op := model.CashOp{
		Instrument: model.Instrument{
			Symbol:   p.text("Ticker"),
			Name:     p.text("Instrument"),
			Category: p.text("Category"),
		},
		ExternalID: p.text("ID"),
		RawType:    p.text("Type"),
		Time:       p.time("Time"),
		Product:    p.text("Product"),
		PositionID: p.text("Position ID"),
		Comment:    p.text("Comment"),
		Amount:     p.float("Amount"),
	}
	if p.err != nil {
		return model.CashOp{}, fmt.Errorf("cash op %s: field %q: %w", op.ExternalID, p.field, p.err)
	}
	op.Kind = cashOpKind(op.RawType)

	if !op.MovesStock() {
		return op, nil
	}
	volume, price, err := parseStockTradeComment(op.Comment)
	if err != nil {
		return model.CashOp{}, fmt.Errorf("cash op %s: %w", op.ExternalID, err)
	}
	op.Volume, op.Price = volume, price
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
