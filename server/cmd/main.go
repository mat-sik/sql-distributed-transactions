package main

import (
	"context"
	"database/sql"
	"errors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/logging"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/server"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/transaction"
	"log/slog"
	"net/http"
)

func main() {
	ctx := context.Background()

	loggerConfig := config.NewLoggerConfig(ctx)
	logging.SetUpLogger(loggerConfig)

	transaction.RegisterMetrics()

	databaseConfig := config.NewDatabaseConfig(ctx)

	pool, err := sql.Open("pgx/v5", databaseConfig.URL)
	if err != nil {
		panic(err)
	}
	defer logging.LoggedClose(pool)

	if err = transaction.CreateTransactionsTableIfNotExist(ctx, pool); err != nil {
		panic(err)
	}

	client := &http.Client{}

	executorConfig := config.NewExecutorConfig(ctx)
	go func() {
		e := transaction.NewExecutor(pool, client, executorConfig)
		e.Start(ctx)
	}()

	handler := server.NewHandler(pool)
	serverConfig := config.NewServer(ctx)
	s := server.NewServer(serverConfig, handler)

	slog.Info("starting the server", "server config", serverConfig, "executor config", executorConfig)
	if err = s.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
