package processor

import (
	"github.com/horockey/go-toolbox/prometheus_helpers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	hashingTimeHist prometheus.Histogram
}

func newMetrics() *metrics {
	const ss = "processor"
	return &metrics{
		hashingTimeHist: prometheus.NewHistogram(*prometheus_helpers.NewHistOpts(
			"handle_time_hist",
			prometheus_helpers.HistOptsWithSubsystem(ss),
			prometheus_helpers.HistOptsWithHelp("Handle time distribution"),
		)),
	}
}

func (m *metrics) list() []prometheus.Collector {
	return []prometheus.Collector{m.hashingTimeHist}
}
