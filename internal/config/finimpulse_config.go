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
}
