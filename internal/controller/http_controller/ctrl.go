package http_controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/horockey/dkv/internal/controller/http_controller/dto"
	"github.com/horockey/dkv/internal/processor"
	"github.com/horockey/go-toolbox/http_helpers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
)

type HttpController[V any] struct {
	serv    *http.Server
	apiKey  string
	proc    *processor.Processor[V]
	logger  zerolog.Logger
	metrics *metrics
}

func New[V any](
	addr string,
	apiKey string,
	logger zerolog.Logger,
) *HttpController[V] {
	ctrl := HttpController[V]{
		serv: &http.Server{
			Addr: addr,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotImplemented)
			}),
		},
		apiKey:  apiKey,
		logger:  logger,
		metrics: newMetrics(),
	}

	router := mux.NewRouter()
	if ctrl.serv.Handler != nil {
		router.NotFoundHandler = ctrl.serv.Handler
	}

	router.HandleFunc("/kv/{key}", ctrl.getKVKeyHandler).Methods(http.MethodGet)
	router.HandleFunc("/kv/{key}", ctrl.deleteKVKeyHandler).Methods(http.MethodDelete)
	router.HandleFunc("/kv", ctrl.postKVHandler).Methods(http.MethodPost)
	router.Use(ctrl.authMW)

	ctrl.serv.Handler = router

	return &ctrl
}

func (ctrl *HttpController[V]) Metrics() []prometheus.Collector {
	return ctrl.metrics.list()
}

func (ctrl *HttpController[V]) Start(ctx context.Context, pr *processor.Processor[V]) (resErr error) {
	ctrl.proc = pr
	var wg sync.WaitGroup
	defer wg.Wait()

	errCh := make(chan error, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ctrl.serv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() != nil && !errors.Is(ctx.Err(), context.Canceled) {
			resErr = errors.Join(resErr, fmt.Errorf("running context: %w", ctx.Err()))
		}

		sdCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := ctrl.serv.Shutdown(sdCtx); err != nil {
			resErr = errors.Join(resErr, fmt.Errorf("shutting down server: %w", err))
		}
		return resErr

	case err := <-errCh:
		return fmt.Errorf("running server: %w", err)
	}
}

func (ctrl *HttpController[V]) authMW(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("X-Api-Key") != ctrl.apiKey {
			w.WriteHeader(http.StatusForbidden)
		}
		next.ServeHTTP(w, req)
	})
}

func (ctrl *HttpController[V]) getKVKeyHandler(w http.ResponseWriter, req *http.Request) {
	key, found := mux.Vars(req)["key"]
	if !found {
		err := errors.New("missing key")
		ctrl.logger.Error().Err(err).Send()
		_ = http_helpers.RespondWithErr(w, http.StatusBadRequest, err)
		return
	}

	kvp, err := ctrl.proc.Get(req.Context(), key)
	if err != nil {
		ctrl.logger.
			Error().
			Err(fmt.Errorf("getting kvp from proc: %w", err)).
			Send()
		_ = http_helpers.RespondWithErr(w, http.StatusInternalServerError, nil)
		return
	}

	noValStr := req.URL.Query().Get("no-value")
	if noValStr == "true" {
		kvp.Value = *new(V)
	}

	dtoKV, err := dto.NewKV(kvp)
	if err != nil {
		ctrl.logger.
			Error().
			Err(fmt.Errorf("converting model kvp to dto: %w", err)).
			Send()
		_ = http_helpers.RespondWithErr(w, http.StatusInternalServerError, nil)
		return
	}

	_ = http_helpers.RespondOK(w, dtoKV)
}

func (ctrl *HttpController[V]) deleteKVKeyHandler(w http.ResponseWriter, req *http.Request) {
	key, found := mux.Vars(req)["key"]
	if !found {
		err := errors.New("missing key")
		ctrl.logger.Error().Err(err).Send()
		_ = http_helpers.RespondWithErr(w, http.StatusBadRequest, err)
		return
	}

	if err := ctrl.proc.Remove(req.Context(), key); err != nil {
		ctrl.logger.
			Error().
			Err(fmt.Errorf("deleting kvp from proc: %w", err)).
			Send()
		_ = http_helpers.RespondWithErr(w, http.StatusInternalServerError, nil)
		return
	}

	_ = http_helpers.RespondOK(w, nil)
}

func (ctrl *HttpController[V]) postKVHandler(w http.ResponseWriter, req *http.Request) {
	dtoKV := dto.KV{}
	if err := json.NewDecoder(req.Body).Decode(&dtoKV); err != nil {
		ctrl.logger.
			Error().
			Err(fmt.Errorf("decoding body dto: %w", err)).
			Send()
		_ = http_helpers.RespondWithErr(w, http.StatusInternalServerError, nil)
		return
	}

	kvp, err := dto.KVToModel[V](dtoKV)
	if err != nil {
		ctrl.logger.
			Error().
			Err(fmt.Errorf("converting dto to model: %w", err)).
			Send()
		_ = http_helpers.RespondWithErr(w, http.StatusInternalServerError, nil)
		return
	}

	if err := ctrl.proc.AddOrUpdate(
		req.Context(),
		kvp.Key,
		kvp.Value,
	); err != nil {
		ctrl.logger.
			Error().
			Err(fmt.Errorf("setting kvp to proc: %w", err)).
			Send()
		_ = http_helpers.RespondWithErr(w, http.StatusInternalServerError, nil)
		return
	}

	_ = http_helpers.RespondOK(w, nil)
}
