package local_kv_pairs

import (
	"fmt"

	"github.com/horockey/dkv/internal/model"
)

type Repository[K fmt.Stringer, V any] interface {
	model.MetricsProvider
	Get(K) (model.KVPair[K, V], error)
	GetNoValue(K) (model.KVPair[K, V], error)
	AddOrUpdate(kv model.KVPair[K, V], mf model.Merger[K, V]) error
	Remove(K) error
	CheckTombstone(K) (ts int64, err error)
	GetAllNoValue() ([]model.KVPair[K, V], error)
}
