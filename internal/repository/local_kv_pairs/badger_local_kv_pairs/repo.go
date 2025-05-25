package badger_local_kv_pairs

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/prometheus/client_golang/prometheus"
)

var _ local_kv_pairs.Repository[any] = &badgerLocalKVPairs[any]{}

const (
	// tombstoneSuffix = "---tombstone---"
	withValueSuffix = "---with_value---"
)

type badgerLocalKVPairs[V any] struct {
	db            *badger.DB
	tombstonesTTL time.Duration
	metrics       *metrics
}

func New[V any](
	db *badger.DB,
	tombstonesTTL time.Duration,
) *badgerLocalKVPairs[V] {
	return &badgerLocalKVPairs[V]{
		db:            db,
		tombstonesTTL: tombstonesTTL,
		metrics:       newMetrics(db),
	}
}

func (repo *badgerLocalKVPairs[V]) Metrics() []prometheus.Collector {
	return repo.metrics.list()
}

func (repo *badgerLocalKVPairs[V]) Get(key string) (resKV model.KVPair[V], resErr error) {
	defer func(ts time.Time) {
		repo.metrics.requestsCnt.Inc()
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch {
		case resErr == nil:
			repo.metrics.successProcessCnt.Inc()
			repo.metrics.keyHitsCnt.Inc()
		case errors.Is(resErr, badger.ErrKeyNotFound):
			repo.metrics.keyMissesCnt.Inc()
			fallthrough
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	res := model.KVPair[V]{
		Key: key,
	}
	if err := repo.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key + withValueSuffix))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return model.KeyNotFoundError{Key: key}
			}
			return fmt.Errorf("getting item: %w", err)
		}

		if err := item.Value(func(val []byte) error {
			if err := gob.
				NewDecoder(bytes.NewBuffer(val)).
				Decode(&res); err != nil {
				return fmt.Errorf("decoding gob: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("getting value: %w", err)
		}

		return nil
	}); err != nil {
		return model.KVPair[V]{}, fmt.Errorf("reading ftom db: %w", err)
	}

	return res, nil
}

