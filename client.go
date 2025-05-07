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

	"github.com/dgraph-io/badger"
	"github.com/horockey/dkv/internal/controller/http_controller"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs/http_remote_kv_pairs"
	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/processor"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs/badger_local_kv_pairs"
	"github.com/horockey/dkv/pkg/hashringx"
	"github.com/horockey/go-toolbox/options"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type (
	Discovery = model.Discovery
	Node      = model.Node
)

type Client[K fmt.Stringer, V any] struct {
	*processor.Processor[K, V]
	localRepo  local_kv_pairs.Repository[K, V]
	remoteRepo remote_kv_pairs.Gateway[K, V]
	ctrl       Controller[K, V]
}

type createClientParams[K fmt.Stringer, V any] struct {
	badgerDir              string
	servicePort            int
	weight                 uint
	replicas               uint8
	merger                 Merger[K, V]
	hashFunc               hashringx.HashFunc
	revertTimeout          time.Duration
	logger                 zerolog.Logger
	localRepoTombstonesTTL time.Duration

	localRepo  local_kv_pairs.Repository[K, V]
	remoteRepo remote_kv_pairs.Gateway[K, V]
	controller Controller[K, V]
}

func defaultCreateClientParams[K fmt.Stringer, V any]() createClientParams[K, V] {
	return createClientParams[K, V]{
		badgerDir:              "./badger",
		servicePort:            7000,             //nolint: mnd
		weight:                 1,                //nolint: mnd
		replicas:               2,                //nolint: mnd
		revertTimeout:          time.Second * 10, //nolint: mnd
		localRepoTombstonesTTL: time.Hour * 24,   //nolint: mnd
		merger:                 model.MergeFunc[K, V](model.LastTsMerge[K, V]),
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

func New[K fmt.Stringer, V any](
	apiKey string,
	hostname string,
	discovery Discovery,
	opts ...options.Option[createClientParams[K, V]],
) (*Client[K, V], error) {
	params := defaultCreateClientParams[K, V]()
	if err := options.ApplyOptions(&params, opts...); err != nil {
		return nil, fmt.Errorf("applying opts: %w", err)
	}

	if params.localRepo == nil {
		if err := os.MkdirAll(params.badgerDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("creating badger dir: %w", err)
		}

		badgerDB, err := badger.Open(badger.DefaultOptions(params.badgerDir))
		if err != nil {
			return nil, fmt.Errorf("creating badger DB: %w", err)
		}

		params.localRepo = badger_local_kv_pairs.New[K, V](badgerDB, params.localRepoTombstonesTTL)
	}

	if params.remoteRepo == nil {
		params.remoteRepo = http_remote_kv_pairs.New[K, V](params.servicePort, apiKey)
	}

	if params.controller == nil {
		params.controller = http_controller.New[K, V](
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

	return &Client[K, V]{
		Processor:  proc,
		ctrl:       params.controller,
		localRepo:  params.localRepo,
		remoteRepo: params.remoteRepo,
	}, nil
}

func (cl *Client[K, V]) Start(ctx context.Context) error {
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

func (cl *Client[K, V]) Metrics() []prometheus.Collector {
	return slices.Concat(
		cl.ctrl.Metrics(),
		cl.Processor.Metrics(),
		cl.localRepo.Metrics(),
		cl.remoteRepo.Metrics(),
	)
}
