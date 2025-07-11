package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
)

type Database struct {
	URL string `env:"COORDINATOR_DB_URL, default=postgres://postgres:postgres@localhost:5432/coordinator"`
}

func NewDatabaseConfig(ctx context.Context) Database {
	var config Database
	if err := envconfig.Process(ctx, &config); err != nil {
		panic(err)
	}

	return config
}
