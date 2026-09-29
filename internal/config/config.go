package config

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Config holds application settings loaded from the environment and an optional
// .env file.
type Config struct {
	DBPath string `env:"DB_PATH" env-default:"./calculon.db"`
	// User is who the local dashboard and imports act as when --user is not
	// given. It may stay empty while the database holds a single user.
	User string `env:"CALCULON_USER"`

	SSHAddr    string `env:"SSH_ADDR" env-default:":23234"`
	SSHHostKey string `env:"SSH_HOST_KEY" env-default:"./.ssh/calculon_ed25519"`

	BaseCurrency string `env:"BASE_CURRENCY" env-default:"PLN"`
	// FXRates values foreign currencies in the base one, e.g. "USD=3.65,EUR=4.26".
	FXRates string `env:"FX_RATES"`
}

// Load reads configuration from the process environment, filling anything
// unset from .env when present, then applies defaults. Real environment
// variables win, so a deployment or a one-off `DB_PATH=... calculon` can
// override the file.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to read .env: %w", err)
	}

	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
