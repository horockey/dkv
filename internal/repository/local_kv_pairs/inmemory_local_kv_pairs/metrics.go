package inmemory_local_kv_pairs

import (
	"unsafe"

	"github.com/horockey/go-toolbox/prometheus_helpers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	handleTimeHist     prometheus.Histogram
	getRequestsCnt     prometheus.Counter
	setRequestsCnt     prometheus.Counter
	delRequestsCnt     prometheus.Counter
	successProcessCnt  prometheus.Counter
	errProcessCnt      prometheus.Counter
	repoSizeItemsGauge prometheus.GaugeFunc
	repoSizeBytesGauge prometheus.GaugeFunc
}

func newMetrics[V any](repo *inmemoryLocalKVPairs[V]) *metrics {
	const ss = "inmemory_local_kv_pairs"

	var v V
	sizeOfElement := unsafe.Sizeof(v)

	return &metrics{
		handleTimeHist: prometheus.NewHistogram(*prometheus_helpers.NewHistOpts(
			"handle_time_hist",
			prometheus_helpers.HistOptsWithSubsystem(ss),
			prometheus_helpers.HistOptsWithHelp("Handle time distribution"),
		)),
		getRequestsCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "get_requests_cnt",
			Subsystem: ss,
			Help:      "Count of incoming requests for kv read",
		}),
		setRequestsCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "set_requests_cnt",
			Subsystem: ss,
			Help:      "Count of incoming requests for kv put",
		}),
		delRequestsCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "del_requests_cnt",
			Subsystem: ss,
			Help:      "Count of incoming requests for kv del",
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
			return float64(len(repo.storage) * int(sizeOfElement))
		}),
	}
}

func (m *metrics) list() []prometheus.Collector {
	return []prometheus.Collector{
		m.handleTimeHist,
		m.getRequestsCnt,
		m.successProcessCnt,
		m.errProcessCnt,
		m.repoSizeItemsGauge,
		m.repoSizeBytesGauge,
	}
}
