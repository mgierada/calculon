package xtb

import (
	"strconv"
	"strings"
	"time"
)

// xtbTimeLayout is how XTB writes timestamps, always in UTC,
// e.g. "2026-09-16 08:17:01".
const xtbTimeLayout = "2006-01-02 15:04:05"

// summaryRowLabels mark the rows that terminate a data table.
var summaryRowLabels = map[string]bool{"Total": true, "Profit/loss": true}

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

// dataRows returns the rows between a table header and its summary row,
// skipping blank rows. The bool is false when the header is missing.
func dataRows(rows [][]string, required []string) ([][]string, columns, bool) {
	headerIdx, cols := findHeader(rows, required)
	if headerIdx == -1 {
		return nil, nil, false
	}

	var data [][]string
	for _, row := range rows[headerIdx+1:] {
		first := firstCell(row)
		if summaryRowLabels[first] {
			return data, cols, true
		}
		if first == "" {
			continue
		}
		data = append(data, row)
	}
	return data, cols, true
}

// headerValue returns the cell right of a label in the metadata block at the
// top of a sheet, e.g. "Account number | 50747414".
func headerValue(rows [][]string, label string) string {
	for _, row := range rows {
		for col, cell := range row {
			if strings.TrimSpace(cell) != label || col+1 >= len(row) {
				continue
			}
			return strings.TrimSpace(row[col+1])
		}
	}
	return ""
}

// fieldParser collects the first error hit while reading a row's cells, so a
// row reads as a flat list of assignments instead of a ladder of checks.
type fieldParser struct {
	row  []string
	cols columns
	err  error
	// field names the first cell that failed to parse.
	field string
}

func (p *fieldParser) text(label string) string {
	return p.cols.get(p.row, label)
}

func (p *fieldParser) float(label string) float64 {
	value, err := parseFloat(p.cols.get(p.row, label))
	p.fail(label, err)
	return value
}

func (p *fieldParser) time(label string) time.Time {
	value, err := parseTime(p.cols.get(p.row, label))
	p.fail(label, err)
	return value
}

func (p *fieldParser) fail(label string, err error) {
	if err != nil && p.err == nil {
		p.err, p.field = err, label
	}
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
	return time.ParseInLocation(xtbTimeLayout, s, time.UTC)
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

// sequencer numbers rows that share a key, in file order, so otherwise
// identical rows get distinct and stable identities.
type sequencer map[string]int

func (s sequencer) next(key string) int {
	seq := s[key]
	s[key] = seq + 1
	return seq
}
