package widgets

// tableState is what a table keeps across a rebuild from a new report.
type tableState struct {
	// sortKey is the sorted column's key, empty when unsorted.
	sortKey  string
	sortDesc bool
	filter   string
	// cursorKey identifies the highlighted row in a keyed table; cursorIndex
	// places the cursor when the table is unkeyed or the row is gone.
	cursorKey   any
	cursorIndex int
}

// KeyedBy names the row key that identifies a row across rebuilds, so the
// cursor stays on the same row when new data reorders the table.
func (t *Table) KeyedBy(key string) *Table {
	t.rowKey = key
	return t
}

// State implements ui.Stateful.
func (t *Table) State() any {
	state := tableState{
		sortDesc:    t.sortDesc,
		filter:      t.table.GetCurrentFilter(),
		cursorIndex: t.table.GetHighlightedRowIndex(),
	}
	if t.sorted() {
		state.sortKey = t.columns[t.sortColumn].Key
	}
	if row, ok := t.highlighted(); ok && t.rowKey != "" {
		state.cursorKey = row[t.rowKey]
	}
	return state
}

// Restore implements ui.Stateful. Call it once the table is sized: which page
// shows the cursor depends on the height.
func (t *Table) Restore(state any) {
	s, ok := state.(tableState)
	if !ok {
		return
	}
	if t.sortable {
		t.sortColumn, t.sortDesc = t.columnIndex(s.sortKey), s.sortDesc
		t.applySort()
	}
	if t.filtered && s.filter != "" {
		t.table = t.table.WithFilterInputValue(s.filter)
	}
	t.table = t.table.WithHighlightedRow(t.cursorIndex(s))
}

// columnIndex is the index of the column with key, or unsorted when none has it.
func (t *Table) columnIndex(key string) int {
	for i, c := range t.columns {
		if key != "" && c.Key == key {
			return i
		}
	}
	return unsorted
}

// cursorIndex is where the restored cursor goes: the row it was on when the
// table is keyed and the row is still visible, otherwise the same position.
func (t *Table) cursorIndex(s tableState) int {
	if t.rowKey == "" || s.cursorKey == nil {
		return s.cursorIndex
	}
	for i, row := range t.table.GetVisibleRows() {
		if row.Data[t.rowKey] == s.cursorKey {
			return i
		}
	}
	return s.cursorIndex
}
