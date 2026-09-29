package xtb

import (
	"fmt"
	"strings"

	"github.com/mgierada/calculon/internal/model"
)

// closedHeader are the labels that identify the closed position table.
var closedHeader = []string{"Ticker", "Position ID", "Volume", "Close Time (UTC)"}

// openHeader are the labels that identify the open position table.
var openHeader = []string{"Instrument/Position", "Ticker", "Type", "Volume", "Current price"}

// parseClosedPositions reads the closed position sheet into validated records.
// It returns nil when the sheet holds no closed position table.
func parseClosedPositions(rows [][]string) ([]model.Position, error) {
	data, cols, ok := dataRows(rows, closedHeader)
	if !ok {
		return nil, nil
	}

	seqs := sequencer{}
	positions := make([]model.Position, 0, len(data))
	for _, row := range data {
		position, err := parseClosedRow(row, cols)
		if err != nil {
			return nil, err
		}
		position.Seq = seqs.next(position.Key())
		if err := position.Validate(); err != nil {
			return nil, err
		}
		positions = append(positions, position)
	}
	return positions, nil
}

// parseClosedRow maps one closed position row onto a position.
func parseClosedRow(row []string, cols columns) (model.Position, error) {
	p := fieldParser{row: row, cols: cols}
	position := model.Position{
		Instrument: model.Instrument{
			Symbol:   p.text("Ticker"),
			Name:     p.text("Instrument"),
			Category: p.text("Category"),
		},
		PositionID:    p.text("Position ID"),
		Product:       p.text("Product"),
		Side:          model.Side(strings.ToUpper(p.text("Type"))),
		Volume:        p.float("Volume"),
		OpenTime:      p.time("Open Time (UTC)"),
		OpenPrice:     p.float("Open Price"),
		CloseTime:     p.time("Close Time (UTC)"),
		ClosePrice:    p.float("Close Price"),
		PurchaseValue: p.float("Purchase Value"),
		SaleValue:     p.float("Sale Value"),
		Commission:    p.float("Commission"),
		Swap:          p.float("Swap"),
		Rollover:      p.float("Rollover"),
		GrossPL:       p.float("Gross Profit"),
		NetPL:         p.float("Profit/Loss"),
		CloseOrigin:   p.text("Close Origin"),
		Comment:       p.text("Comment"),
	}
	if p.err != nil {
		return model.Position{}, fmt.Errorf("position %s: field %q: %w",
			position.PositionID, p.field, p.err)
	}
	return position, nil
}

// parseOpenLots reads the open position sheet. XTB groups it by ticker: a
// summary row carrying the instrument name and category (and no side) precedes
// one row per lot, whose "Instrument/Position" cell holds the position id.
func parseOpenLots(rows [][]string) ([]model.OpenLot, error) {
	data, cols, ok := dataRows(rows, openHeader)
	if !ok {
		return nil, nil
	}

	instruments := map[string]model.Instrument{}
	seqs := sequencer{}
	var lots []model.OpenLot
	for _, row := range data {
		if cols.get(row, "Type") == "" {
			symbol := cols.get(row, "Ticker")
			instruments[symbol] = model.Instrument{
				Symbol:   symbol,
				Name:     cols.get(row, "Instrument/Position"),
				Category: cols.get(row, "Category"),
			}
			continue
		}

		lot, err := parseOpenLotRow(row, cols)
		if err != nil {
			return nil, err
		}
		if instrument, ok := instruments[lot.Symbol]; ok {
			lot.Instrument = instrument
		}
		lot.Seq = seqs.next(lot.PositionID)
		if err := lot.Validate(); err != nil {
			return nil, err
		}
		lots = append(lots, lot)
	}
	return lots, nil
}

// parseOpenLotRow maps one lot row onto an open lot.
func parseOpenLotRow(row []string, cols columns) (model.OpenLot, error) {
	p := fieldParser{row: row, cols: cols}
	lot := model.OpenLot{
		Instrument:   model.Instrument{Symbol: p.text("Ticker")},
		PositionID:   p.text("Instrument/Position"),
		Product:      p.text("Product"),
		Side:         model.Side(strings.ToUpper(p.text("Type"))),
		Volume:       p.float("Volume"),
		OpenTime:     p.time("Open time (UTC)"),
		OpenPrice:    p.float("Open price"),
		CurrentPrice: p.float("Current price"),
		Value:        p.float("Value"),
		GrossPL:      p.float("Gross Profit"),
		NetPL:        p.float("Net Profit"),
		Commission:   p.float("Open Commission"),
		Swap:         p.float("Swap"),
	}
	if p.err != nil {
		return model.OpenLot{}, fmt.Errorf("open lot %s: field %q: %w", lot.PositionID, p.field, p.err)
	}
	return lot, nil
}
