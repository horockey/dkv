package dkv

import (
	"errors"
	"fmt"
	"time"

	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/dkv/pkg/hashringx"
	"github.com/horockey/go-toolbox/options"
	"github.com/rs/zerolog"
)

// Sets custom badger root dir.
// Default is ./badger
func WithBadgerDir[V any](dir string) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if dir == "" {
			return errors.New("got nil badger dir")
		}
		target.badgerDir = dir
		return nil
	}
}

// Sets custom service port.
// Default is 7000.
func WithServicePort[V any](p int) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if p <= 0 {
			return fmt.Errorf("port must be positive, got: %d", p)
		}
		target.servicePort = p
		return nil
	}
}

// Sets custom node weight to report to discovery.
// Default is 1.
func WithNodeWeight[V any](w uint) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if w == 0 {
			return fmt.Errorf("node weight must be positive, got: %d", w)
		}
		target.weight = w
		return nil
	}
}

// Sets custom write replicas count.
// Default is 2.
func WithReplicasCount[V any](r uint8) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if r == 0 {
			return fmt.Errorf("replicas count must be positive, got: %d", r)
		}
		target.replicas = r
		return nil
	}
}

// Sets custom timeout for transaction revert.
// Default is 10s.
func WithRevertTimeout[V any](to time.Duration) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if to <= 0 {
			return fmt.Errorf("reveert timeout must be positive, got: %s", to.String())
		}
		target.revertTimeout = to
		return nil
	}
}

// Sets custom tombstone TTL.
// Default is 1 day.
func WithLocalRepoTombstonesTTL[V any](ttl time.Duration) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if ttl <= 0 {
			return fmt.Errorf("ttl must be positive, got: %s", ttl.String())
		}
		target.localRepoTombstonesTTL = ttl
		return nil
	}
}

// Sets custom logger.
// Default is stdout logger.
func WithLogger[V any](l zerolog.Logger) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		target.logger = l
		return nil
	}
}

// Sets custom hashring func.
// Default is md5.
func WithHashFunc[V any](hf hashringx.HashFunc) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if hf == nil {
			return errors.New("got nil hashfunc")
		}
		target.hashFunc = hf
		return nil
	}
}

// Sets custom merger.
// By default newest data overrites old one.
func WithMerger[V any](m Merger[V]) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if m == nil {
			return errors.New("got nil merger")
		}
		target.merger = m
		return nil
	}
}

// Sets user-defined implementation of local repository.
// Default is badger persistent repo.
//
// WARNING! Apply this opt only if you know what you are doing.
func WithLocalRepo[V any](
	repo local_kv_pairs.Repository[V],
) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if repo == nil {
			return errors.New("got nil local repo")
		}
		target.localRepo = repo
		return nil
	}
}

// Sets user-defined implementations of remote repo and coresponding controller.
// Default are HTTP.
//
//	WARNING! Apply this opt only if you know what you are doing.
func WithRemoteRepoAndController[V any](
	repo remote_kv_pairs.Gateway[V],
	ctrl Controller[V],
) options.Option[createClientParams[V]] {
	return func(target *createClientParams[V]) error {
		if repo == nil {
			return errors.New("got nil remote repo")
		}
		if ctrl == nil {
			return errors.New("got nil controller")
		}

		target.remoteRepo = repo
		target.controller = ctrl
		return nil
	}
}
