package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
	"log"
)

type Server struct {
	Port int `env:"SERVER_PORT, default=40690"`
}

func NewServer(ctx context.Context) Server {
	var config Server
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal(err)
	}

	return config
}
