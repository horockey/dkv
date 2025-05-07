package local_kv_pairs

import (
	"fmt"
	"time"
)

var _ error = KeyNotFoundError{}

type KeyNotFoundError struct {
	Key string
}

func (err KeyNotFoundError) Error() string {
	return fmt.Sprintf("key %s not found", err.Key)
}

type KvTooOldError struct {
	Key         string
	InsertingTs time.Time
	InRepoTs    time.Time
}

func (err KvTooOldError) Error() string {
	return fmt.Sprintf(
		"too old mod ts for %s: %s, in repo: %s",
		err.Key,
		err.InsertingTs.Format(time.RFC3339),
		err.InRepoTs.Format(time.RFC3339),
	)
}
