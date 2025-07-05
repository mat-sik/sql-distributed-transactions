package server

import (
	"github.com/prometheus/client_golang/prometheus"
)

var counter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "transaction_enqueue_requests_total",
		Help: "A counter for transaction enqueue requests.",
	},
	[]string{"code"},
)

var inFlightGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "in_flight_transaction_enqueue_requests",
		Help: "A gauge of transaction enqueue requests currently being served.",
	},
)

var duration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "transaction_enqueue_request_duration_seconds",
		Help:    "A histogram of latencies for transaction enqueue requests.",
		Buckets: []float64{.25, .5, 1, 2, 3, 5},
	},
	[]string{"code"},
)
