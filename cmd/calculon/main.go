// Command calculon imports broker statements and shows the resulting portfolio
// in fullscreen terminal dashboards, locally or over SSH.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mgierada/calculon/internal/auth"
	"github.com/mgierada/calculon/internal/config"
	"github.com/mgierada/calculon/internal/data_processing/parsers"
	"github.com/mgierada/calculon/internal/db"
	"github.com/mgierada/calculon/internal/earnings"
	"github.com/mgierada/calculon/internal/finimpulse"
	"github.com/mgierada/calculon/internal/marketdata"
	"github.com/mgierada/calculon/internal/model"
	"github.com/mgierada/calculon/internal/news"
	"github.com/mgierada/calculon/internal/portfolio"
	"github.com/mgierada/calculon/internal/recommendations"
	"github.com/mgierada/calculon/internal/server"
	"github.com/mgierada/calculon/internal/ui"
	"github.com/mgierada/calculon/internal/ui/dashboards"
)

const usage = `calculon - portfolio dashboards

usage:
  calculon [ui] [--user NAME]                       open the dashboards fullscreen
  calculon serve                                    serve the dashboards over ssh
  calculon import [--user NAME] [--provider P] PATH...
                                                    import statements; directories
                                                    are searched recursively
  calculon user add NAME [--key FILE]               create a user, optionally with
                                                    an ssh public key
  calculon user key NAME FILE                       let another ssh key log in as NAME
  calculon user list                                list users
  calculon account list [--user NAME]               list a user's accounts
  calculon account rename [--user NAME] ID NAME     label an account, e.g. IKE
  calculon earnings backfill --from DATE [--to DATE] (--all | --symbol S...)
                                                    fetch earnings history of
                                                    held symbols, e.g. --from 2016-01-01
  calculon recommendations backfill --from DATE [--to DATE] (--all | --symbol S...)
                                                    fetch analyst recommendations
                                                    of held symbols

--user defaults to CALCULON_USER, or to the only user when there is just one.
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
		err = runUI(args)
	case "serve":
		err = runServe()
	case "import":
		err = runImport(args)
	case "user":
		err = runUser(args)
	case "account":
		err = runAccount(args)
	case "earnings":
		err = runEarnings(args)
	case "recommendations":
		err = runRecommendations(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		err = fmt.Errorf("unknown command %q\n\n%s", command, usage)
	}

	if err != nil {
		log.Fatal(err)
	}
}

// runUI opens the dashboards for one user in this terminal.
func runUI(args []string) error {
	flags := flag.NewFlagSet("ui", flag.ContinueOnError)
	userName := flags.String("user", "", "user whose portfolio to show")
	if _, err := parseInterspersed(flags, args); err != nil {
		return err
	}

	env, err := openEnv()
	if err != nil {
		return err
	}
	defer env.conn.Close()

	user, err := resolveUser(env, *userName)
	if err != nil {
		return err
	}
	load := func(scope *model.AccountKey) (portfolio.Report, error) {
		return portfolio.Load(env.conn, user, env.report, scope)
	}

	// Failures go to the footer only: logging would draw over the dashboards.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	poller := newPoller(env, nil)
	updates, unsubscribe := poller.Subscribe()
	defer unsubscribe()
	go poller.Run(ctx)

	splash := ui.Splash{User: user.Name, MinDuration: ui.DefaultSplashDuration}
	app := ui.NewApp(dashboards.All(refreshers(env)), load)
	return ui.Run(app.WithSplash(splash).WithPrices(updates))
}

// runServe serves the dashboards over SSH until interrupted.
func runServe() error {
	env, err := openEnv()
	if err != nil {
		return err
	}
	defer env.conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	poller := newPoller(env, log.Printf)
	go poller.Run(ctx)

	return server.Run(ctx, server.Options{
		Addr:        env.cfg.CalculonConfig.SSHAddr,
		HostKeyPath: env.cfg.CalculonConfig.SSHHostKey,
		Conn:        env.conn,
		Dashboards:  dashboards.All(refreshers(env)),
		Report:      env.report,
		Prices:      poller,
	})
}

// refreshers fetch what the API-backed dashboards show when r is pressed.
func refreshers(e env) dashboards.Refreshers {
	return dashboards.Refreshers{
		News: newsRefresher(e), Earnings: earningsRefresher(e),
		Recommendations: recommendationsRefresher(e),
	}
}

// recommendationsRefresher fetches the shown stock's analyst recommendations
// over the configured lookback, page by page.
func recommendationsRefresher(e env) ui.Refresher {
	fetcher := newRecommendationsFetcher(e)
	return func(ctx context.Context, _ *portfolio.Report, symbol string,
		progress func(ui.Progress)) (string, error) {
		if symbol == "" {
			return "", errors.New("pick a stock with p first")
		}
		progress(ui.Progress{Total: 1, Current: symbol})
		result, err := fetcher.FetchRecent(ctx, symbol, func(p recommendations.Page) {
			progress(ui.Progress{Done: p.Items, Total: max(p.Total, p.Items), Current: symbol})
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("recommendations: %s, %d months", symbol, result.Items), nil
	}
}

// earningsRefresher fetches the shown stock's earnings over the configured
// lookback, page by page.
func earningsRefresher(e env) ui.Refresher {
	fetcher := newEarningsFetcher(e)
	return func(ctx context.Context, _ *portfolio.Report, symbol string,
		progress func(ui.Progress)) (string, error) {
		if symbol == "" {
			return "", errors.New("pick a stock with p first")
		}
		progress(ui.Progress{Total: 1, Current: symbol})
		result, err := fetcher.FetchRecent(ctx, symbol, func(p earnings.Page) {
			progress(ui.Progress{Done: p.Items, Total: max(p.Total, p.Items), Current: symbol})
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("earnings: %s, %d items", symbol, result.Items), nil
	}
}

// newsRefresher fetches news about the report's top holdings from finimpulse,
// reporting each symbol as it goes.
func newsRefresher(e env) ui.Refresher {
	cfg := e.cfg.FinimpulseConfig
	fetcher := news.NewFetcher(e.conn, finimpulse.New(cfg.BaseURL, cfg.Token),
		cfg.NewsPerSymbol, cfg.NewsLookback)
	return func(ctx context.Context, report *portfolio.Report, _ string,
		progress func(ui.Progress)) (string, error) {
		result, err := fetcher.Fetch(ctx, report.NewsSymbols, func(step news.Step) {
			progress(newsProgress(step))
		})
		summary := fmt.Sprintf("news: %d new", result.Added)
		if len(result.Failed) > 0 {
			summary += fmt.Sprintf(", %d failed", len(result.Failed))
		}
		return summary, err
	}
}

// newsProgress describes a news fetch step for the progress box.
func newsProgress(step news.Step) ui.Progress {
	p := ui.Progress{Done: step.Done, Total: step.Total, Current: step.Current}
	if f := step.Finished; f != nil {
		p.Finished, p.Detail = f.Symbol, fmt.Sprintf("%d new", f.Added)
		if f.Err != nil {
			p.Detail, p.Failed = f.Err.Error(), true
		}
	}
	return p
}

// newPoller keeps held symbols priced from finimpulse; logf receives failures.
func newPoller(e env, logf func(format string, args ...any)) *marketdata.Poller {
	cfg := e.cfg.FinimpulseConfig
	client := finimpulse.New(cfg.BaseURL, cfg.Token)
	return marketdata.NewPoller(e.conn, client, cfg.PollInterval, logf)
}

// runImport parses each statement and stores it for a user.
func runImport(args []string) error {
	flags := flag.NewFlagSet("import", flag.ContinueOnError)
	userName := flags.String("user", "", "user the statements belong to")
	providerName := flags.String("provider", "",
		"provider to parse with, one of: "+strings.Join(parsers.Providers(), ", "))
	paths, err := parseInterspersed(flags, args)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("import needs at least one statement file or directory\n\n%s", usage)
	}

	files, err := parsers.Expand(paths)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no statements found in %s", strings.Join(paths, ", "))
	}

	env, err := openEnv()
	if err != nil {
		return err
	}
	defer env.conn.Close()

	user, err := resolveUser(env, *userName)
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := importFile(env.conn, user, file, model.Provider(*providerName)); err != nil {
			return err
		}
	}
	return nil
}

// importFile imports a single statement and logs what changed.
func importFile(conn *db.Conn, user db.User, path string, provider model.Provider) error {
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

	result, err := db.Import(conn, user.ID, statement)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	snapshot := "older than the stored snapshot, kept the stored one"
	if result.SnapshotReplaced {
		snapshot = fmt.Sprintf("%d open lots as of %s", result.OpenLots,
			statement.AsOf.Format("2006-01-02 15:04 MST"))
	}
	log.Printf("%s\n  account  %s %s (%s) for %s\n  closed   %s\n  cash     %s\n  quotes   %s\n  snapshot %s",
		path, result.Account.Provider, result.Account.ID, result.Account.Currency, user.Name,
		result.Positions, result.CashOps, result.Quotes, snapshot)
	return nil
}

// runUser manages users and their SSH keys.
func runUser(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("user needs a subcommand: add, key or list\n\n%s", usage)
	}
	subcommand, args := args[0], args[1:]

	env, err := openEnv()
	if err != nil {
		return err
	}
	defer env.conn.Close()

	switch subcommand {
	case "add":
		return runUserAdd(env.conn, args)
	case "key":
		if len(args) != 2 {
			return fmt.Errorf("usage: calculon user key NAME FILE")
		}
		user, err := db.UserByName(env.conn, args[0])
		if err != nil {
			return err
		}
		return addKey(env.conn, user, args[1])
	case "list":
		return listUsers(env.conn)
	default:
		return fmt.Errorf("unknown user subcommand %q\n\n%s", subcommand, usage)
	}
}

func runUserAdd(conn *db.Conn, args []string) error {
	flags := flag.NewFlagSet("user add", flag.ContinueOnError)
	keyPath := flags.String("key", "", "ssh public key file the user logs in with")
	names, err := parseInterspersed(flags, args)
	if err != nil {
		return err
	}
	if len(names) != 1 {
		return fmt.Errorf("usage: calculon user add NAME [--key FILE]")
	}

	user, err := db.CreateUser(conn, names[0])
	if err != nil {
		return err
	}
	log.Printf("created user %s", user.Name)
	if *keyPath == "" {
		return nil
	}
	return addKey(conn, user, *keyPath)
}

func addKey(conn *db.Conn, user db.User, path string) error {
	text, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}
	key, err := auth.ParseAuthorizedKey(string(text))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := db.AddUserKey(conn, user.ID, key); err != nil {
		return err
	}
	log.Printf("key %s can now log in as %s", key.Fingerprint, user.Name)
	return nil
}

func listUsers(conn *db.Conn) error {
	users, err := db.Users(conn)
	if err != nil {
		return err
	}
	for _, user := range users {
		fmt.Printf("%s\t%d keys\n", user.Name, user.Keys)
	}
	return nil
}

// runAccount lists or renames a user's accounts.
func runAccount(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("account needs a subcommand: list or rename\n\n%s", usage)
	}
	subcommand, args := args[0], args[1:]

	flags := flag.NewFlagSet("account "+subcommand, flag.ContinueOnError)
	userName := flags.String("user", "", "user whose accounts to manage")
	provider := flags.String("provider", string(model.ProviderXTB), "provider of the account")
	positional, err := parseInterspersed(flags, args)
	if err != nil {
		return err
	}

	env, err := openEnv()
	if err != nil {
		return err
	}
	defer env.conn.Close()

	user, err := resolveUser(env, *userName)
	if err != nil {
		return err
	}

	switch subcommand {
	case "list":
		accounts, err := db.Accounts(env.conn, user.ID)
		if err != nil {
			return err
		}
		for _, a := range accounts {
			fmt.Printf("%s\t%s\t%s\t%s\n", a.Provider, a.ID, a.Currency, a.Name)
		}
		return nil
	case "rename":
		if len(positional) != 2 {
			return fmt.Errorf("usage: calculon account rename [--user NAME] ID NAME")
		}
		if err := db.RenameAccount(env.conn, user.ID, model.Provider(*provider),
			positional[0], positional[1]); err != nil {
			return err
		}
		log.Printf("account %s is now %q", positional[0], positional[1])
		return nil
	default:
		return fmt.Errorf("unknown account subcommand %q\n\n%s", subcommand, usage)
	}
}

// env is what every command needs: configuration, the database and the
// sources reports are valued with.
type env struct {
	cfg    config.Config
	conn   *db.Conn
	report portfolio.Options
}

func openEnv() (env, error) {
	cfg, err := config.Load()
	if err != nil {
		return env{}, err
	}
	rates, err := portfolio.ParseRates(cfg.CalculonConfig.FXRates)
	if err != nil {
		return env{}, fmt.Errorf("FX_RATES: %w", err)
	}
	conn, err := db.Open(cfg.DBConfig.DBPath)
	if err != nil {
		return env{}, err
	}
	return env{
		cfg:  cfg,
		conn: conn,
		report: portfolio.Options{
			FX: portfolio.NewStaticFX(cfg.CalculonConfig.BaseCurrency, rates),
			News: portfolio.NewsOptions{
				Positions: cfg.FinimpulseConfig.NewsPositions,
				PerSymbol: cfg.FinimpulseConfig.NewsPerSymbol,
			},
			EarningsMethodology: cfg.FinimpulseConfig.EarningsMethodology,
		},
	}, nil
}

// resolveUser picks who a local command acts as: the flag, then CALCULON_USER,
// then the only user when there is exactly one.
func resolveUser(e env, name string) (db.User, error) {
	if name == "" {
		name = e.cfg.CalculonConfig.User
	}
	if name != "" {
		return db.UserByName(e.conn, name)
	}

	users, err := db.Users(e.conn)
	if err != nil {
		return db.User{}, err
	}
	switch len(users) {
	case 0:
		return db.User{}, errors.New("no users yet, create one with `calculon user add NAME`")
	case 1:
		return users[0], nil
	default:
		return db.User{}, errors.New("several users exist, pass --user or set CALCULON_USER")
	}
}

// parseInterspersed parses flags wherever they appear among positional
// arguments, which the flag package alone stops at, and returns the positionals.
func parseInterspersed(flags *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := flags.Parse(args); err != nil {
			return nil, err
		}
		if flags.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, flags.Arg(0))
		args = flags.Args()[1:]
	}
}

// runEarnings backfills earnings history from the API.
func runEarnings(args []string) error {
	return runBackfill("earnings", args, func(e env) backfillFetch {
		fetcher := newEarningsFetcher(e)
		return func(ctx context.Context, symbol string, from, to time.Time) (int, int, error) {
			result, err := fetcher.Fetch(ctx, symbol, from, to, func(p earnings.Page) {
				log.Printf("%s  page %d: %d of %d items", symbol, p.Number, p.Items, p.Total)
			})
			return result.Items, result.Pages, err
		}
	})
}

// runRecommendations backfills analyst recommendations from the API.
func runRecommendations(args []string) error {
	return runBackfill("recommendations", args, func(e env) backfillFetch {
		fetcher := newRecommendationsFetcher(e)
		return func(ctx context.Context, symbol string, from, to time.Time) (int, int, error) {
			result, err := fetcher.Fetch(ctx, symbol, from, to, func(p recommendations.Page) {
				log.Printf("%s  page %d: %d of %d items", symbol, p.Number, p.Items, p.Total)
			})
			return result.Items, result.Pages, err
		}
	})
}

// backfillFetch stores one symbol's data between from and to, returning how
// many items it stored in how many pages.
type backfillFetch func(ctx context.Context, symbol string, from, to time.Time) (int, int, error)

// runBackfill parses a backfill subcommand of command and fetches each symbol
// in turn, logging failures and carrying on.
func runBackfill(command string, args []string, newFetch func(env) backfillFetch) error {
	if len(args) == 0 || args[0] != "backfill" {
		return fmt.Errorf("%s needs a subcommand: backfill\n\n%s", command, usage)
	}
	flags := flag.NewFlagSet(command+" backfill", flag.ContinueOnError)
	from := flags.String("from", "", "first period to fetch, YYYY-MM-DD")
	to := flags.String("to", time.Now().Format(time.DateOnly), "last period to fetch, YYYY-MM-DD")
	all := flags.Bool("all", false, "every held symbol")
	var symbols symbolList
	flags.Var(&symbols, "symbol", "symbol to fetch, e.g. EQIX.US; repeatable")
	if _, err := parseInterspersed(flags, args[1:]); err != nil {
		return err
	}
	start, end, err := backfillRange(command, *from, *to)
	if err != nil {
		return err
	}
	if *all == (len(symbols) > 0) {
		return fmt.Errorf("%s backfill needs either --all or --symbol", command)
	}

	env, err := openEnv()
	if err != nil {
		return err
	}
	defer env.conn.Close()
	if *all {
		if symbols, err = db.HeldSymbols(env.conn); err != nil {
			return err
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fetch := newFetch(env)
	for _, symbol := range symbols {
		items, pages, err := fetch(ctx, symbol, start, end)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			log.Printf("%s  failed: %v", symbol, err)
			continue
		}
		log.Printf("%s  stored %d items in %d pages", symbol, items, pages)
	}
	return nil
}

// backfillRange parses the backfill's dates.
func backfillRange(command, from, to string) (time.Time, time.Time, error) {
	if from == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("%s backfill needs --from YYYY-MM-DD", command)
	}
	start, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--from: %w", err)
	}
	end, err := time.Parse(time.DateOnly, to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--to: %w", err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, errors.New("--to is before --from")
	}
	return start, end, nil
}

// symbolList is a repeatable --symbol flag.
type symbolList []string

func (s *symbolList) String() string { return strings.Join(*s, ",") }

func (s *symbolList) Set(symbol string) error {
	*s = append(*s, symbol)
	return nil
}

// newEarningsFetcher fetches earnings from finimpulse as configured.
func newEarningsFetcher(e env) *earnings.Fetcher {
	cfg := e.cfg.FinimpulseConfig
	return earnings.NewFetcher(e.conn, finimpulse.New(cfg.BaseURL, cfg.Token), earnings.Options{
		Types: cfg.EarningsTypes, Methodology: cfg.EarningsMethodology,
		Lookback: cfg.EarningsLookback, PageSize: cfg.EarningsPage,
	})
}

// newRecommendationsFetcher fetches analyst recommendations from finimpulse as
// configured.
func newRecommendationsFetcher(e env) *recommendations.Fetcher {
	cfg := e.cfg.FinimpulseConfig
	return recommendations.NewFetcher(e.conn, finimpulse.New(cfg.BaseURL, cfg.Token),
		recommendations.Options{Lookback: cfg.RecommendationsLookback, PageSize: cfg.RecommendationsPage})
}
