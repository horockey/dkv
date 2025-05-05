package dkv

import (
	"errors"
	"fmt"
	"time"

	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/go-toolbox/options"
	"github.com/rs/zerolog"
)

// Sets custom badger root dir.
// Default is ./badger
func WithBadgerDir[K fmt.Stringer, V any](dir string) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
		if dir == "" {
			return errors.New("got nil badger dir")
		}
		target.badgerDir = dir
		return nil
	}
}

// Sets custom service port.
// Default is 7000.
func WithServicePort[K fmt.Stringer, V any](p int) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
		if p <= 0 {
			return fmt.Errorf("port must be positive, got: %d", p)
		}
		target.servicePort = int(p)
		return nil
	}
}

// Sets custom node weight to report to discovery.
// Default is 1.
func WithNodeWeight[K fmt.Stringer, V any](w uint) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
		if w == 0 {
			return fmt.Errorf("node weight must be positive, got: %d", w)
		}
		target.weight = w
		return nil
	}
}

// Sets custom write replicas count.
// Default is 2.
func WithReplicasCount[K fmt.Stringer, V any](r uint8) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
		if r == 0 {
			return fmt.Errorf("replicas count must be positive, got: %d", r)
		}
		target.replicas = r
		return nil
	}
}

// Sets custom timeout for transaction revert.
// Default is 10s.
func WithRevertTimeout[K fmt.Stringer, V any](to time.Duration) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
		if to <= 0 {
			return fmt.Errorf("reveert timeout must be positive, got: %s", to.String())
		}
		target.revertTimeout = to
		return nil
	}
}

// Sets custom logger.
// Default is stdout logger.
func WithLogger[K fmt.Stringer, V any](l zerolog.Logger) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
		target.logger = l
		return nil
	}
}

// Sets user-defined implementation of local repository.
// Default is badger persistent repo.
//
// WARNING! Apply this opt only if you know what you are doing.
func WithLocalRepo[K fmt.Stringer, V any](repo local_kv_pairs.Repository[K, V]) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
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
func WithRemoteRepoAndController[K fmt.Stringer, V any](repo remote_kv_pairs.Gateway[K, V], ctrl Controller) options.Option[createClientParams[K, V]] {
	return func(target *createClientParams[K, V]) error {
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
