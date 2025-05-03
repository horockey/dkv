package dkv

import (
	"context"
	"fmt"

	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/dkv/pkg/hashringx"
)

type Client[K fmt.Stringer, V any] struct {
	localStorage      local_kv_pairs.Repository[K, V]
	remoteStorage     remote_kv_pairs.Gateway[K, V]
	discovery         Discovery
	hashRing          *hashringx.HashRing
	replicationFactor float64
	minReplicas       uint16
	maxReplicas       uint16
}

func (cl *Client[K, V]) Get(ctx context.Context, key K) (V, error)
func (cl *Client[K, V]) AddOrUpdate(ctx context.Context, key K, value V) error
func (cl *Client[K, V]) Remove(ctx context.Context, key K) error
