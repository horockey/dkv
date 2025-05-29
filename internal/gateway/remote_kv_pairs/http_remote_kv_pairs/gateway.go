package http_remote_kv_pairs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	controller_dto "github.com/horockey/dkv/internal/controller/http_controller/dto"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

var _ remote_kv_pairs.Gateway[any] = &httpRemoteKVPairs[any]{}

type httpRemoteKVPairs[V any] struct {
	cl      *resty.Client
	metrics *metrics
	logger  zerolog.Logger
}

func New[V any](
	servicePort int,
	apiKey string,
	logger zerolog.Logger,
) *httpRemoteKVPairs[V] {
	return &httpRemoteKVPairs[V]{
		metrics: newMetrics(),
		logger:  logger,
		cl: resty.New().
			SetPathParam("port", strconv.Itoa(servicePort)).
			SetHeader("X-Api-Key", apiKey).
			SetRetryCount(0),
	}
}

func (gw *httpRemoteKVPairs[V]) Metrics() []prometheus.Collector {
	return gw.metrics.list()
}

func (gw *httpRemoteKVPairs[V]) Get(
	ctx context.Context,
	hostname string,
	key string,
) (res model.KVPair[V], resErr error) {
	gw.logger.Debug().Str("key", key).Str("host", hostname).Msg("Getting KV from remote")
	defer func(ts time.Time) {
		gw.metrics.requestsCnt.Inc()
		gw.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch resErr {
		case nil:
			gw.metrics.successProcessCnt.Inc()
		default:
			gw.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	resp, err := gw.cl.R().
		SetContext(ctx).
		SetPathParam("hostname", hostname).
		SetPathParam("key", key).
		Get("http://{hostname}:{port}/kv/{key}")
	if err != nil {
		return model.KVPair[V]{}, fmt.Errorf("executing request: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return model.KVPair[V]{}, model.KeyNotFoundError{Key: key}
	default:
		return model.KVPair[V]{}, fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	dtoKV := controller_dto.KV{}
	if err := json.Unmarshal(resp.Body(), &dtoKV); err != nil {
		return model.KVPair[V]{}, fmt.Errorf("unmarshaling json: %w", err)
	}

	kv, err := controller_dto.KVToModel[V](dtoKV)
	if err != nil {
		return model.KVPair[V]{}, fmt.Errorf("converting dto kv to model: %w", err)
	}

	return kv, nil
}

func (gw *httpRemoteKVPairs[V]) AddOrUpdate(
	ctx context.Context,
	hostname string,
	kvp model.KVPair[V],
) (resErr error) {
	defer func(ts time.Time) {
		gw.metrics.requestsCnt.Inc()
		gw.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch resErr {
		case nil:
			gw.metrics.successProcessCnt.Inc()
		default:
			gw.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	dtoKVP, err := controller_dto.NewKV(kvp)
	if err != nil {
		return fmt.Errorf("converting model kv to dto: %w", err)
	}

	resp, err := gw.cl.R().
		SetContext(ctx).
		SetPathParam("hostname", hostname).
		SetHeader("Content-Type", "application/json").
		SetBody(dtoKVP).
		Post("http://{hostname}:{port}/kv")
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return model.KeyNotFoundError{Key: kvp.Key}
	default:
		return fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	return nil
}

func (gw *httpRemoteKVPairs[V]) Remove(ctx context.Context, hostname string, key string) (resErr error) {
	defer func(ts time.Time) {
		gw.metrics.requestsCnt.Inc()
		gw.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch resErr {
		case nil:
			gw.metrics.successProcessCnt.Inc()
		default:
			gw.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	resp, err := gw.cl.R().
		SetContext(ctx).
		SetPathParam("hostname", hostname).
		SetPathParam("key", key).
		Delete("http://{hostname}:{port}/kv/{key}")
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return model.KeyNotFoundError{Key: key}
	default:
		return fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	return nil
}
