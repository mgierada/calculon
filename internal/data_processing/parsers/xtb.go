package parsers

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const xtbTimeLayout = "02/01/2006 15:04:05"

type Position struct {
	ID            string
	Symbol        string
	Type          string
	Volume        float64
	OpenTime      time.Time
	OpenPrice     float64
	CloseTime     time.Time
	ClosePrice    float64
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

// ParseClosedPositions extracts closed positions from raw XTB statement rows.
// It locates the position table by its header row and reads data rows until the
// trailing "Total" summary row.
func ParseClosedPositions(rows [][]string) ([]Position, error) {
	headerIdx, cols := findHeader(rows)
	if headerIdx == -1 {
		return nil, fmt.Errorf("position header row not found")
	}

	var positions []Position
	for _, row := range rows[headerIdx+1:] {
		if firstCell(row) == "Total" {
			break
		}
		if isEmptyRow(row) {
			continue
		}

		pos, err := parseRow(row, cols)
		if err != nil {
			return nil, err
		}
		positions = append(positions, pos)
	}

	return positions, nil
}

// findHeader returns the index of the position table header row and a map from
// header label to column index. Returns -1 when no header is found.
func findHeader(rows [][]string) (int, map[string]int) {
	for i, row := range rows {
		cols := make(map[string]int, len(row))
		for j, cell := range row {
			cols[strings.TrimSpace(cell)] = j
		}

		_, hasSymbol := cols["Symbol"]
		_, hasVolume := cols["Volume"]
		_, hasGrossPL := cols["Gross P/L"]
		if hasSymbol && hasVolume && hasGrossPL {
			return i, cols
		}
	}
	return -1, nil
}

// parseRow maps a single statement row to a Position using the header column map.
func parseRow(row []string, cols map[string]int) (Position, error) {
	get := func(label string) string {
		idx, ok := cols[label]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	var pos Position
	var err error

	pos.ID = get("Position")
	pos.Symbol = get("Symbol")
	pos.Type = get("Type")
	pos.OpenOrigin = get("Open origin")
	pos.CloseOrigin = get("Close origin")
	pos.Comment = get("Comment")

	floatFields := []struct {
		dst   *float64
		label string
	}{
		{&pos.Volume, "Volume"},
		{&pos.OpenPrice, "Open price"},
		{&pos.ClosePrice, "Close price"},
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
	for _, f := range floatFields {
		if *f.dst, err = parseFloat(get(f.label)); err != nil {
			return Position{}, fmt.Errorf("position %s: field %q: %w", pos.ID, f.label, err)
		}
	}

	if pos.OpenTime, err = parseTime(get("Open time")); err != nil {
		return Position{}, fmt.Errorf("position %s: open time: %w", pos.ID, err)
	}
	if pos.CloseTime, err = parseTime(get("Close time")); err != nil {
		return Position{}, fmt.Errorf("position %s: close time: %w", pos.ID, err)
	}

	return pos, nil
}

// parseFloat parses an XTB numeric cell, treating an empty cell as zero.
func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.ParseFloat(s, 64)
}

// parseTime parses an XTB timestamp cell, treating an empty cell as the zero time.
func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(xtbTimeLayout, s)
}

// firstCell returns the first non-empty, trimmed cell of a row, or "" if none.
func firstCell(row []string) string {
	for _, c := range row {
		if t := strings.TrimSpace(c); t != "" {
			return t
		}
	}
	return ""
}

func isEmptyRow(row []string) bool {
	return firstCell(row) == ""
}
