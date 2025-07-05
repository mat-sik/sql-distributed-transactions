package server

import (
	"database/sql"
	"fmt"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/config"
	"github.com/mat-sik/sql-distributed-transactions/server/internal/transaction"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

func NewServer(serverConfig config.Server, handler http.Handler) http.Server {
	return http.Server{
		Addr:         fmt.Sprintf(":%d", serverConfig.Port),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Minute,
		Handler:      handler,
	}
}

func NewHandler(pool *sql.DB) http.Handler {
	prometheus.MustRegister(inFlightGauge, duration, counter)

	mux := http.NewServeMux()
	transactionHandler := transaction.NewHandler(pool)

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.Handle("POST /transactions/enqueue",
		promhttp.InstrumentHandlerInFlight(inFlightGauge,
			promhttp.InstrumentHandlerDuration(duration,
				promhttp.InstrumentHandlerCounter(counter,
					transactionHandler,
				),
			),
		),
	)

	return mux
}
