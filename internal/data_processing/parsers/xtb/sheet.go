package xtb

import (
	"strconv"
	"strings"
	"time"
)

// xtbTimeLayout is how XTB writes timestamps, e.g. "02/01/2006 15:04:05".
const xtbTimeLayout = "02/01/2006 15:04:05"

// totalRowLabel marks the summary row that terminates a data table.
const totalRowLabel = "Total"

// columns maps a header label to its column index.
type columns map[string]int

// findHeader returns the index of the first row containing every required label
// and a lookup from label to column index. The index is -1 when not found.
func findHeader(rows [][]string, required []string) (int, columns) {
	for i, row := range rows {
		cols := make(columns, len(row))
		for j, cell := range row {
			cols[strings.TrimSpace(cell)] = j
		}
		if cols.hasAll(required) {
			return i, cols
		}
	}
	return -1, nil
}

func (c columns) hasAll(labels []string) bool {
	for _, label := range labels {
		if _, ok := c[label]; !ok {
			return false
		}
	}
	return true
}

// get returns a trimmed cell value, or "" when the column is absent from this
// sheet or the row is short.
func (c columns) get(row []string, label string) string {
	idx, ok := c[label]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// dataRows returns the rows between a table header and its "Total" summary row,
// skipping blank rows. The bool is false when the header is missing.
func dataRows(rows [][]string, required []string) ([][]string, columns, bool) {
	headerIdx, cols := findHeader(rows, required)
	if headerIdx == -1 {
		return nil, nil, false
	}

	var data [][]string
	for _, row := range rows[headerIdx+1:] {
		switch firstCell(row) {
		case totalRowLabel:
			return data, cols, true
		case "":
			continue
		}
		data = append(data, row)
	}
	return data, cols, true
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
	for _, cell := range row {
		if trimmed := strings.TrimSpace(cell); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
