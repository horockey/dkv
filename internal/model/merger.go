package model

type Merger[V any] interface {
	Merge(a, b KVPair[V]) KVPair[V]
}

type MergeFunc[V any] func(a, b KVPair[V]) KVPair[V]

func (mf MergeFunc[V]) Merge(a, b KVPair[V]) KVPair[V] {
	return mf(a, b)
}

func LastTsMerge[V any](a, b KVPair[V]) KVPair[V] {
	if a.Modified.After(b.Modified) {
		return a
	}
	return b
}
