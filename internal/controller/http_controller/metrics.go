package http_controller

import "github.com/prometheus/client_golang/prometheus"

// TODO: impl
type metrics struct{}

func newMetrics() *metrics
func (m *metrics) list() []prometheus.Collector
