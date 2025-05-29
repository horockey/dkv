package dkv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/horockey/dkv/internal/controller/http_controller"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs/http_remote_kv_pairs"
	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/processor"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs/inmemory_local_kv_pairs"
	"github.com/horockey/dkv/pkg/hashringx"
	"github.com/horockey/go-toolbox/options"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type Client[V any] struct {
	*processor.Processor[V]
	localRepo  local_kv_pairs.Repository[V]
	remoteRepo remote_kv_pairs.Gateway[V]
	ctrl       Controller[V]
}

type createClientParams[V any] struct {
	badgerDir              string
	servicePort            int
	weight                 uint
	replicas               uint8
	merger                 Merger[V]
	hashFunc               hashringx.HashFunc
	revertTimeout          time.Duration
	logger                 zerolog.Logger
	localRepoTombstonesTTL time.Duration

	localRepo  local_kv_pairs.Repository[V]
	remoteRepo remote_kv_pairs.Gateway[V]
	controller Controller[V]
}

func defaultCreateClientParams[V any]() createClientParams[V] {
	return createClientParams[V]{
		badgerDir:              "./badger",
		servicePort:            7000,             //nolint: mnd
		weight:                 1,                //nolint: mnd
		replicas:               2,                //nolint: mnd
		revertTimeout:          time.Second * 10, //nolint: mnd
		localRepoTombstonesTTL: time.Hour * 24,   //nolint: mnd
		merger:                 model.MergeFunc[V](model.LastTsMerge[V]),
		hashFunc:               hashringx.DefaultHashFunc,
		logger: zerolog.New(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}).With().
			Timestamp().
			Str("scope", "dkv_client").
			Logger(),
	}
}

func NewClient[V any](
	apiKey string,
	hostname string,
	discovery Discovery,
	opts ...options.Option[createClientParams[V]],
) (*Client[V], error) {
	params := defaultCreateClientParams[V]()
	if err := options.ApplyOptions(&params, opts...); err != nil {
		return nil, fmt.Errorf("applying opts: %w", err)
	}

	if params.localRepo == nil {
		params.localRepo = inmemory_local_kv_pairs.New[V]()
	}

	if params.remoteRepo == nil {
		params.remoteRepo = http_remote_kv_pairs.New[V](
			params.servicePort,
			apiKey,
			params.logger.With().Str("scope", "remote GW").Logger(),
			hostname,
		)
	}

	if params.controller == nil {
		params.controller = http_controller.New[V](
			"0.0.0.0:"+strconv.Itoa(params.servicePort),
			apiKey,
			params.logger.With().Str("subscope", "http_controller").Logger(),
		)
	}

	proc := processor.New(
		params.localRepo,
		params.merger,
		params.remoteRepo,
		hostname,
		params.weight,
		discovery,
		params.hashFunc,
		params.replicas,
		params.revertTimeout,
		params.logger,
	)

	return &Client[V]{
		Processor:  proc,
		ctrl:       params.controller,
		localRepo:  params.localRepo,
		remoteRepo: params.remoteRepo,
	}, nil
}

func (cl *Client[V]) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cl.ctrl.Start(runCtx, cl.Processor); err != nil && !errors.Is(err, context.Canceled) {
			cl.Logger.
				Error().
				Err(fmt.Errorf("running http controller: %w", err)).
				Send()
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cl.Processor.Start(runCtx); err != nil {
			cl.Logger.
				Error().
				Err(fmt.Errorf("running processor: %w", err)).
				Send()
			cancel()
		}
	}()

	<-runCtx.Done()
	wg.Wait()
	return fmt.Errorf("running context: %w", runCtx.Err())
}

func (cl *Client[V]) Metrics() []prometheus.Collector {
	return slices.Concat(
		cl.ctrl.Metrics(),
		cl.Processor.Metrics(),
		cl.localRepo.Metrics(),
		cl.remoteRepo.Metrics(),
	)
}

func (cl *Client[V]) AddOrUpdate(ctx context.Context, key string, value V) error {
	return cl.Processor.AddOrUpdate(ctx, key, value, "") //nolint
}

func (cl *Client[V]) Remove(ctx context.Context, key string) error {
	return cl.Processor.Remove(ctx, key, "") //nolint
}
