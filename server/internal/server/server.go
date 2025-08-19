package server

import (
	"context"
	"fmt"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/transaction"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"net"
	"net/http"
	"time"
)

func NewServer(ctx context.Context, serverConfig config.Server, handler http.Handler) http.Server {
	return http.Server{
		Addr:         fmt.Sprintf(":%d", serverConfig.Port),
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Minute,
		Handler:      handler,
	}
}

func NewHandler(tracer trace.Tracer, repository transaction.Repository) http.Handler {
	mux := http.NewServeMux()

	handleFunc := func(pattern string, handler http.Handler) {
		handler = otelhttp.WithRouteTag(pattern, handler)
		mux.Handle(pattern, handler)
	}

	transactionHandler := transaction.NewHandler(tracer, repository)

	handleFunc("POST /transactions/enqueue", transactionHandler)

	handler := otelhttp.NewHandler(mux, "/")
	return handler
}
