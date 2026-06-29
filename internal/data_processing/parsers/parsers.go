// Package parsers resolves a statement file to the provider parser that can
// read it. Provider implementations live in subpackages and only depend on
// internal/model, so adding an xlsx or csv provider means writing a Parser and
// registering it here.
package parsers

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mgierada/calculon/internal/data_processing/parsers/xtb"
	"github.com/mgierada/calculon/internal/model"
)

// Parser turns one provider statement file into normalized records.
type Parser interface {
	// Provider identifies the records this parser produces.
	Provider() model.Provider
	// Parse reads the statement file at path.
	Parse(path string) (model.Statement, error)
}

// registry holds every known provider parser.
var registry = map[model.Provider]Parser{
	model.ProviderXTB: xtb.New(),
}

// Get returns the parser for a provider.
func Get(provider model.Provider) (Parser, error) {
	parser, ok := registry[provider]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q, known providers: %s",
			provider, strings.Join(Providers(), ", "))
	}
	return parser, nil
}

// Providers lists the registered provider names, sorted.
func Providers() []string {
	names := make([]string, 0, len(registry))
	for provider := range registry {
		names = append(names, string(provider))
	}
	sort.Strings(names)
	return names
}

// Detect guesses the provider from a statement file name. XTB names its exports
// "account_<number>_<currency>_xlsx_<from>_<to>.xlsx".
func Detect(path string) (model.Provider, error) {
	name := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(name, "account_") && strings.HasSuffix(name, ".xlsx") {
		return model.ProviderXTB, nil
	}
	return "", fmt.Errorf("cannot detect provider for %q, pass --provider (one of: %s)",
		filepath.Base(path), strings.Join(Providers(), ", "))
}
