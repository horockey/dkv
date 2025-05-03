package local_kv_pairs

import (
	"fmt"

	"github.com/horockey/dkv/internal/model"
)

type Repository[K fmt.Stringer, V any] interface {
	Get(K) (model.KVPair[K, V], error)
	GetNoValue(K) (model.KVPair[K, V], error)
	AddOrUpdate(model.KVPair[K, V]) error
	Remove(K) error
}
