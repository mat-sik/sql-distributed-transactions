package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	setup "github.com/mat-sik/sql-distributed-transactions/common/otel"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/logging"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/server"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/transaction"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	collectorConfig, err := config.NewCollectorConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize the collector config", "err", err)
		panic(err)
	}

	shutdown, err := setup.InitOTelSDK(ctx, collectorConfig.CollectorHost, serviceName)
	if err != nil {
		slog.Error("Failed to initialize otel SDK", "err", err)
		panic(err)
	}
	defer func() {
		if err = shutdown(ctx); err != nil {
			slog.Error("Failed to shutdown otel SDK", "err", err)
			panic(err)
		}
	}()

	tracer := otel.Tracer(instrumentationScope)
	meter := otel.Meter(instrumentationScope)

	logger := otelslog.NewLogger(instrumentationScope)
	slog.SetDefault(logger)

	databaseConfig, err := config.NewDatabaseConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize the database config", "err", err)
		panic(err)
	}

	pool, err := newDBPool(ctx, databaseConfig)
	if err != nil {
		slog.Error("Failed to initialize the pool", "err", err)
		panic(err)
	}
	defer logging.LoggedClose(pool)

	repository := transaction.NewSQLRepository(pool)

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	executorConfig, err := config.NewExecutorConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize the executor configuration", "err", err)
		panic(err)
	}

	go func() {
		slog.Info("starting the executor", "config", executorConfig)
		e := transaction.NewExecutor(tracer, meter, repository, client, executorConfig)
		e.Start(ctx)
	}()

	serverConfig, err := config.NewServer(ctx)
	if err != nil {
		slog.Error("Failed to initialize the server config", "err", err)
		panic(err)
	}

	handler := server.NewHandler(tracer, repository)
	srv := server.NewServer(ctx, serverConfig, handler)

	serverErrCh := make(chan error)
	go func() {
		slog.Info("starting the server", "config", serverConfig)
		serverErrCh <- srv.ListenAndServe()
	}()

	select {
	case err = <-serverErrCh:
		slog.Error("Received server error", "err", err)
		panic(err)
	case <-ctx.Done():
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = srv.Shutdown(shutdownCtx)
		if err != nil {
			slog.Error("Server shutdown failed", "err", err)
			panic(err)
		}
		slog.Info("Server shutdown complete")
	}
}

func newDBPool(ctx context.Context, databaseConfig config.Database) (*sql.DB, error) {
	pool, err := sql.Open("pgx/v5", databaseConfig.URL)
	if err != nil {
		return nil, err
	}

	if err = transaction.CreateTransactionsTableIfNotExist(ctx, pool); err != nil {
		return nil, err
	}

	return pool, nil
}

const (
	instrumentationScope = "github.com/mat-sik/sql-distributed-transactions/server"
	serviceName          = "server"
)
