package remote_kv_pairs

import (
	"context"
	"fmt"

	"github.com/horockey/dkv/internal/model"
)

type Gateway[K fmt.Stringer, V any] interface {
	model.MetricsProvider
	Get(ctx context.Context, hostname string, key K) (model.KVPair[K, V], error)
	GetNoValue(ctx context.Context, hostname string, key K) (model.KVPair[K, V], error)
	AddOrUpdate(ctx context.Context, hostname string, kvp model.KVPair[K, V]) error
	Remove(ctx context.Context, hostname string, key K) error
}
