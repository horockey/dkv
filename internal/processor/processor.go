package processor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/dkv/pkg/hashringx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
)

type Processor[K fmt.Stringer, V any] struct {
	localStorage  local_kv_pairs.Repository[K, V]
	merger        model.Merger[K, V]
	remoteStorage remote_kv_pairs.Gateway[K, V]
	hostname      string
	weight        uint
	discovery     model.Discovery
	hashRing      *hashringx.HashRing
	hashFunc      hashringx.HashFunc
	replicas      uint8
	revertTimeout time.Duration
	Logger        zerolog.Logger
	metrics       *metrics
}

func New[K fmt.Stringer, V any](
	localStorage local_kv_pairs.Repository[K, V],
	merger model.Merger[K, V],
	remoteStorage remote_kv_pairs.Gateway[K, V],
	hostname string,
	weight uint,
	discovery model.Discovery,
	hashFunc hashringx.HashFunc,
	replicas uint8,
	revertTimeout time.Duration,
	logger zerolog.Logger,
) *Processor[K, V] {
	return &Processor[K, V]{
		localStorage:  localStorage,
		merger:        merger,
		remoteStorage: remoteStorage,
		hostname:      hostname,
		weight:        weight,
		discovery:     discovery,
		hashFunc:      hashFunc,
		replicas:      replicas,
		revertTimeout: revertTimeout,
		Logger:        logger,
		hashRing:      hashringx.New([]string{}),
		metrics:       newMetrics(),
	}
}

func (pr *Processor[K, V]) Metrics() []prometheus.Collector {
	return pr.metrics.list()
}

func (pr *Processor[K, V]) Start(ctx context.Context) error {
	// on start - check all local keys, move to others and delete on self if needed
	// on cluster update - the same + upd hashring state (use weight from meta)
	// only holder and R replicas allowed to hold key

	allNodes, err := pr.discovery.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("getting all nodes from discovery: %w", err)
	}

	pr.hashRing = hashringx.NewWithHashAndWeights(
		lo.SliceToMap(
			allNodes,
			func(el model.Node) (string, int) {
				return el.Hostname, parseWeightFromMeta(el.Meta)
			},
		),
		pr.hashFunc,
	)
	pr.hashRing = pr.hashRing.AddWeightedNode(pr.hostname, int(pr.weight))

	err = pr.discovery.Register(
		ctx,
		pr.hostname,
		func(upd model.Node) error {
			switch upd.State {
			case model.StateDown:
				// If already processed
				allNodes, _ := pr.hashRing.GetNodes("", pr.hashRing.Size())
				if !slices.Contains(allNodes, upd.Hostname) {
					return nil
				}

				pr.hashRing = pr.hashRing.RemoveNode(upd.Hostname)
			case model.StateUp:
				// If already processed
				allNodes, _ := pr.hashRing.GetNodes("", pr.hashRing.Size())
				if slices.Contains(allNodes, upd.Hostname) {
					return nil
				}

				pr.hashRing = pr.hashRing.AddWeightedNode(upd.Hostname, parseWeightFromMeta(upd.Meta))
			}
			pr.moveExtraKvpsToRemotes(ctx)
			return nil
		},
		map[string]string{"weight": strconv.Itoa(int(pr.weight))},
	)
	if err != nil {
		return fmt.Errorf("registering service in discovery: %w", err)
	}
	defer func() {
		if err := pr.discovery.Deregister(ctx); err != nil {
			pr.Logger.
				Error().
				Err(fmt.Errorf("deregidtering from doscovery: %w", err)).
				Send()
		}
	}()

	pr.moveExtraKvpsToRemotes(ctx)

	<-ctx.Done()
	return fmt.Errorf("running context: %w", ctx.Err())
}

func (pr *Processor[K, V]) Get(ctx context.Context, key K) (model.KVPair[K, V], error) {
	// check holder by hashring
	// if self - retrieve from local (or return err for absent)
	// if other - retrieve via gateway

	owner, ok := pr.hashRing.GetNode(key.String())
	if !ok {
		return model.KVPair[K, V]{}, fmt.Errorf("unable to get owner for %s", key.String())
	}

	if owner != pr.hostname {
		kvp, err := pr.remoteStorage.Get(ctx, owner, key)
		if err != nil {
			return model.KVPair[K, V]{}, fmt.Errorf("getting from remote storage: %w", err)
		}
		return kvp, nil
	}

	kvp, err := pr.localStorage.Get(key)
	if err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("getting from local storage: %w", err)
	}
	return kvp, nil
}

