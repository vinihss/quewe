package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	QueueDepth = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "queue_depth",
			Help: "Current number of queued messages.",
		},
	)

	LeaseExpirations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "lease_expirations_total",
			Help: "Total number of lease expirations reaped.",
		},
	)

	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_workers",
			Help: "Number of active workers.",
		},
	)
)
