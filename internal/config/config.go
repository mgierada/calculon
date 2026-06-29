package config

import (
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config holds application settings loaded from the environment and an optional
// .env file.
type Config struct {
	DBPath string `env:"DB_PATH" env-default:"./calculon.db"`
}

// Load reads configuration from .env when present, otherwise from the process
// environment, applying defaults and validating required fields.
func Load() (Config, error) {
	var cfg Config

	if _, err := os.Stat(".env"); err == nil {
		if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
