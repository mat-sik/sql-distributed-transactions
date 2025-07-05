package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mat-sik/sql-distributed-transactions/dummy/internal/config"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	ctx := context.Background()

	loggerConfig := config.NewLoggerConfig(ctx)
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: loggerConfig.Level,
	})

	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)

	serverConfig := config.NewServerConfig(ctx)

	toCome := make(map[int]struct{}, serverConfig.ToReceive)
	for i := 0; i < serverConfig.ToReceive; i++ {
		toCome[i] = struct{}{}
	}

	handler := http.NewServeMux()
	handler.Handle("POST /", &counterHandler{
		mutex:  &sync.Mutex{},
		toCome: toCome,
	})

	server := newServer(40691, handler)

	slog.Info("starting the server", "port", 40691, "config", serverConfig)
	if err := server.ListenAndServe(); err != nil {
		panic(err)
	}
}

type counterHandler struct {
	mutex  *sync.Mutex
	toCome map[int]struct{}
}

func (h *counterHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	var data struct {
		Iteration int `json:"iteration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		slog.Error("error unmarshalling request body", "err", err)
		return
	}
	h.delete(data.Iteration)
}

func (h *counterHandler) delete(i int) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	_, ok := h.toCome[i]
	if !ok {
		slog.Warn("counter i has been already deleted", "i", i)
		return
	}
	delete(h.toCome, i)
	slog.Debug("n yet to come", "n", len(h.toCome))
}

func newServer(port int, handler http.Handler) http.Server {
	return http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Minute,
		Handler:      handler,
	}
}
