package dto

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/horockey/dkv/internal/model"
)

type KV struct {
	Key          string `json:"key"`
	ValueB64     string `json:"value"`
	ModifiedUnix int64  `json:"modified"`
}

func KVToModel[V any](kv KV) (model.KVPair[V], error) {
	res := model.KVPair[V]{
		Key:      kv.Key,
		Modified: time.Unix(kv.ModifiedUnix, 0),
	}

	data, err := base64.StdEncoding.DecodeString(kv.ValueB64)
	if err != nil {
		return model.KVPair[V]{}, fmt.Errorf("decoding base64: %w", err)
	}

	if err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(&res.Value); err != nil {
		return model.KVPair[V]{}, fmt.Errorf("decoding gob value: %w", err)
	}

	return res, nil
}

func NewKV[V any](kvp model.KVPair[V]) (KV, error) {
	buf := bytes.NewBuffer(nil)
	if err := gob.NewEncoder(buf).Encode(kvp.Value); err != nil {
		return KV{}, fmt.Errorf("encoding gob value: %w", err)
	}

	return KV{
		Key:          kvp.Key,
		ValueB64:     base64.StdEncoding.EncodeToString(buf.Bytes()),
		ModifiedUnix: kvp.Modified.Unix(),
	}, nil
}
