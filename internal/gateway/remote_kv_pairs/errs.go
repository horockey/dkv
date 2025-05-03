package remote_kv_pairs

import "fmt"

var _ error = KeyNotFoundError{}

type KeyNotFoundError struct {
	key string
}

func (err KeyNotFoundError) Error() string {
	return fmt.Sprintf("key %s not found", err.key)
}
