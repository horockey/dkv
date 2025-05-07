package model

import "fmt"

type Merger[K fmt.Stringer, V any] interface {
	Merge(a, b KVPair[K, V]) KVPair[K, V]
}

type MergeFunc[K fmt.Stringer, V any] func(a, b KVPair[K, V]) KVPair[K, V]

func (mf MergeFunc[K, V]) Merge(a, b KVPair[K, V]) KVPair[K, V] {
	return mf(a, b)
}

func LastTsMerge[K fmt.Stringer, V any](a, b KVPair[K, V]) KVPair[K, V] {
	if a.Modified.Before(b.Modified) {
		return a
	}
	return b
}