func (repo *badgerLocalKVPairs[V]) GetNoValue(key string) (resKV model.KVPair[V], resErr error) {
	defer func(ts time.Time) {
		repo.metrics.requestsCnt.Inc()
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch {
		case resErr == nil:
			repo.metrics.successProcessCnt.Inc()
			repo.metrics.keyHitsCnt.Inc()
		case errors.Is(resErr, badger.ErrKeyNotFound):
			repo.metrics.keyMissesCnt.Inc()
			fallthrough
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	res := model.KVPair[V]{
		Key: key,
	}
	if err := repo.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return model.KeyNotFoundError{Key: key}
			}
			return fmt.Errorf("getting item: %w", err)
		}

		if err := item.Value(func(val []byte) error {
			if err := gob.
				NewDecoder(bytes.NewBuffer(val)).
				Decode(&res); err != nil {
				return fmt.Errorf("decoding gob: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("getting value: %w", err)
		}

		return nil
	}); err != nil {
		return model.KVPair[V]{}, fmt.Errorf("reading ftom db: %w", err)
	}

	return res, nil
}

// Updates kvp.
// If repo has newer version of it, will return local_kv_pairs.KvTooOldError.
//
// nolint: gocognit
// TODO: refactor to multiple cognitive acceptable funcs
func (repo *badgerLocalKVPairs[V]) AddOrUpdate(kvp model.KVPair[V], mf model.Merger[V]) (resErr error) {
	defer func(ts time.Time) {
		repo.metrics.requestsCnt.Inc()
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
			repo.metrics.repoSizeItemsGauge.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	if err := repo.db.Update(func(txn *badger.Txn) error {
		oldKVPItem, err := txn.Get([]byte(kvp.Key + withValueSuffix))
		oldKVP := model.KVPair[V]{}

		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
			break
		case err == nil:
			if err := oldKVPItem.Value(func(val []byte) error {
				if err := gob.
					NewDecoder(bytes.NewBuffer(val)).
					Decode(&oldKVP); err != nil {
					return fmt.Errorf("decoding gob: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("getting value of modItem: %w", err)
			}

			if oldKVP.Modified.After(kvp.Modified) {
				return model.KvTooOldError{
					Key:         kvp.Key,
					InsertingTs: kvp.Modified,
					InRepoTs:    oldKVP.Modified,
				}
			}
		}

		mergedKvp := mf.Merge(oldKVP, kvp)

		buf := bytes.NewBuffer(nil)
		if err := gob.
			NewEncoder(buf).
			Encode(mergedKvp); err != nil {
			return fmt.Errorf("encoding gob: %w", err)
		}

		data := slices.Clone(buf.Bytes())
		buf.Reset()

		if err := txn.Set([]byte(mergedKvp.Key+withValueSuffix), data); err != nil {
			return fmt.Errorf("setting item to db: %w", err)
		}

		if err := gob.
			NewEncoder(buf).
			Encode(model.KVPair[V]{
				Key:      mergedKvp.Key,
				Modified: mergedKvp.Modified,
			}); err != nil {
			return fmt.Errorf("encoding item: %w", err)
		}
		if err := txn.Set([]byte(mergedKvp.Key), buf.Bytes()); err != nil {
			return fmt.Errorf("setting item to db: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("performing upd txn: %w", err)
	}

	return nil
}

func (repo *badgerLocalKVPairs[V]) Remove(key string) (resErr error) {
	defer func(ts time.Time) {
		repo.metrics.requestsCnt.Inc()
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
			repo.metrics.repoSizeItemsGauge.Dec()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	if err := repo.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete([]byte(key)); err != nil {
			return fmt.Errorf("deleting item: %w", err)
		}

		if err := txn.Delete([]byte(key + withValueSuffix)); err != nil {
			return fmt.Errorf("deleting modItem: %w", err)
		}

		// buf := bytes.NewBuffer(nil)
		// if err := gob.NewEncoder(buf).Encode(time.Now().Unix()); err != nil {
		// 	return fmt.Errorf("encoding gob tombstone: %w", err)
		// }

		// e := badger.Entry{
		// 	Key:   []byte(key + tombstoneSuffix),
		// 	Value: buf.Bytes(),
		// }

		// if err := txn.SetEntry(e.WithTTL(repo.tombstonesTTL)); err != nil {
		// 	return fmt.Errorf("setting tombstone: %w", err)
		// }

		return nil
	}); err != nil {
		return fmt.Errorf("performing del txn: %w", err)
	}

	return nil
}

// func (repo *badgerLocalKVPairs[V]) CheckTombstone(key string) (ts int64, resErr error) {
// 	defer func(ts time.Time) {
// 		repo.metrics.requestsCnt.Inc()
// 		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

// 		switch {
// 		case resErr == nil:
// 			repo.metrics.successProcessCnt.Inc()
// 			repo.metrics.keyHitsCnt.Inc()
// 		case errors.Is(resErr, badger.ErrKeyNotFound):
// 			repo.metrics.keyMissesCnt.Inc()
// 			fallthrough
// 		default:
// 			repo.metrics.errProcessCnt.Inc()
// 		}
// 	}(time.Now())

// 	if err := repo.db.View(func(txn *badger.Txn) error {
// 		item, err := txn.Get([]byte(key + tombstoneSuffix))
// 		if err != nil {
// 			return fmt.Errorf("getting tombstone item: %w", err)
// 		}

// 		if err := item.Value(func(val []byte) error {
// 			if err := gob.NewDecoder(bytes.NewBuffer(val)).Decode(&ts); err != nil {
// 				return fmt.Errorf("decoding gob tombstone: %w", err)
// 			}
// 			return nil
// 		}); err != nil {
// 			return fmt.Errorf("getting tombstone item value: %w", err)
// 		}

// 		return nil
// 	}); err != nil {
// 		return 0, fmt.Errorf("performing view txn: %w", err)
// 	}

// 	return ts, nil
// }

func (repo *badgerLocalKVPairs[V]) GetAllNoValue() (resKVs []model.KVPair[V], resErr error) {
	defer func(ts time.Time) {
		repo.metrics.requestsCnt.Inc()
		repo.metrics.handleTimeHist.Observe(float64(time.Since(ts)))

		switch resErr {
		case nil:
			repo.metrics.successProcessCnt.Inc()
		default:
			repo.metrics.errProcessCnt.Inc()
		}
	}(time.Now())

	resKVs = []model.KVPair[V]{}

	err := repo.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {

			item := it.Item()

			kv := model.KVPair[V]{}

			if bytes.HasSuffix(item.Key(), []byte(withValueSuffix)) {
				// || bytes.HasSuffix(item.Key(), []byte(tombstoneSuffix))
				continue
			}

			if err := item.Value(func(val []byte) error {
				if err := gob.
					NewDecoder(bytes.NewBuffer(val)).
					Decode(&kv); err != nil {
					return fmt.Errorf("decoding gob: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("getting value: %w", err)
			}

			resKVs = append(resKVs, kv)

		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("performing view txn: %w", err)
	}

	return resKVs, nil
}
