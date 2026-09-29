// Package xtb parses XTB brokerage xlsx account statements into the canonical
// records in internal/model.
//
// An XTB statement holds three sheets. "Closed Positions" and "Cash Operations"
// are the account's full history; "Open Positions" is a snapshot of what is
// still held, valued at the time the report was generated. Each sheet starts
// with a small metadata block ("Account number", report dates) above its table.
package xtb

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	reader "github.com/mgierada/calculon/internal/data_processing/readers"
	"github.com/mgierada/calculon/internal/model"
)

const (
	sheetClosedPositions = "Closed Positions"
	sheetOpenPositions   = "Open Positions"
	sheetCashOps         = "Cash Operations"

	labelAccount  = "Account number"
	labelAsOf     = "Data as of report generated"
	labelDateTo   = "Date to (UTC)"
	labelCurrency = "Currency"
	labelMetric   = "Metric"

	summaryMetricPrefix = "Open position"
)

// accountSheets are searched in order for the account number. The open position
// sheet comes last because XTB stamps the parent account number on it for
// sub-accounts such as IKE, while the history sheets carry the real one.
var accountSheets = []string{sheetClosedPositions, sheetCashOps, sheetOpenPositions}

// productCurrencies are the currencies of XTB products whose statement neither
// states a currency nor is named after one.
var productCurrencies = map[string]string{"IKE": "PLN", "IKZE": "PLN"}

// currencyCode matches an ISO 4217 code at the start of a statement file name,
// e.g. "USD_51727538_2006-01-01_2026-09-28.xlsx".
var currencyCode = regexp.MustCompile(`^([A-Z]{3})_`)

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

// Accepts reports whether the file at path is an XTB statement, judged by the
// sheets it contains rather than its name.
func (Parser) Accepts(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		return false
	}
	names, err := reader.XlsxSheetNames(path)
	if err != nil {
		return false
	}
	for _, name := range names {
		if name == sheetCashOps {
			return true
		}
	}
	return false
}

// Parse reads every data-bearing sheet of the statement at path. It fails on a
// malformed row rather than skipping it, so a format change surfaces instead of
// silently dropping trades.
func (Parser) Parse(path string) (model.Statement, error) {
	sheets, err := reader.ReadXlsxSheets(path)
	if err != nil {
		return model.Statement{}, err
	}
	statement, err := parseSheets(sheets, filepath.Base(path))
	if err != nil {
		return model.Statement{}, fmt.Errorf("%s: %w", path, err)
	}
	return statement, nil
}

// parseSheets builds a statement from the workbook's sheets. fileName is used
// only as a fallback source for the account currency.
func parseSheets(sheets map[string][][]string, fileName string) (model.Statement, error) {
	account, err := parseAccount(sheets, fileName)
	if err != nil {
		return model.Statement{}, err
	}
	statement := model.Statement{Account: account, AsOf: parseAsOf(sheets)}

	if statement.Positions, err = parseClosedPositions(sheets[sheetClosedPositions]); err != nil {
		return model.Statement{}, fmt.Errorf("sheet %q: %w", sheetClosedPositions, err)
	}
	if statement.OpenLots, err = parseOpenLots(sheets[sheetOpenPositions]); err != nil {
		return model.Statement{}, fmt.Errorf("sheet %q: %w", sheetOpenPositions, err)
	}
	if statement.CashOps, err = parseCashOps(sheets[sheetCashOps]); err != nil {
		return model.Statement{}, fmt.Errorf("sheet %q: %w", sheetCashOps, err)
	}
	return statement, nil
}

// parseAccount reads the account number and currency.
func parseAccount(sheets map[string][][]string, fileName string) (model.Account, error) {
	account := model.Account{Provider: model.ProviderXTB}
	for _, name := range accountSheets {
		if id := headerValue(sheets[name], labelAccount); id != "" {
			account.ID = id
			break
		}
	}
	if account.ID == "" {
		return model.Account{}, fmt.Errorf("account number not found in any sheet header")
	}

	account.Currency = parseCurrency(sheets, fileName)
	if account.Currency == "" {
		return model.Account{}, fmt.Errorf("account %s: cannot determine currency", account.ID)
	}
	return account, nil
}

// parseCurrency reads the currency from the open position summary table, then
// falls back to the product the statement covers and to the file name.
func parseCurrency(sheets map[string][][]string, fileName string) string {
	rows := sheets[sheetOpenPositions]
	data, cols, ok := dataRows(rows, []string{"Product", labelMetric, "Amount", labelCurrency})
	if ok {
		for _, row := range data {
			// The summary table runs straight into the lot table, whose
			// columns mean something else, so only metric rows are read.
			if !strings.HasPrefix(cols.get(row, labelMetric), summaryMetricPrefix) {
				continue
			}
			if currency := cols.get(row, labelCurrency); currency != "" {
				return currency
			}
			if currency, ok := productCurrencies[cols.get(row, "Product")]; ok {
				return currency
			}
		}
	}
	if match := currencyCode.FindStringSubmatch(fileName); match != nil {
		return match[1]
	}
	return ""
}

// parseAsOf reads when the open position snapshot was taken, falling back to the
// end of the reported period.
func parseAsOf(sheets map[string][][]string) time.Time {
	for _, candidate := range []struct{ sheet, label string }{
		{sheetOpenPositions, labelAsOf},
		{sheetCashOps, labelDateTo},
		{sheetClosedPositions, labelDateTo},
	} {
		if asOf, err := parseTime(headerValue(sheets[candidate.sheet], candidate.label)); err == nil &&
			!asOf.IsZero() {
			return asOf
		}
	}
	return time.Time{}
}
