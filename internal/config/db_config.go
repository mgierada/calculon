package config

type DBConfig struct {
	DBPath string `env:"DB_PATH" env-default:"./calculon.db"`
}
