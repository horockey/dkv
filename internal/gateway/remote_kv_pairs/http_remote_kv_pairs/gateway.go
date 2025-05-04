package http_remote_kv_pairs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-resty/resty/v2"
	controller_dto "github.com/horockey/dkv/internal/controller/http_controller/dto"
	"github.com/horockey/dkv/internal/gateway/remote_kv_pairs"
	"github.com/horockey/dkv/internal/model"
)

var _ remote_kv_pairs.Gateway[fmt.Stringer, any] = &httpRemoteKVPairs[fmt.Stringer, any]{}

type httpRemoteKVPairs[K fmt.Stringer, V any] struct {
	cl *resty.Client
}

func New[K fmt.Stringer, V any](
	servicePort int,
	apiKey string,
) *httpRemoteKVPairs[K, V] {
	return &httpRemoteKVPairs[K, V]{
		cl: resty.New().
			SetPathParam("port", strconv.Itoa(servicePort)).
			SetHeader("X-Api-Key", apiKey).
			SetRetryCount(0),
	}
}

func (gw *httpRemoteKVPairs[K, V]) Get(ctx context.Context, hostname string, key K) (model.KVPair[K, V], error) {
	resp, err := gw.cl.R().
		SetContext(ctx).
		SetPathParam("hostname", hostname).
		SetPathParam("key", key.String()).
		Get("http://{hostname}:{port}/kv/{key}")
	if err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("executing request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return model.KVPair[K, V]{}, fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	dtoKV := controller_dto.KV{}
	if err := json.Unmarshal(resp.Body(), &dtoKV); err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("unmarshaling json: %w", err)
	}

	kv, err := controller_dto.KVToModel[K, V](dtoKV)
	if err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("converting dto kv to model: %w", err)
	}

	return kv, nil
}

func (gw *httpRemoteKVPairs[K, V]) GetNoValue(ctx context.Context, hostname string, key K) (model.KVPair[K, V], error) {
	resp, err := gw.cl.R().
		SetContext(ctx).
		SetPathParam("hostname", hostname).
		SetPathParam("key", key.String()).
		SetQueryParam("no-value", "true").
		Get("http://{hostname}:{port}/kv/{key}")
	if err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("executing request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return model.KVPair[K, V]{}, fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	dtoKV := controller_dto.KV{}
	if err := json.Unmarshal(resp.Body(), &dtoKV); err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("unmarshaling json: %w", err)
	}

	kv, err := controller_dto.KVToModel[K, V](dtoKV)
	if err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("converting dto kv to model: %w", err)
	}

	return kv, nil
}

func (gw *httpRemoteKVPairs[K, V]) AddOrUpdate(ctx context.Context, hostname string, kvp model.KVPair[K, V]) error {
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
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	return nil
}

func (gw *httpRemoteKVPairs[K, V]) Remove(ctx context.Context, hostname string, key K) error {
	resp, err := gw.cl.R().
		SetContext(ctx).
		SetPathParam("hostname", hostname).
		SetPathParam("key", key.String()).
		Delete("http://{hostname}:{port}/kv/{key}")
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("got non-ok response (%s): %s", resp.Status(), resp.String())
	}

	return nil
}
