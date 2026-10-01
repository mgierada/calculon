package widgets

import (
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// highlightedSymbol is the symbol of the row enter would open.
func highlightedSymbol(table *Table) string {
	row, _ := table.highlighted()
	return row["symbol"].(string)
}

// rebuilt is a fresh sortable table over rows, sized like sortTestTable.
func rebuilt(rows []Row) *Table {
	columns := sortTestTable().columns
	table := NewTable("positions", columns, rows).Sortable().KeyedBy("symbol")
	table.SetSize(80, 12)
	table.SetFocused(true)
	return table
}

func TestTableRestoresSortAndCursorRow(t *testing.T) {
	old := sortTestTable().KeyedBy("symbol")
	old.Update(sortKey("s"))
	old.Update(sortKey("s"))
	old.Update(tea.KeyPressMsg(tea.Key{Text: "j", Code: 'j'}))
	if got := highlightedSymbol(old); got != "b.PL" {
		t.Fatalf("setup: cursor on %s, want b.PL", got)
	}

	// New prices make C.PL the largest, moving b.PL down a row.
	rows := slices.Clone(old.rows)
	rows[2] = Row{"symbol": "C.PL", "value": "2 000.00 PLN", SortBy("value"): 2000.0}
	table := rebuilt(rows)
	table.Restore(old.State())

	if got := order(t, table); !slices.Equal(got, []string{"C.PL", "A.PL", "b.PL"}) {
		t.Errorf("order = %v, want still sorted by value descending", got)
	}
	if got := highlightedSymbol(table); got != "b.PL" {
		t.Errorf("cursor on %s, want it to follow b.PL", got)
	}
}

func TestTableRestoresUnsortedAndFilter(t *testing.T) {
	old := sortTestTable().Filterable()
	old.SortedBy("value", true)
	// From value, s moves to day, then to unsorted.
	for range 2 {
		old.Update(sortKey("s"))
	}
	old.table = old.table.WithFilterInputValue("pl")

	table := rebuilt(old.rows).Filterable()
	table.SortedBy("value", true)
	table.Restore(old.State())

	if table.sorted() {
		t.Error("table sorted again, want the unsorted order the user picked kept")
	}
	if got := table.table.GetCurrentFilter(); got != "pl" {
		t.Errorf("filter = %q, want it kept", got)
	}
}

func TestTableIgnoresForeignState(t *testing.T) {
	table := sortTestTable()
	table.SortedBy("value", true)
	table.Restore("not a table's state")

	if !table.sorted() {
		t.Error("foreign state changed the table")
	}
}
