package badger_local_kv_pairs

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/prometheus/client_golang/prometheus"
)

var _ local_kv_pairs.Repository[fmt.Stringer, any] = &badgerLocalKVPairs[fmt.Stringer, any]{}

const (
	modSuffix       = "---modified---"
	tombstoneSuffix = "---tombstone---"
)

type badgerLocalKVPairs[K fmt.Stringer, V any] struct {
	db            *badger.DB
	tombstonesTTL time.Duration
	metrics       *metrics
}

func New[K fmt.Stringer, V any](
	db *badger.DB,
	tombstonesTTL time.Duration,
) *badgerLocalKVPairs[K, V] {
	return &badgerLocalKVPairs[K, V]{
		db:            db,
		tombstonesTTL: tombstonesTTL,
		metrics:       newMetrics(db),
	}
}

func (repo *badgerLocalKVPairs[K, V]) Metrics() []prometheus.Collector {
	return repo.metrics.list()
}

func (repo *badgerLocalKVPairs[K, V]) Get(key K) (resKV model.KVPair[K, V], resErr error) {
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

	res := model.KVPair[K, V]{
		Key: key,
	}
	if err := repo.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key.String()))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return local_kv_pairs.KeyNotFoundError{Key: key.String()}
			}
			return fmt.Errorf("getting item: %w", err)
		}

		if err := item.Value(func(val []byte) error {
			if err := gob.
				NewDecoder(bytes.NewBuffer(val)).
				Decode(&res.Value); err != nil {
				return fmt.Errorf("decoding gob: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("getting value: %w", err)
		}

		modItem, err := txn.Get([]byte(key.String() + modSuffix))
		if err != nil {
			return fmt.Errorf("getting modItem: %w", err)
		}

		var modUnix int64
		if err := modItem.Value(func(val []byte) error {
			if err := gob.
				NewDecoder(bytes.NewBuffer(val)).
				Decode(&modUnix); err != nil {
				return fmt.Errorf("decoding gob: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("getting value of modItem: %w", err)
		}

		res.Modified = time.Unix(modUnix, 0)

		return nil
	}); err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("reading ftom db: %w", err)
	}

	return res, nil
}

func (repo *badgerLocalKVPairs[K, V]) GetNoValue(key K) (resKV model.KVPair[K, V], resErr error) {
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

	res := model.KVPair[K, V]{
		Key: key,
	}
	if err := repo.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key.String()))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return local_kv_pairs.KeyNotFoundError{Key: key.String()}
			}
			return fmt.Errorf("getting item: %w", err)
		}

		modItem, err := txn.Get([]byte(key.String() + modSuffix))
		if err != nil {
			return fmt.Errorf("getting modItem: %w", err)
		}

		var modUnix int64
		if err := modItem.Value(func(val []byte) error {
			if err := gob.
				NewDecoder(bytes.NewBuffer(val)).
				Decode(&modUnix); err != nil {
				return fmt.Errorf("decoding gob: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("getting value of modItem: %w", err)
		}

		res.Modified = time.Unix(modUnix, 0)

		return nil
	}); err != nil {
		return model.KVPair[K, V]{}, fmt.Errorf("reading ftom db: %w", err)
	}

	return res, nil
}

