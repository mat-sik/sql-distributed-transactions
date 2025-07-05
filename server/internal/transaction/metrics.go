package transaction

import "github.com/prometheus/client_golang/prometheus"

var counter = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "transaction_executions_total",
		Help: "A counter for executed remote transactions.",
	},
	[]string{"code"},
)

var inFlightGauge = prometheus.NewGauge(
	prometheus.GaugeOpts{
		Name: "in_flight_transaction_executions",
		Help: "A gauge of remote transactions currently being executed.",
	},
)

var duration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "transaction_execution_duration_seconds",
		Help:    "A histogram of latencies for transaction executions.",
		Buckets: []float64{.25, .5, 1, 2, 3, 5},
	},
	[]string{"code"},
)

func RegisterMetrics() {
	prometheus.MustRegister(counter, inFlightGauge, duration)
}
