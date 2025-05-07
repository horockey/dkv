package http_controller

import (
	"github.com/horockey/go-toolbox/prometheus_helpers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	handleTimeHist    prometheus.Histogram
	requestsCnt       prometheus.Counter
	successProcessCnt prometheus.Counter
	errProcessCnt     prometheus.Counter
}

func newMetrics() *metrics {
	const ss = "http_rcontroller"
	return &metrics{
		handleTimeHist: prometheus.NewHistogram(*prometheus_helpers.NewHistOpts(
			"handle_time_hist",
			prometheus_helpers.HistOptsWithSubsystem(ss),
			prometheus_helpers.HistOptsWithHelp("Handle time distribution"),
		)),
		requestsCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "requests_cnt",
			Subsystem: ss,
			Help:      "Count of incoming requests",
		}),
		successProcessCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "success_responses_cnt",
			Subsystem: ss,
			Help:      "Count of successfully finished processes",
		}),
		errProcessCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "err_processes_cnt",
			Subsystem: ss,
			Help:      "Count of processes finished with non-nil error",
		}),
	}
}

func (m *metrics) list() []prometheus.Collector {
	return []prometheus.Collector{
		m.handleTimeHist,
		m.requestsCnt,
		m.successProcessCnt,
		m.errProcessCnt,
	}
}
