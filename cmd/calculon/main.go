// Command calculon imports broker statements and shows the resulting portfolio
// in a fullscreen terminal dashboard.
//
//	calculon                            fullscreen dashboard
//	calculon ui                         same
//	calculon import statement.xlsx      import one or more statements
//	calculon import --provider xtb f.xlsx
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mgierada/calculon/internal/config"
	"github.com/mgierada/calculon/internal/data_processing/parsers"
	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/ui"
)

const usage = `calculon - portfolio dashboard

usage:
  calculon [ui]                              open the fullscreen dashboard
  calculon import [--provider NAME] FILE...  import broker statements

flags:
  --provider NAME   force a provider instead of detecting it from the file name
`

func main() {
	log.SetFlags(0)

	args := os.Args[1:]
	command := "ui"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command, args = args[0], args[1:]
	}

	var err error
	switch command {
	case "ui":
		err = runUI()
	case "import":
		err = runImport(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		err = fmt.Errorf("unknown command %q\n\n%s", command, usage)
	}

	if err != nil {
		log.Fatal(err)
	}
}

// runUI opens the dashboard over the portfolio currently in the database.
func runUI() error {
	conn, err := openDB()
	if err != nil {
		return err
	}
	defer conn.Close()

	holdings, err := db.OpenHoldings(conn)
	if err != nil {
		return err
	}

	return ui.Run(ui.PortfolioGrid(holdings, time.Now()))
}

// runImport parses each statement and stores it, skipping records already held.
func runImport(args []string) error {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	providerName := flags.String("provider", "",
		"provider to parse with, one of: "+strings.Join(parsers.Providers(), ", "))
	if err := flags.Parse(args); err != nil {
		return err
	}

	paths := flags.Args()
	if len(paths) == 0 {
		return fmt.Errorf("import needs at least one statement file\n\n%s", usage)
	}

	conn, err := openDB()
	if err != nil {
		return err
	}
	defer conn.Close()

	for _, path := range paths {
		if err := importFile(conn, path, model.Provider(*providerName)); err != nil {
			return err
		}
	}
	return nil
}

// importFile imports a single statement and logs what changed.
func importFile(conn *db.Conn, path string, provider model.Provider) error {
	if provider == "" {
		detected, err := parsers.Detect(path)
		if err != nil {
			return err
		}
		provider = detected
	}

	parser, err := parsers.Get(provider)
	if err != nil {
		return err
	}

	statement, err := parser.Parse(path)
	if err != nil {
		return err
	}

	result, err := db.Import(conn, statement)
	if err != nil {
		return err
	}

	log.Printf("%s: provider %s account %s: %d records imported, %d already present",
		path, result.Provider, result.AccountID, result.Inserted, len(result.Duplicates))
	for _, duplicate := range result.Duplicates {
		log.Printf("  duplicate: %s", duplicate)
	}

	return nil
}

// openDB connects to the configured database, creating the schema if needed.
func openDB() (*db.Conn, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return db.Open(cfg.DBPath)
}
