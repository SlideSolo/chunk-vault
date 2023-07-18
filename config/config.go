package config

import (
	"context"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	App App `env:",prefix=APP_"`
	Log Log `env:",prefix=LOG_"`
}

type App struct {
	ServiceName       string `env:"SERVICE_NAME,default=chunk-vault"`
	NumStorageServers int    `env:"NUM_STORAGE_SERVERS,default=6"`
	PromServeAddr     string `env:"PROM_SERVE_ADDR,default=:8081"`
}

type Log struct {
	Level    string `env:"LEVEL,default=info"`
	Format   string `env:"FORMAT,default=json"`
	FilePath string `env:"FILE"`
}

func New(ctx context.Context) (*Config, error) {
	var cfg Config

	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
