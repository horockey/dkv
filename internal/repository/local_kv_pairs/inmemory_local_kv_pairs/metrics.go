package inmemory_local_kv_pairs

import (
	"encoding/binary"
	"reflect"

	"github.com/horockey/go-toolbox/prometheus_helpers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	handleTimeHist     prometheus.Histogram
	requestsCnt        prometheus.Counter
	successProcessCnt  prometheus.Counter
	errProcessCnt      prometheus.Counter
	repoSizeItemsGauge prometheus.GaugeFunc
	repoSizeBytesGauge prometheus.GaugeFunc
}

func newMetrics[V any](repo *inmemoryLocalKVPairs[V]) *metrics {
	const ss = "inmemory_local_kv_pairs"
	sizeOfElement := binary.Size(reflect.ValueOf(*new(V)))

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
		repoSizeItemsGauge: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "repo_size_items_gauge",
			Subsystem: ss,
			Help:      "actual count of items in repo",
		}, func() float64 {
			return float64(len(repo.storage))
		}),
		repoSizeBytesGauge: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "repo_size_bytes_gauge",
			Subsystem: ss,
			Help:      "actual size of repo in bytes",
		}, func() float64 {
			return float64(len(repo.storage) * sizeOfElement)
		}),
	}
}

func (m *metrics) list() []prometheus.Collector {
	return []prometheus.Collector{
		m.handleTimeHist,
		m.requestsCnt,
		m.successProcessCnt,
		m.errProcessCnt,
		m.repoSizeItemsGauge,
		m.repoSizeBytesGauge,
	}
}
