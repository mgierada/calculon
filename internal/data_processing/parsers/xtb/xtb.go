// Package xtb parses XTB brokerage xlsx account statements into the canonical
// records in internal/model.
//
// An XTB statement holds several sheets. Closed trades live in the closed
// position sheet; CFD positions still open live in the open position sheet.
// Cash-equity holdings appear in neither: XTB records them only in the cash
// operation sheet, as "Stock purchase" and "Stock sale" rows whose volume and
// price are buried in a free-text comment. All three are parsed here.
package xtb

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mgierada/calculon/internal/data_processing/readers"
	"github.com/mgierada/calculon/internal/model"
)

const (
	sheetClosedPositions = "CLOSED POSITION HISTORY"
	// The open position sheet name carries the export date, e.g.
	// "OPEN POSITION 27062026", so it is matched by prefix.
	sheetOpenPositions = "OPEN POSITION"
	sheetCashOps       = "CASH OPERATION HISTORY"

	accountLabel = "Account"
)

// Parser reads XTB xlsx account statements.
type Parser struct{}

// New returns an XTB statement parser.
func New() Parser {
	return Parser{}
}

// Provider identifies the records this parser produces.
func (Parser) Provider() model.Provider {
	return model.ProviderXTB
}

// Parse reads every data-bearing sheet of the statement at path. It fails on a
// malformed row rather than skipping it, so a format change surfaces instead of
// silently dropping trades.
func (p Parser) Parse(path string) (model.Statement, error) {
	sheets, err := reader.ReadXlsxSheets(path)
	if err != nil {
		return model.Statement{}, err
	}

	accountID, err := findAccountID(sheets)
	if err != nil {
		return model.Statement{}, fmt.Errorf("%s: %w", path, err)
	}

	statement := model.Statement{Provider: model.ProviderXTB, AccountID: accountID}

	for _, name := range []string{sheetClosedPositions, sheetOpenPositions} {
		rows, ok := findSheet(sheets, name)
		if !ok {
			continue
		}
		positions, err := parsePositions(rows, accountID)
		if err != nil {
			return model.Statement{}, fmt.Errorf("%s: sheet %q: %w", path, name, err)
		}
		statement.Positions = append(statement.Positions, positions...)
	}

	if rows, ok := findSheet(sheets, sheetCashOps); ok {
		cashOps, err := parseCashOps(rows, accountID)
		if err != nil {
			return model.Statement{}, fmt.Errorf("%s: sheet %q: %w", path, sheetCashOps, err)
		}
		statement.CashOps = cashOps
	}

	return statement, nil
}

// findSheet looks up a sheet by exact name, falling back to a prefix match for
// sheets whose name embeds the export date.
func findSheet(sheets map[string][][]string, name string) ([][]string, bool) {
	if rows, ok := sheets[name]; ok {
		return rows, true
	}
	for sheetName, rows := range sheets {
		if strings.HasPrefix(sheetName, name) {
			return rows, true
		}
	}
	return nil, false
}

// findAccountID reads the account number from the statement header block, where
// it sits directly below an "Account" label.
func findAccountID(sheets map[string][][]string) (string, error) {
	names := make([]string, 0, len(sheets))
	for name := range sheets {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if id := accountIDFromRows(sheets[name]); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("account number not found in any sheet header")
}

// accountIDFromRows finds the "Account" label and returns the value beneath it.
func accountIDFromRows(rows [][]string) string {
	for i, row := range rows {
		for col, cell := range row {
			if strings.TrimSpace(cell) != accountLabel {
				continue
			}
			if i+1 >= len(rows) {
				continue
			}
			below := rows[i+1]
			if col >= len(below) {
				continue
			}
			if id := strings.TrimSpace(below[col]); id != "" {
				return id
			}
		}
	}
	return ""
}
