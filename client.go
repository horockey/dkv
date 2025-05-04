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

func (cl *Client[K, V]) Start(ctx context.Context) error {
	// on start - check all local keys, move to others and remove on self if needed
	// on cluster update - the same
	// only holder and R replical allowed to hold key
	return nil // TODO: impl
}

func (cl *Client[K, V]) Get(ctx context.Context, key K) (V, error) {
	// check holder by hashring
	// if self - retrieve from local (or return err for absent)
	// if other - retrieve via gateway
	return *new(V), nil // TODO: impl
}

func (cl *Client[K, V]) AddOrUpdate(ctx context.Context, key K, value V) error {
	// put to self
	// put to R replicas
	// all in 1 transaction
	return nil // TODO: impl
}

func (cl *Client[K, V]) Remove(ctx context.Context, key K) error {
	// remove from self
	// remove from every other node (background)
	return nil // TODO: impl
}
