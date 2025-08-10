package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
)

type Client struct {
	DummyHost   string `env:"DUMMY_HOST, default=localhost:40691"`
	ToSend      int    `env:"CLIENT_TO_SEND, default=100_000"`
	WorkerCount int    `env:"CLIENT_WORKER_COUNT, default=2"`
}

func NewClientConfig(ctx context.Context) (Client, error) {
	var config Client
	if err := envconfig.Process(ctx, &config); err != nil {
		return config, err
	}

	return config, nil
}
