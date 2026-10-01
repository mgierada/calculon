package config

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Config is every setting, grouped by what it configures. Each group lives in
// its own file.
type Config struct {
	DBConfig         DBConfig
	CalculonConfig   CalculonConfig
	FinimpulseConfig FinimpulseConfig
}

// Load reads the full configuration from the environment, filling anything
// unset from .env when present. Real environment variables win, so a
// deployment or a one-off `DB_PATH=... calculon` can override the file. It
// fails on the first missing required variable so the server never starts
// half-configured.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to read .env: %w", err)
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
