package remote_kv_pairs

import (
	"context"

	"github.com/horockey/dkv/internal/model"
)

type Gateway[V any] interface {
	model.MetricsProvider
	Get(ctx context.Context, hostname string, key string) (model.KVPair[V], error)
	AddOrUpdate(ctx context.Context, hostname string, kvp model.KVPair[V], processedOn []string) error
	Remove(ctx context.Context, hostname string, key string, processedOn []string) error
}
