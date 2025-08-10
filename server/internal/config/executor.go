package config

import (
	"context"
	"github.com/sethvargo/go-envconfig"
	"time"
)

type Executor struct {
	ExecuteTransactionInterval time.Duration `env:"SERVER_EXECUTOR_TRANSACTION_INTERVAL, default=1s"`
	WorkerAmount               int           `env:"SERVER_EXECUTOR_WORKER_AMOUNT, default=2"`
	BatchSize                  int           `env:"SERVER_EXECUTOR_BATCH_SIZE, default=400"`
	SenderAmount               int           `env:"SERVER_EXECUTOR_SENDER_AMOUNT, default=2"`
}

func NewExecutorConfig(ctx context.Context) (Executor, error) {
	var config Executor
	if err := envconfig.Process(ctx, &config); err != nil {
		return config, err
	}

	return config, nil
}
