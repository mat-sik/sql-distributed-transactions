package main

import (
	"context"
	"fmt"
	"github.com/cenkalti/backoff/v5"
	api "github.com/mat-sik/sql-distributed-transactions/api/transaction"
	"github.com/mat-sik/sql-distributed-transactions/client/internal/config"
	setup "github.com/mat-sik/sql-distributed-transactions/common/otel"
	commons "github.com/mat-sik/sql-distributed-transactions/common/transaction"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	collectorConfig, err := config.NewCollectorConfig(ctx)
	if err != nil {
		slog.Error("failed to initialize collector config", "err", err)
		return
	}

	clientConfig, err := config.NewClientConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize client config", "err", err)
		return
	}

	shutdown, err := setup.InitOTelSDK(ctx, collectorConfig.CollectorHost, serviceName)
	if err != nil {
		slog.Error("Failed to initialize otel SDK", "err", err)
		return
	}
	defer func() {
		if err = shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown otel SDK", "err", err)
		}
	}()

	logger := otelslog.NewLogger(instrumentationScope)
	slog.SetDefault(logger)

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	runClient(ctx, clientConfig, client)
}

func runClient(ctx context.Context, clientConfig config.Client, client *http.Client) {
	tClient := api.NewClient(ctx, client)
	slog.Info("creating client", "config", clientConfig)

	toSendCh := make(chan commons.EnqueueTransactionRequest, clientConfig.ToSend)
	for i := 0; i < clientConfig.ToSend; i++ {
		req := commons.EnqueueTransactionRequest{
			Host:    clientConfig.DummyHost,
			Path:    fmt.Sprintf("/call/%d", i),
			Method:  "POST",
			Payload: fmt.Sprintf(`{"iteration": %d}`, i),
		}
		toSendCh <- req
	}
	close(toSendCh)

	wg := sync.WaitGroup{}
	for i := 0; i < clientConfig.WorkerCount; i++ {
		wg.Add(1)
		go sendAll(ctx, &wg, tClient, toSendCh)
	}
	wg.Wait()
}

func sendAll(ctx context.Context, wg *sync.WaitGroup, tClient api.Client, toSendCh chan commons.EnqueueTransactionRequest) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-toSendCh:
			if !ok {
				return
			}
			operation := func() (struct{}, error) {
				return struct{}{}, tClient.EnqueueTransaction(ctx, req)
			}

			_, err := backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()), backoff.WithMaxTries(5))
			if err != nil {
				slog.Error("Failed to enqueue transaction", "err", err)
			}
		}
	}
}

const (
	instrumentationScope = "github.com/mat-sik/sql-distributed-transactions/client"
	serviceName          = "client"
)
