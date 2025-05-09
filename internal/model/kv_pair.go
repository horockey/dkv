package model

import (
	"time"
)

type KVPair[V any] struct {
	Key      string
	Value    V
	Modified time.Time
}
