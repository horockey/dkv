package badger_local_kv_pairs_test

import (
	"bytes"
	"encoding/gob"
	"errors"
	"testing"
	"time"

	"github.com/dgraph-io/badger"
	"github.com/horockey/dkv/internal/model"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs"
	"github.com/horockey/dkv/internal/repository/local_kv_pairs/badger_local_kv_pairs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStringer string

func (m mockStringer) String() string {
	return string(m)
}

func setupDB(t *testing.T) (*badger.DB, func()) {
	dir := t.TempDir()

	db, err := badger.Open(badger.DefaultOptions(dir))
	if err != nil {
		t.Fatalf("failed to open badger db: %v", err)
	}

	return db, func() {
		_ = db.Close()
	}
}

func Test_Get_KeyNotFound(t *testing.T) {
	db, teardown := setupDB(t)
	defer teardown()

	repo := badger_local_kv_pairs.New[mockStringer, any](db)

	key := mockStringer("nonexistent_key")

	kv, err := repo.Get(key)

	assert.Empty(t, kv)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, local_kv_pairs.KeyNotFoundError{Key: key.String()}))
}

func Test_Get_Success(t *testing.T) {
	db, teardown := setupDB(t)
	defer teardown()

	repo := badger_local_kv_pairs.New[mockStringer, string](db)
	key := mockStringer("test_key")
	value := "test_value"

	err := repo.AddOrUpdate(model.KVPair[mockStringer, string]{
		Key:      key,
		Value:    value,
		Modified: time.Now(),
	})
	require.NoError(t, err)

	kv, err := repo.Get(key)
	require.NoError(t, err)

	assert.Equal(t, key, kv.Key)
	assert.Equal(t, value, kv.Value)
	assert.WithinDuration(t, time.Now(), kv.Modified, 2*time.Second)
}

func Test_GetNoValue_KeyNotFound(t *testing.T) {
	db, teardown := setupDB(t)
	defer teardown()

	repo := badger_local_kv_pairs.New[mockStringer, any](db)
	key := mockStringer("nonexistent_key")

	kv, err := repo.GetNoValue(key)

	assert.Empty(t, kv)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, local_kv_pairs.KeyNotFoundError{Key: key.String()}))
}

func Test_GetNoValue_Success(t *testing.T) {
	db, teardown := setupDB(t)
	defer teardown()

	repo := badger_local_kv_pairs.New[mockStringer, string](db)
	key := mockStringer("test_key")
	value := "test_value"

	err := repo.AddOrUpdate(model.KVPair[mockStringer, string]{
		Key:      key,
		Value:    value,
		Modified: time.Now(),
	})
	require.NoError(t, err)

	kv, err := repo.GetNoValue(key)

	require.NoError(t, err)
	assert.Equal(t, key, kv.Key)
	assert.Zero(t, kv.Value) // Значение не должно быть заполнено
	assert.NotZero(t, kv.Modified)
}

func Test_AddOrUpdate(t *testing.T) {
	db, teardown := setupDB(t)
	defer teardown()

	repo := badger_local_kv_pairs.New[mockStringer, string](db)
	key := mockStringer("test_key")
	kv := model.KVPair[mockStringer, string]{
		Key:   key,
		Value: "test_value",
	}

	err := repo.AddOrUpdate(kv)
	require.NoError(t, err)

	var storedValue string
	err = db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key.String()))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return gob.NewDecoder(bytes.NewBuffer(val)).Decode(&storedValue)
		})
	})

	require.NoError(t, err)
	assert.Equal(t, kv.Value, storedValue)
}

func Test_Remove(t *testing.T) {
	db, teardown := setupDB(t)
	defer teardown()

	repo := badger_local_kv_pairs.New[mockStringer, string](db)
	key := mockStringer("test_key")
	value := "test_value"

	err := repo.AddOrUpdate(model.KVPair[mockStringer, string]{
		Key:      key,
		Value:    value,
		Modified: time.Now(),
	})
	require.NoError(t, err)

	// Удаляем его
	err = repo.Remove(key)
	require.NoError(t, err)

	// Проверяем, что оно удалено
	err = db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key.String()))
		return err
	})

	assert.Error(t, err)
	assert.True(t, errors.Is(err, badger.ErrKeyNotFound))
}
