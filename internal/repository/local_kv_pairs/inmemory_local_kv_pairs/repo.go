package inmemory_local_kv_pairs

import (
	"sync"
	"time"

	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

var _ local_kv_pairs.Repository[any] = &inmemoryLocalKVPairs[any]{}

type inmemoryLocalKVPairs[V any] struct {
	storage map[string]*item[V]
	mu      sync.RWMutex
	metrics *metrics
}

type item[V any] struct {
	value    V
	modified time.Time
}

func New[V any]() *inmemoryLocalKVPairs[V] {
	repo := inmemoryLocalKVPairs[V]{
		storage: map[string]*item[V]{},
	}

	repo.metrics = newMetrics(&repo)

	return &repo
}

func (repo *inmemoryLocalKVPairs[V]) Get(key string) (res model.KVPair[V], resErr error) {
	repo.metrics.getRequestsCnt.Inc()
	defer func(ts time.Time) {
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))
		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	item, found := repo.storage[key]
	if !found {
		return model.KVPair[V]{}, model.KeyNotFoundError{Key: key}
	}

	return model.KVPair[V]{
		Key:      key,
		Value:    item.value,
		Modified: item.modified,
	}, nil
}

func (repo *inmemoryLocalKVPairs[V]) GetNoValue(key string) (res model.KVPair[V], resErr error) {
	repo.metrics.getRequestsCnt.Inc()
	defer func(ts time.Time) {
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))
		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	item, found := repo.storage[key]
	if !found {
		return model.KVPair[V]{}, model.KeyNotFoundError{Key: key}
	}

	return model.KVPair[V]{
		Key:      key,
		Modified: item.modified,
	}, nil
}

func (repo *inmemoryLocalKVPairs[V]) AddOrUpdate(kv model.KVPair[V], mf model.Merger[V]) (resErr error) {
	repo.metrics.setRequestsCnt.Inc()
	defer func(ts time.Time) {
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))
		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	repo.mu.Lock()
	defer repo.mu.Unlock()

	it := repo.storage[kv.Key]
	if it == nil {
		it = &item[V]{value: kv.Value, modified: kv.Modified}
	}
	oldKV := model.KVPair[V]{Key: kv.Key, Value: it.value, Modified: it.modified}

	resKV := mf.Merge(oldKV, kv)
	repo.storage[kv.Key] = &item[V]{value: resKV.Value, modified: resKV.Modified}

	return nil
}

func (repo *inmemoryLocalKVPairs[V]) Remove(key string) (resErr error) {
	repo.metrics.delRequestsCnt.Inc()
	defer func(ts time.Time) {
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))
		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	repo.mu.Lock()
	defer repo.mu.Unlock()

	delete(repo.storage, key)
	return nil
}

func (repo *inmemoryLocalKVPairs[V]) GetAllNoValue() (res []model.KVPair[V], resErr error) {
	repo.metrics.getRequestsCnt.Inc()
	defer func(ts time.Time) {
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))
		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	repo.mu.RLock()
	defer repo.mu.RUnlock()

	return lo.Map(
			lo.Keys(repo.storage),
			func(el string, _ int) model.KVPair[V] {
				return model.KVPair[V]{Key: el, Modified: repo.storage[el].modified}
			}),
		nil
}

func (repo *inmemoryLocalKVPairs[V]) Metrics() []prometheus.Collector {
	return repo.metrics.list()
}
