package model

import "github.com/prometheus/client_golang/prometheus"

type MetricsProvider interface {
	Metrics() []prometheus.Collector
}
