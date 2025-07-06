package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
)

type Collector struct {
	CollectorHost string `env:"DUMMY_COLLECTOR_HOST, default=localhost:4317"`
}

func NewCollectorConfig(ctx context.Context) (Collector, error) {
	var config Collector
	if err := envconfig.Process(ctx, &config); err != nil {
		return config, err
	}

	return config, nil
}