// Updates kvp.
// If repo has newer version of it, will return local_kv_pairs.KvTooOldError.
//
// nolint: gocognit
// TODO: refactor to multiple cognitive acceptable funcs
func (repo *badgerLocalKVPairs[K, V]) AddOrUpdate(kvp model.KVPair[K, V], mf model.Merger[K, V]) (resErr error) {
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
		var inRepoTs time.Time
		modItem, err := txn.Get([]byte(kvp.Key.String() + modSuffix))

		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
			break
		case err == nil:
			var modUnix int64

			if err := modItem.Value(func(val []byte) error {
				if err := gob.
					NewDecoder(bytes.NewBuffer(val)).
					Decode(&modUnix); err != nil {
					return fmt.Errorf("decoding gob: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("getting value of modItem: %w", err)
			}

			inRepoTs = time.Unix(modUnix, 0)
			if inRepoTs.After(kvp.Modified) {
				return local_kv_pairs.KvTooOldError{
					Key:         kvp.Key.String(),
					InsertingTs: kvp.Modified,
					InRepoTs:    inRepoTs,
				}
			}
		}

		var oldValue V = *new(V)

		item, err := txn.Get([]byte(kvp.Key.String()))
		switch {
		case errors.Is(err, badger.ErrKeyNotFound):
			break
		case err != nil:
			return fmt.Errorf("getting item: %w", err)
		default:
			if err := item.Value(func(val []byte) error {
				if err := gob.
					NewDecoder(bytes.NewBuffer(val)).
					Decode(&oldValue); err != nil {
					return fmt.Errorf("decoding gob: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("getting value: %w", err)
			}
		}

		mergedKvp := mf.Merge(
			model.KVPair[K, V]{
				Key:      kvp.Key,
				Value:    oldValue,
				Modified: inRepoTs,
			},
			kvp,
		)

		buf := bytes.NewBuffer(nil)
		if err := gob.
			NewEncoder(buf).
			Encode(mergedKvp.Value); err != nil {
			return fmt.Errorf("encoding gob: %w", err)
		}

		val := bytes.Clone(buf.Bytes())
		buf.Reset()
		if err := txn.Set([]byte(mergedKvp.Key.String()), val); err != nil {
			return fmt.Errorf("setting item to db: %w", err)
		}

		if err := gob.NewEncoder(buf).Encode(mergedKvp.Modified.Unix()); err != nil {
			return fmt.Errorf("encoding modItem: %w", err)
		}
		if err := txn.Set([]byte(mergedKvp.Key.String()+modSuffix), buf.Bytes()); err != nil {
			return fmt.Errorf("setting modItem to db: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("performing upd txn: %w", err)
	}

	return nil
}

func (repo *badgerLocalKVPairs[K, V]) Remove(key K) (resErr error) {
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
		if err := txn.Delete([]byte(key.String())); err != nil {
			return fmt.Errorf("deleting item: %w", err)
		}

		if err := txn.Delete([]byte(key.String() + modSuffix)); err != nil {
			return fmt.Errorf("deleting modItem: %w", err)
		}

		buf := bytes.NewBuffer(nil)
		if err := gob.NewEncoder(buf).Encode(time.Now().Unix()); err != nil {
			return fmt.Errorf("encoding gob tombstone: %w", err)
		}

		e := badger.Entry{
			Key:   []byte(key.String() + tombstoneSuffix),
			Value: buf.Bytes(),
		}

		if err := txn.SetEntry(e.WithTTL(repo.tombstonesTTL)); err != nil {
			return fmt.Errorf("setting tombstone: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("performing del txn: %w", err)
	}

	return nil
}

func (repo *badgerLocalKVPairs[K, V]) CheckTombstone(key K) (ts int64, resErr error) {
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

	if err := repo.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key.String() + tombstoneSuffix))
		if err != nil {
			return fmt.Errorf("getting tombstone item: %w", err)
		}

		if err := item.Value(func(val []byte) error {
			if err := gob.NewDecoder(bytes.NewBuffer(val)).Decode(&ts); err != nil {
				return fmt.Errorf("decoding gob tombstone: %w", err)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("getting tombstone item value: %w", err)
		}

		return nil
	}); err != nil {
		return 0, fmt.Errorf("repforming view txn: %w", err)
	}

	return ts, nil
}

func (repo *badgerLocalKVPairs[K, V]) GetAllNoValue() (resKVs []model.KVPair[K, V], resErr error) {
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

	res := []model.KVPair[K, V]{}

	err := repo.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {

			//nolint: forcetypeassert, errcheck
			key := fmt.Stringer(model.MockStringer(string(it.Item().Key()))).(K)

			_, err := txn.Get([]byte(key.String()))
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					return local_kv_pairs.KeyNotFoundError{Key: key.String()}
				}
				return fmt.Errorf("getting item: %w", err)
			}

			modItem, err := txn.Get([]byte(key.String() + modSuffix))
			if err != nil {
				return fmt.Errorf("getting modItem: %w", err)
			}

			var modUnix int64
			if err := modItem.Value(func(val []byte) error {
				if err := gob.
					NewDecoder(bytes.NewBuffer(val)).
					Decode(&modUnix); err != nil {
					return fmt.Errorf("decoding gob: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("getting value of modItem: %w", err)
			}

			res = append(res, model.KVPair[K, V]{
				Key:      key,
				Modified: time.Unix(modUnix, 0),
			})

		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("performing view txn: %w", err)
	}

	return res, nil
}
