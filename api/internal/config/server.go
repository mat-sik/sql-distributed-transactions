package config

import (
	"context"
	"log"

	"github.com/sethvargo/go-envconfig"
)

type Server struct {
	URL string `env:"SERVER_URL, default=http://localhost:40690"`
}

func NewServer(ctx context.Context) Server {
	var config Server
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal(err)
	}

	return config
}
