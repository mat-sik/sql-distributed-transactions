package config

import (
	"context"
	"fmt"
	"github.com/sethvargo/go-envconfig"
	"log"
	"log/slog"
	"strings"
)

type Logger struct {
	Level slog.Level
}

type logger struct {
	Level string `env:"DUMMY_LOGGING_LEVEL, default=info"`
}

func NewLoggerConfig(ctx context.Context) Logger {
	var config logger
	if err := envconfig.Process(ctx, &config); err != nil {
		log.Fatal(err)
	}

	return Logger{
		Level: parseLogLevel(config.Level),
	}
}

func parseLogLevel(levelStr string) slog.Level {
	switch strings.ToLower(levelStr) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		panic(fmt.Sprintf("invalid log level: %s", levelStr))
	}
}
