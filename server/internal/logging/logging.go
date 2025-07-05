package logging

import (
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"io"
	"log/slog"
	"os"
)

func SetUpLogger(config config.Logger) {
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.Level,
	})

	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)
}

func LoggedClose(closer io.Closer) {
	if err := closer.Close(); err != nil {
		slog.Error("encountered error while trying to close a resource", "error", err)
	}
}
