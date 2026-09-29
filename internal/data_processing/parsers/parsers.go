// Package parsers resolves a statement file to the provider parser that can
// read it. Provider implementations live in subpackages and only depend on
// internal/model, so adding an xlsx or csv provider means writing a Parser and
// registering it here.
package parsers

import (
	"fmt"
	"io/fs"
	"os"
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
	// Accepts reports whether the file at path is a statement this parser reads.
	Accepts(path string) bool
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

// Detect finds the provider whose parser accepts the file.
func Detect(path string) (model.Provider, error) {
	for _, name := range Providers() {
		provider := model.Provider(name)
		if registry[provider].Accepts(path) {
			return provider, nil
		}
	}
	return "", fmt.Errorf("cannot detect provider for %q, pass --provider (one of: %s)",
		filepath.Base(path), strings.Join(Providers(), ", "))
}

// Expand turns a mix of files and directories into the statement files to
// import. Directories are walked recursively and contribute every file some
// parser accepts; files named explicitly are always kept so a wrong one fails
// loudly instead of being skipped.
func Expand(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		found, err := statementsIn(path)
		if err != nil {
			return nil, err
		}
		files = append(files, found...)
	}
	return files, nil
}

// statementsIn walks a directory for files some parser accepts, sorted by path.
func statementsIn(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		if _, err := Detect(path); err == nil {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan %q: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}
