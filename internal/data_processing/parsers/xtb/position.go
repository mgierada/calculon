package xtb

import (
	"fmt"
	"strings"
	"time"

	"github.com/mgierada/calculon/internal/model"
)

// positionHeader are the labels that identify a position table. Both the closed
// and the open sheet carry them; the columns they differ on (close time and
// price versus market price) are read opportunistically.
var positionHeader = []string{"Position", "Symbol", "Volume"}

// XTBPosition is one row of an XTB position sheet, in XTB's own shape. Fields
// absent from a given sheet stay zero.
type XTBPosition struct {
	Position      string
	Symbol        string
	Type          string
	Volume        float64
	OpenTime      time.Time
	OpenPrice     float64
	CloseTime     time.Time
	ClosePrice    float64
	MarketPrice   float64
	OpenOrigin    string
	CloseOrigin   string
	PurchaseValue float64
	SaleValue     float64
	SL            float64
	TP            float64
	Margin        float64
	Commission    float64
	Swap          float64
	Rollover      float64
	GrossPL       float64
	Comment       string
}

// ToPosition converts the XTB row into the canonical position record.
func (p XTBPosition) ToPosition(accountID string) model.Position {
	return model.Position{
		Provider:      model.ProviderXTB,
		AccountID:     accountID,
		ExternalID:    p.Position,
		Symbol:        p.Symbol,
		Side:          model.Side(strings.ToUpper(p.Type)),
		Volume:        p.Volume,
		OpenTime:      p.OpenTime,
		OpenPrice:     p.OpenPrice,
		CloseTime:     p.CloseTime,
		ClosePrice:    p.ClosePrice,
		PurchaseValue: p.PurchaseValue,
		SaleValue:     p.SaleValue,
		Commission:    p.Commission,
		Swap:          p.Swap,
		Rollover:      p.Rollover,
		GrossPL:       p.GrossPL,
		Comment:       p.Comment,
	}
}

// parsePositions reads a position sheet into validated canonical positions. It
// returns nil when the sheet holds no position table.
func parsePositions(rows [][]string, accountID string) ([]model.Position, error) {
	data, cols, ok := dataRows(rows, positionHeader)
	if !ok {
		return nil, nil
	}

	positions := make([]model.Position, 0, len(data))
	for _, row := range data {
		raw, err := parsePositionRow(row, cols)
		if err != nil {
			return nil, err
		}
		position := raw.ToPosition(accountID)
		if err := position.Validate(); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}

	return positions, nil
}

// parsePositionRow maps one sheet row onto an XTBPosition.
func parsePositionRow(row []string, cols columns) (XTBPosition, error) {
	var pos XTBPosition

	pos.Position = cols.get(row, "Position")
	pos.Symbol = cols.get(row, "Symbol")
	pos.Type = cols.get(row, "Type")
	pos.OpenOrigin = cols.get(row, "Open origin")
	pos.CloseOrigin = cols.get(row, "Close origin")
	pos.Comment = cols.get(row, "Comment")

	floatFields := []struct {
		dst   *float64
		label string
	}{
		{&pos.Volume, "Volume"},
		{&pos.OpenPrice, "Open price"},
		{&pos.ClosePrice, "Close price"},
		{&pos.MarketPrice, "Market price"},
		{&pos.PurchaseValue, "Purchase value"},
		{&pos.SaleValue, "Sale value"},
		{&pos.SL, "SL"},
		{&pos.TP, "TP"},
		{&pos.Margin, "Margin"},
		{&pos.Commission, "Commission"},
		{&pos.Swap, "Swap"},
		{&pos.Rollover, "Rollover"},
		{&pos.GrossPL, "Gross P/L"},
	}
	for _, field := range floatFields {
		value, err := parseFloat(cols.get(row, field.label))
		if err != nil {
			return XTBPosition{}, fmt.Errorf("position %s: field %q: %w", pos.Position, field.label, err)
		}
		*field.dst = value
	}

	timeFields := []struct {
		dst   *time.Time
		label string
	}{
		{&pos.OpenTime, "Open time"},
		{&pos.CloseTime, "Close time"},
	}
	for _, field := range timeFields {
		value, err := parseTime(cols.get(row, field.label))
		if err != nil {
			return XTBPosition{}, fmt.Errorf("position %s: field %q: %w", pos.Position, field.label, err)
		}
		*field.dst = value
	}

	return pos, nil
}
