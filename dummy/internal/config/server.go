package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
)

type Server struct {
	ToReceive int `env:"DUMMY_TO_RECEIVE, default=100_000"`
}

func NewServerConfig(ctx context.Context) Server {
	var config Server
	if err := envconfig.Process(ctx, &config); err != nil {
		panic(err)
	}

	return config
}
