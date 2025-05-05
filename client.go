package dkv

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	"github.com/horockey/go-toolbox/options"
	"github.com/rs/zerolog"
)

type (
	Discovery = model.Discovery
)

type Client[K fmt.Stringer, V any] struct {
	*processor.Processor[K, V]
	ctrl Controller
}

type createClientParams[K fmt.Stringer, V any] struct {
	badgerDir     string
	servicePort   int
	weight        uint
	replicas      uint8
	revertTimeout time.Duration
	logger        zerolog.Logger

	localRepo  local_kv_pairs.Repository[K, V]
	remoteRepo remote_kv_pairs.Gateway[K, V]
	controller Controller
}

func defaultCreateClientParams[K fmt.Stringer, V any]() createClientParams[K, V] {
	return createClientParams[K, V]{
		badgerDir:     "./badger",
		servicePort:   7000,
		weight:        1,
		replicas:      2,
		revertTimeout: time.Second * 10,
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

		params.localRepo = badger_local_kv_pairs.New[K, V](badgerDB)
	}

	if params.remoteRepo == nil {
		params.remoteRepo = http_remote_kv_pairs.New[K, V](params.servicePort, apiKey)
	}

	proc := processor.New(
		params.localRepo,
		params.remoteRepo,
		hostname,
		params.weight,
		discovery,
		params.replicas,
		params.revertTimeout,
		params.logger,
	)

	if params.controller == nil {
		params.controller = http_controller.New(
			"0.0.0.0:"+strconv.Itoa(params.servicePort),
			apiKey,
			proc,
			params.logger.With().Str("subscope", "http_controller").Logger(),
		)
	}

	return &Client[K, V]{Processor: proc, ctrl: params.controller}, nil
}

func (cl *Client[K, V]) Start(ctx context.Context) error {
	runCtx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := cl.ctrl.Start(runCtx); err != nil && !errors.Is(err, context.Canceled) {
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
