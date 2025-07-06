package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
)

type Server struct {
	Port int `env:"SERVER_PORT, default=40690"`
}

func NewServer(ctx context.Context) (Server, error) {
	var config Server
	if err := envconfig.Process(ctx, &config); err != nil {
		return Server{}, err
	}

	return config, nil
}
