package config

import (
	"os"
	"path/filepath"
	"testing"
)

// chdirWithEnvFile runs the test from a directory holding the given .env.
func chdirWithEnvFile(t *testing.T, contents string) {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(contents), 0o600); err != nil {
		t.Fatalf("writing .env: %v", err)
	}
	t.Chdir(dir)
}

// Regression: .env used to overwrite variables already set in the environment.
func TestLoadPrefersEnvironmentOverEnvFile(t *testing.T) {
	chdirWithEnvFile(t, "DB_PATH=from-file.db\nBASE_CURRENCY=EUR\n")
	t.Setenv("FINIMPULSE_BASE_URL", "https://api.example.test")
	t.Setenv("DB_PATH", "from-env.db")
	t.Setenv("BASE_CURRENCY", "")
	os.Unsetenv("BASE_CURRENCY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.DBConfig.DBPath != "from-env.db" {
		t.Errorf("DBPath = %q, want the environment's value", cfg.DBConfig.DBPath)
	}
	if cfg.CalculonConfig.BaseCurrency != "EUR" {
		t.Errorf("BaseCurrency = %q, want the file's value for an unset variable",
			cfg.CalculonConfig.BaseCurrency)
	}
}

func TestLoadWithoutEnvFileAppliesDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	os.Unsetenv("DB_PATH")
	t.Setenv("SSH_ADDR", ":2222")
	t.Setenv("FINIMPULSE_BASE_URL", "https://api.example.test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.DBConfig.DBPath != "./calculon.db" || cfg.CalculonConfig.SSHAddr != ":2222" ||
		cfg.CalculonConfig.BaseCurrency != "PLN" {
		t.Errorf("cfg = %+v", cfg)
	}
}

// Every group is read, nested structs included.
func TestLoadFillsEveryGroup(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("FINIMPULSE_BASE_URL", "https://api.example.test")
	t.Setenv("FINIMPULSE_TOKEN", "secret")
	t.Setenv("CALCULON_USER", "maciej")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FinimpulseConfig.BaseURL != "https://api.example.test" || cfg.FinimpulseConfig.Token != "secret" {
		t.Errorf("finimpulse config = %+v", cfg.FinimpulseConfig)
	}
	if cfg.CalculonConfig.User != "maciej" {
		t.Errorf("calculon user = %q", cfg.CalculonConfig.User)
	}
}

func TestLoadRequiresFinimpulseBaseURL(t *testing.T) {
	t.Chdir(t.TempDir())
	os.Unsetenv("FINIMPULSE_BASE_URL")

	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded without FINIMPULSE_BASE_URL")
	}
}
