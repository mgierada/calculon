package config

type FinimpulseConfig struct {
	BaseURL string `env:"FINIMPULSE_BASE_URL" env-required:"true"`
	Token   string `env:"FINIMPULSE_TOKEN" env-required:"true"`
}
