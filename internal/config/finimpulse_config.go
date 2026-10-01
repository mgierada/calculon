package config

import "time"

type FinimpulseConfig struct {
	BaseURL string `env:"FINIMPULSE_BASE_URL" env-required:"true"`
	Token   string `env:"FINIMPULSE_TOKEN" env-required:"true"`
	// PollInterval is how often held symbols are priced, and how old the
	// stored prices may be before a start fetches them again.
	PollInterval time.Duration `env:"FINIMPULSE_POLL_INTERVAL" env-default:"15m"`
}
