package config

import "time"

type FinimpulseConfig struct {
	BaseURL string `env:"FINIMPULSE_BASE_URL" env-required:"true"`
	Token   string `env:"FINIMPULSE_TOKEN" env-required:"true"`
	// PollInterval is how often held symbols are priced, and how old the
	// stored prices may be before a start fetches them again.
	PollInterval time.Duration `env:"FINIMPULSE_POLL_INTERVAL" env-default:"15m"`
	// NewsPositions is how many of the most valuable holdings news is
	// fetched and shown for, and NewsPerSymbol how many articles each.
	NewsPositions int `env:"FINIMPULSE_NEWS_POSITIONS" env-default:"5"`
	NewsPerSymbol int `env:"FINIMPULSE_NEWS_PER_SYMBOL" env-default:"5"`
	// NewsLookback is how far back news is searched for a symbol without any
	// stored; later fetches start from the newest stored article.
	NewsLookback time.Duration `env:"FINIMPULSE_NEWS_LOOKBACK" env-default:"720h"`
	// Earnings requests ask for these item types in this methodology (empty
	// for all), EarningsPage items a page. EarningsLookback is how far back
	// the dashboard's r reaches; a backfill sets its own start.
	EarningsTypes       []string      `env:"FINIMPULSE_EARNINGS_TYPES" env-default:"eps_actual,earnings_revenue,growth" env-separator:","`
	EarningsMethodology string        `env:"FINIMPULSE_EARNINGS_METHODOLOGY" env-default:"gaap"`
	EarningsLookback    time.Duration `env:"FINIMPULSE_EARNINGS_LOOKBACK" env-default:"17520h"`
	EarningsPage        int           `env:"FINIMPULSE_EARNINGS_PAGE" env-default:"50"`
	// Recommendation requests ask for RecommendationsPage months a page.
	// RecommendationsLookback is how far back the dashboard's r reaches; a
	// backfill sets its own start.
	RecommendationsLookback time.Duration `env:"FINIMPULSE_RECOMMENDATIONS_LOOKBACK" env-default:"17520h"`
	RecommendationsPage     int           `env:"FINIMPULSE_RECOMMENDATIONS_PAGE" env-default:"50"`
}
