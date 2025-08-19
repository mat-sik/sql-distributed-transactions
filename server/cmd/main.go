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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	collectorConfig, err := config.NewCollectorConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize the collector config", "err", err)
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
	meter := otel.Meter(instrumentationScope)

	logger := otelslog.NewLogger(instrumentationScope)
	slog.SetDefault(logger)

	databaseConfig, err := config.NewDatabaseConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize the database config", "err", err)
		return
	}

	pool, err := newDBPool(ctx, databaseConfig)
	if err != nil {
		slog.Error("Failed to initialize the pool", "err", err)
		return
	}
	defer logging.LoggedClose(pool)

	repository := transaction.NewSQLRepository(pool)

	client := &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	executorConfig, err := config.NewExecutorConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize the executor configuration", "err", err)
		return
	}

	go func() {
		slog.Info("starting the executor", "config", executorConfig)
		e := transaction.NewExecutor(tracer, meter, repository, client, executorConfig)
		e.Start(ctx)
	}()

	serverConfig, err := config.NewServer(ctx)
	if err != nil {
		slog.Error("Failed to initialize the server config", "err", err)
		return
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
	case <-ctx.Done():
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err = srv.Shutdown(shutdownCtx)
		if err != nil {
			slog.Error("Server shutdown failed", "err", err)
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
