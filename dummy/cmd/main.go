package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	setup "github.com/mat-sik/sql-distributed-transactions/common/otel"
	"github.com/mat-sik/sql-distributed-transactions/dummy/internal/config"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	collectorConfig, err := config.NewCollectorConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize collector config", "err", err)
		return
	}

	serverConfig, err := config.NewServerConfig(ctx)
	if err != nil {
		slog.Error("Failed to initialize server config", "err", err)
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

	runServer(ctx, tracer, meter, serverConfig)
}

func runServer(ctx context.Context, tracer trace.Tracer, meter metric.Meter, serverConfig config.Server) {
	toCome := make(map[int]struct{}, serverConfig.ToReceive)
	for i := 0; i < serverConfig.ToReceive; i++ {
		toCome[i] = struct{}{}
	}

	handler := newHandler(tracer, meter, toCome)
	server := newServer(ctx, serverConfig.Port, handler)

	serverErrCh := make(chan error)
	go func() {
		slog.Info("starting the server", "config", serverConfig)
		serverErrCh <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrCh:
		slog.Error("Received server error", "err", err)
	case <-ctx.Done():
		slog.Info("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := server.Shutdown(shutdownCtx)
		if err != nil {
			slog.Error("Server shutdown failed", "err", err)
		}
		slog.Info("Server shutdown complete")
	}
}

func newHandler(tracer trace.Tracer, meter metric.Meter, toCome map[int]struct{}) http.Handler {
	mux := http.NewServeMux()

	handleFunc := func(pattern string, handler http.Handler) {
		handler = otelhttp.WithRouteTag(pattern, handler)
		mux.Handle(pattern, handler)
	}

	handleFunc("POST /", &counterHandler{
		tracer: tracer,
		meter:  meter,
		mutex:  &sync.Mutex{},
		toCome: toCome,
	})

	handler := otelhttp.NewHandler(mux, "/")
	return handler
}

type counterHandler struct {
	mutex  *sync.Mutex
	toCome map[int]struct{}
	tracer trace.Tracer
	meter  metric.Meter
}

func (h *counterHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	ctx, span := h.tracer.Start(ctx, "dummyHandler")
	defer span.End()

	var data struct {
		Iteration int `json:"iteration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		span.SetStatus(codes.Error, "Failed to unmarshal request body")
		span.RecordError(err)
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}
	h.executeTransaction(ctx, data.Iteration)
}

func (h *counterHandler) executeTransaction(ctx context.Context, transactionNumber int) {
	ctx, span := h.tracer.Start(ctx, "executeTransaction")
	defer span.End()

	span.AddEvent("Trying to obtain a lock")
	h.mutex.Lock()
	defer func() {
		span.AddEvent("Releasing the lock")
		h.mutex.Unlock()
	}()

	_, ok := h.toCome[transactionNumber]
	if !ok {
		err := errors.New("transaction has been already executed")
		span.SetStatus(codes.Error, "Transaction duplication")
		span.RecordError(err, trace.WithAttributes(
			attribute.Int("transaction number", transactionNumber),
		))
		return
	}
	delete(h.toCome, transactionNumber)

	span.AddEvent("Executed the transaction successfully", trace.WithAttributes(
		attribute.Int("transaction number", transactionNumber),
		attribute.Int("transactions yet to come", len(h.toCome)),
	))
}

func newServer(ctx context.Context, port int, handler http.Handler) http.Server {
	return http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Minute,
		Handler:      handler,
	}
}

const (
	instrumentationScope = "github.com/mat-sik/sql-distributed-transactions/dummy"
	serviceName          = "dummy"
)
