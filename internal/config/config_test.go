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
	t.Setenv("DB_PATH", "from-env.db")
	t.Setenv("BASE_CURRENCY", "")
	os.Unsetenv("BASE_CURRENCY")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.DBPath != "from-env.db" {
		t.Errorf("DBPath = %q, want the environment's value", cfg.DBPath)
	}
	if cfg.BaseCurrency != "EUR" {
		t.Errorf("BaseCurrency = %q, want the file's value for an unset variable", cfg.BaseCurrency)
	}
}

func TestLoadWithoutEnvFileAppliesDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	os.Unsetenv("DB_PATH")
	t.Setenv("SSH_ADDR", ":2222")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.DBPath != "./calculon.db" || cfg.SSHAddr != ":2222" || cfg.BaseCurrency != "PLN" {
		t.Errorf("cfg = %+v", cfg)
	}
}
