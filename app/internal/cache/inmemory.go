package cache

import (
	"context"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

type lruCache[K comparable, V any] struct {
	storage *expirable.LRU[K, V]
}

func NewInMemoryLRUCache[K comparable, V any](size int, duration time.Duration) *lruCache[K, V] {
	return &lruCache[K, V]{storage: expirable.NewLRU[K, V](size, nil, duration)}
}

func (l *lruCache[K, V]) Get(_ context.Context, key K) (V, bool) {
	return l.storage.Get(key)
}

func (l *lruCache[K, V]) Set(_ context.Context, key K, value V) {
	l.storage.Add(key, value)
}

func (l *lruCache[K, V]) Delete(_ context.Context, key K) {
	l.storage.Remove(key)
}