func (pr *Processor[K, V]) AddOrUpdate(ctx context.Context, key K, value V) (resErr error) {
	// put to self
	// put to R replicas
	// OR put via remote
	// all in 1 transaction

	revertOnErr := func(nodesToRevert []string) {
		if resErr == nil {
			return
		}

		revCtx, cancel := context.WithTimeout(context.Background(), pr.revertTimeout)
		defer cancel()

		for _, node := range nodesToRevert {
			var err error

			if node == pr.hostname {
				err = pr.localStorage.Remove(key)
			} else {
				err = pr.remoteStorage.Remove(revCtx, node, key)
			}

			if err != nil {
				resErr = errors.Join(resErr, fmt.Errorf("reverting on %s: %w", node, err))
				if errors.Is(err, context.DeadlineExceeded) {
					break
				}
			}
		}
	}

	kvp := model.KVPair[K, V]{
		Key:      key,
		Value:    value,
		Modified: time.Now(),
	}

	owner, replicas, err := pr.getOwnerAndReplicas(key)
	if err != nil {
		return fmt.Errorf("unable to get owner and replicas for %s", key.String())
	}

	if owner != pr.hostname {
		if err := pr.remoteStorage.AddOrUpdate(ctx, owner, kvp); err != nil {
			return fmt.Errorf("setting to remote repo (%s): %w", owner, err)
		}

		return nil
	}

	tombTS, err := pr.localStorage.CheckTombstone(key)
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return fmt.Errorf("checking tombstone: %w", err)
	} else if tomb := time.Unix(tombTS, 0); err == nil && tomb.After(kvp.Modified) {
		return fmt.Errorf("key %s has been tombstoned at %s", key.String(), tomb.Format(time.RFC3339))
	}

	defer revertOnErr(append(replicas, owner))
	if err := pr.localStorage.AddOrUpdate(kvp, pr.merger); err != nil {
		return fmt.Errorf("setting to local repo: %w", err)
	}

	for _, node := range replicas {
		if err := pr.remoteStorage.AddOrUpdate(ctx, node, kvp); err != nil {
			return fmt.Errorf("setting to remote repo (%s): %w", node, err)
		}
	}

	return nil
}

func (pr *Processor[K, V]) Remove(ctx context.Context, key K) (resErr error) {
	// remove from self
	// remove from R replicas
	// OR remove via remote
	// all in 1 transaction

	revert := func(nodesToRevert []string, kvp model.KVPair[K, V]) {
		revCtx, cancel := context.WithTimeout(context.Background(), pr.revertTimeout)
		defer cancel()

		for _, node := range nodesToRevert {
			err := pr.remoteStorage.AddOrUpdate(revCtx, node, kvp)
			if err != nil {
				resErr = errors.Join(resErr, fmt.Errorf("reverting on %s: %w", node, err))
				if errors.Is(err, context.DeadlineExceeded) {
					break
				}
			}
		}
	}

	kvp, err := pr.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("getting kvp for restore: %w", err)
	}

	owner, replicas, err := pr.getOwnerAndReplicas(key)
	if err != nil {
		return fmt.Errorf("unable to get owner and replicas for %s", key.String())
	}

	if pr.hostname != owner {
		if err := pr.remoteStorage.Remove(ctx, owner, key); err != nil {
			return fmt.Errorf("removing from remote repo (%s): %w", owner, err)
		}
		return nil
	}

	processedNodes := []string{}

	if err := pr.localStorage.Remove(key); err != nil {
		return fmt.Errorf("removing from local repo: %w", err)
	}
	processedNodes = append(processedNodes, owner)

	for _, node := range replicas {
		if err := pr.remoteStorage.Remove(ctx, node, key); err != nil {
			defer revert(processedNodes, kvp)
			return fmt.Errorf("removing from remote repo (%s): %w", node, err)
		}
		processedNodes = append(processedNodes, node)
	}

	return nil
}

func (pr *Processor[K, V]) getOwnerAndReplicas(key K) (owner string, replicas []string, resErr error) {
	nodes, ok := pr.hashRing.GetNodes(key.String(), min(int(pr.replicas)+1, pr.hashRing.Size()))
	if !ok {
		return "", nil, fmt.Errorf("unable to get owner from hashring")
	}

	o := nodes[0]
	rs := []string{}
	if len(nodes) > 1 {
		rs = nodes[1:]
	}

	return o, rs, nil
}

func (pr *Processor[K, V]) moveExtraKvpsToRemotes(ctx context.Context) {
	localKeys, err := pr.localStorage.GetAllNoValue()
	if err != nil {
		pr.Logger.
			Error().
			Err(fmt.Errorf("getting all keys from local repo: %w", err)).
			Send()
		return
	}

	keysToMove := lo.FilterSliceToMap(
		localKeys,
		func(el model.KVPair[K, V]) (string, string, bool) {
			host, ok := pr.hashRing.GetNode(el.Key.String())
			if !ok || host == pr.hostname {
				return "", "", false
			}
			return el.Key.String(), host, true
		},
	)

	for _, kvp := range localKeys {
		if !slices.Contains(lo.Keys(keysToMove), kvp.Key.String()) {
			continue
		}

		host := keysToMove[kvp.Key.String()]
		remoteKVP, err := pr.remoteStorage.GetNoValue(ctx, host, kvp.Key)
		if err != nil {
			pr.Logger.
				Error().
				Err(fmt.Errorf("getting key info from remote (%s): %w", host, err)).
				Send()
			continue
		}

		if remoteKVP.Modified.After(kvp.Modified) {
			// newer version on remote
			continue
		}

		fullLocalKVP, err := pr.localStorage.Get(kvp.Key)
		if err != nil {
			pr.Logger.
				Error().
				Err(fmt.Errorf("getting full local kvp: %w", err)).
				Send()
			continue
		}

		if err := pr.remoteStorage.AddOrUpdate(ctx, host, fullLocalKVP); err != nil {
			pr.Logger.
				Error().
				Err(fmt.Errorf("putting kvp to remote: %w", err)).
				Send()
			continue
		}

		if err := pr.localStorage.Remove(kvp.Key); err != nil {
			pr.Logger.
				Error().
				Err(fmt.Errorf("removing from local storage: %w", err)).
				Send()
			continue
		}

	}
}

func parseWeightFromMeta(meta map[string]string) int {
	wStr, found := meta["weight"]
	if !found {
		return 1
	}

	w, err := strconv.Atoi(wStr)
	if err != nil {
		return 1
	}

	return w
}
