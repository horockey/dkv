package badger_local_kv_pairs

import (
	"github.com/dgraph-io/badger"
	"github.com/horockey/go-toolbox/prometheus_helpers"
	"github.com/prometheus/client_golang/prometheus"
)

type metrics struct {
	handleTimeHist     prometheus.Histogram
	requestsCnt        prometheus.Counter
	successProcessCnt  prometheus.Counter
	errProcessCnt      prometheus.Counter
	keyHitsCnt         prometheus.Counter
	keyMissesCnt       prometheus.Counter
	repoSizeItemsGauge prometheus.Gauge
	repoSizeBytesGauge prometheus.GaugeFunc
}

func newMetrics(db *badger.DB) *metrics {
	const ss = "badger_local_kv_pairs"
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
		keyHitsCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "key_hits_cnt",
			Subsystem: ss,
			Help:      "Count of requests to keys exsisting in repo",
		}),
		keyMissesCnt: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "key_misses_cnt",
			Subsystem: ss,
			Help:      "Count of requests to keys not exsisting in repo",
		}),
		repoSizeItemsGauge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "repo_size_items_gauge",
			Subsystem: ss,
			Help:      "actual count of items in repo",
		}),
		repoSizeBytesGauge: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:      "repo_size_bytes_gauge",
			Subsystem: ss,
			Help:      "actual size of repo in bytes",
		}, func() float64 {
			kSize, vSize := db.Size()
			return float64(kSize + vSize)
		}),
	}
}

func (m *metrics) list() []prometheus.Collector {
	return []prometheus.Collector{
		m.handleTimeHist,
		m.requestsCnt,
		m.successProcessCnt,
		m.errProcessCnt,
		m.keyHitsCnt,
		m.keyMissesCnt,
		m.repoSizeItemsGauge,
		m.repoSizeBytesGauge,
	}
}
