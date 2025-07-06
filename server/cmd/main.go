package main

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	setup "github.com/mat-sik/sql-distributed-transactions/common/otel"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/logging"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/server"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/transaction"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	collectorConfig := config.NewCollectorConfig(ctx)
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
	meter := otel.Meter(instrumentationScope)

	logger := otelslog.NewLogger(instrumentationScope)
	slog.SetDefault(logger)

	pool, err := getDBPool(ctx)
	if err != nil {
		slog.Error("Failed to initialize the pool", "err", err)
		return
	}
	defer logging.LoggedClose(pool)

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	executorConfig := config.NewExecutorConfig(ctx)
	go func() {
		slog.Info("starting the executor", "config", executorConfig)
		e := transaction.NewExecutor(pool, client, executorConfig)
		e.Start(ctx)
	}()

	runServer(ctx, tracer, meter, pool)
}

func getDBPool(ctx context.Context) (*sql.DB, error) {
	databaseConfig := config.NewDatabaseConfig(ctx)

	pool, err := sql.Open("pgx/v5", databaseConfig.URL)
	if err != nil {
		return nil, err
	}

	if err = transaction.CreateTransactionsTableIfNotExist(ctx, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

func runServer(ctx context.Context, tracer trace.Tracer, meter metric.Meter, pool *sql.DB) {
	handler := server.NewHandler(pool)
	serverConfig := config.NewServer(ctx)
	srv := server.NewServer(serverConfig, handler)

	serverErrCh := make(chan error)
	go func() {
		slog.Info("starting the server", "config", serverConfig)
		serverErrCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErrCh:
		slog.Error("Received server error", "err", err)
	case <-ctx.Done():
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)
		if err != nil {
			slog.Error("Server shutdown failed", "err", err)
		}
		slog.Info("Server shutdown complete")
	}
}

const (
	instrumentationScope = "github.com/mat-sik/sql-distributed-transactions/server"
	serviceName          = "server"
)
