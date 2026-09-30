package widgets

import (
	"fmt"
	"sort"
	"strings"

	btable "github.com/evertras/bubble-table/table"
)

// Keys that change how a sortable table orders its rows.
const (
	sortNextKey    = "s"
	sortReverseKey = "S"
)

// unsorted means rows keep the order they were given in.
const unsorted = -1

// Sort direction markers appended to the sorted column's title.
const (
	ascendingMarker  = " ▲"
	descendingMarker = " ▼"
)

// SortBy is the row key that holds a column's sort value. Cells are rendered
// text such as "1 234.00 PLN", which sorts wrongly, so rows put the raw value
// under this key: a float64 for numbers, a string for text, or nil for a
// value that is not known, which sorts last either way. Columns without one
// sort by their text.
func SortBy(column string) string {
	return "_sort:" + column
}

// Sortable lets the user order rows by any column: s moves to the next
// column, S reverses the direction. Rows start in the order given.
func (t *Table) Sortable() *Table {
	t.sortable = true
	t.sortColumn = unsorted
	return t
}

// handleSortKey applies a sort key, reporting whether it was one.
func (t *Table) handleSortKey(key string) bool {
	if !t.sortable || t.CapturingInput() || len(t.columns) == 0 {
		return false
	}
	switch key {
	case sortNextKey:
		t.sortColumn++
		if t.sortColumn >= len(t.columns) {
			t.sortColumn = unsorted
		}
		// Numbers read best largest first, text alphabetically.
		if t.sortColumn != unsorted {
			t.sortDesc = !t.columns[t.sortColumn].Left
		}
	case sortReverseKey:
		if t.sortColumn == unsorted {
			return true
		}
		t.sortDesc = !t.sortDesc
	default:
		return false
	}
	t.applySort()
	return true
}

// sorted reports whether rows are currently ordered by a column.
func (t *Table) sorted() bool {
	return t.sortable && t.sortColumn != unsorted
}

// applySort shows the rows in the current order and marks the sorted column.
func (t *Table) applySort() {
	rows := t.rows
	if t.sorted() {
		rows = sortedRows(t.rows, t.columns[t.sortColumn].Key, t.sortDesc)
	}

	columns := append([]Column{}, t.columns...)
	if t.sorted() {
		marker := ascendingMarker
		if t.sortDesc {
			marker = descendingMarker
		}
		columns[t.sortColumn].Title += marker
	}
	t.table = t.table.WithColumns(toColumns(columns, t.filtered)).WithRows(toRows(rows))
}

// sortedRows orders a copy of the rows by one column, unknown values last.
func sortedRows(rows []Row, column string, desc bool) []Row {
	sorted := append([]Row{}, rows...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sortValue(sorted[i], column), sortValue(sorted[j], column)
		switch {
		case a == nil || b == nil:
			return a != nil
		case desc:
			return less(b, a)
		default:
			return less(a, b)
		}
	})
	return sorted
}

// sortValue is the value a row sorts by in a column, nil when unknown.
func sortValue(row Row, column string) any {
	if value, ok := row[SortBy(column)]; ok {
		return value
	}
	value := row[column]
	if cell, ok := value.(btable.StyledCell); ok {
		value = cell.Data
	}
	if value == nil {
		return nil
	}
	if number, ok := value.(float64); ok {
		return number
	}
	return strings.ToLower(fmt.Sprint(value))
}

// less compares numbers numerically and anything else as text.
func less(a, b any) bool {
	x, xNumber := a.(float64)
	y, yNumber := b.(float64)
	if xNumber && yNumber {
		return x < y
	}
	return fmt.Sprint(a) < fmt.Sprint(b)
}
