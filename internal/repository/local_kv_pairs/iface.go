package local_kv_pairs

import (
	"github.com/horockey/dkv/internal/model"
)

type Repository[V any] interface {
	model.MetricsProvider
	Get(key string) (model.KVPair[V], error)
	GetNoValue(key string) (model.KVPair[V], error)
	AddOrUpdate(kv model.KVPair[V], mf model.Merger[V]) error
	Remove(key string) error
	CheckTombstone(key string) (ts int64, err error)
	GetAllNoValue() ([]model.KVPair[V], error)
}
