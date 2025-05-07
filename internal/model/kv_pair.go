package model

import (
	"fmt"
	"time"
)

type KVPair[K fmt.Stringer, V any] struct {
	Key      K
	Value    V
	Modified time.Time
}
