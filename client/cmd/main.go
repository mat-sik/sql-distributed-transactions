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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

	tracer := otel.Tracer(instrumentationScope)

	logger := otelslog.NewLogger(instrumentationScope)
	slog.SetDefault(logger)

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	clientConfig, err := config.NewClientConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize client config", "err", err)
		return
	}

	runClient(ctx, tracer, clientConfig, client)
}

func runClient(ctx context.Context, tracer trace.Tracer, clientConfig config.Client, client *http.Client) {
	tClient := api.NewClient(ctx, client)
	slog.Info("creating client", "config", clientConfig)

	chSize := min(clientConfig.ToSend, maxChannelSize)
	toSendCh := make(chan commons.EnqueueTransactionRequest, chSize)

	wg := sync.WaitGroup{}
	for i := 0; i < clientConfig.WorkerCount; i++ {
		wg.Add(1)
		go sendAll(ctx, tracer, &wg, tClient, toSendCh)
	}

	for i := 0; i < clientConfig.ToSend; i++ {
		req := commons.EnqueueTransactionRequest{
			Host:    clientConfig.DummyHost,
			Path:    fmt.Sprintf("/call/%d", i),
			Method:  "POST",
			Payload: fmt.Sprintf(`{"iteration": %d}`, i),
		}
		select {
		case <-ctx.Done():
			close(toSendCh)
			return
		case toSendCh <- req:
		}
	}
	close(toSendCh)

	wg.Wait()
}

func sendAll(ctx context.Context, tracer trace.Tracer, wg *sync.WaitGroup, tClient api.Client, toSendCh chan commons.EnqueueTransactionRequest) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case req, ok := <-toSendCh:
			if !ok {
				return
			}
			send(ctx, tracer, tClient, req)
		}
	}
}

func send(ctx context.Context, tracer trace.Tracer, tClient api.Client, req commons.EnqueueTransactionRequest) {
	ctx, span := tracer.Start(ctx, "send")
	defer span.End()

	operation := func() (struct{}, error) {
		return struct{}{}, tClient.EnqueueTransaction(ctx, req)
	}

	span.AddEvent("Trying to send transaction request", trace.WithAttributes(
		attribute.String("transaction request payload", req.Payload),
	))

	_, err := backoff.Retry(ctx, operation, backoff.WithBackOff(backoff.NewExponentialBackOff()), backoff.WithMaxTries(5))
	if err != nil {
		span.SetStatus(codes.Error, "Failed to enqueue transaction")
		span.RecordError(err, trace.WithAttributes(
			attribute.String("request payload", req.Payload),
		))
	}
}

const (
	instrumentationScope = "github.com/mat-sik/sql-distributed-transactions/client"
	serviceName          = "client"
	maxChannelSize       = 10_240
)
