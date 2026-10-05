// Package symbolmap translates our symbols, which follow XTB's notation, into
// the names market data providers use for the same instruments.
package symbolmap

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mgierada/calculon/internal/db"
)

// Finimpulse is the provider name mappings to finimpulse are stored under.
const Finimpulse = "finimpulse"

// finimpulseSuffixes turns an XTB exchange suffix into finimpulse's, which
// follows Yahoo Finance: US listings carry none.
var finimpulseSuffixes = map[string]string{
	".PL": ".WA",
	".US": "",
	".UK": ".L",
	".DE": ".DE",
	".FR": ".PA",
	".NL": ".AS",
	".ES": ".MC",
	".IT": ".MI",
	".CH": ".SW",
	".BE": ".BR",
	".PT": ".LS",
	".DK": ".CO",
	".NO": ".OL",
	".SE": ".ST",
	".FI": ".HE",
}

// ToFinimpulse names symbol the way finimpulse does. It reports false for
// symbols without a known exchange suffix, such as XTB's CFDs (US500, GOLD).
func ToFinimpulse(symbol string) (string, bool) {
	suffix := Suffix(symbol)
	providerSuffix, ok := finimpulseSuffixes[suffix]
	if suffix == "" || !ok {
		return "", false
	}
	return strings.TrimSuffix(symbol, suffix) + providerSuffix, true
}

// Suffix is the exchange suffix of an XTB symbol, dot included, e.g. ".PL",
// or empty when it has none.
func Suffix(symbol string) string {
	dot := strings.LastIndex(symbol, ".")
	if dot <= 0 {
		return ""
	}
	return symbol[dot:]
}

// Sync stores a finimpulse mapping for every held symbol that has one. Stored
// mappings are kept, so one corrected by hand survives. It returns the held
// symbols no rule maps.
func Sync(conn *sql.DB) ([]string, error) {
	held, err := db.HeldSymbols(conn)
	if err != nil {
		return nil, err
	}
	mapped := map[string]string{}
	var unmapped []string
	for _, symbol := range held {
		if providerSymbol, ok := ToFinimpulse(symbol); ok {
			mapped[symbol] = providerSymbol
		} else {
			unmapped = append(unmapped, symbol)
		}
	}
	return unmapped, db.StoreSymbolMappings(conn, Finimpulse, mapped)
}

// Lookup names symbol the way finimpulse does: the stored mapping, so a hand
// fix applies, or the suffix rules for a symbol not mapped yet.
func Lookup(conn *sql.DB, symbol string) (string, error) {
	stored, ok, err := db.ProviderSymbol(conn, symbol, Finimpulse)
	if err != nil || ok {
		return stored, err
	}
	if mapped, ok := ToFinimpulse(symbol); ok {
		return mapped, nil
	}
	return "", fmt.Errorf("no finimpulse symbol for %s", symbol)
}
