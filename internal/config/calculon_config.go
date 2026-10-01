package config

type CalculonConfig struct {
	User         string `env:"CALCULON_USER"`
	SSHAddr      string `env:"SSH_ADDR" env-default:":23234"`
	SSHHostKey   string `env:"SSH_HOST_KEY" env-default:"./.ssh/calculon_ed25519"`
	BaseCurrency string `env:"BASE_CURRENCY" env-default:"PLN"`
	// FXRates values foreign currencies in the base one, e.g. "USD=3.65,EUR=4.26".
	FXRates string `env:"FX_RATES"`
}
