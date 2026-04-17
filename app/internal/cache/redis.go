package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache[K any, V any] struct {
	client      *redis.Client
	ttl         time.Duration
	keyToString func(K) string
}

func NewRedisCache[K any, V any](
	client *redis.Client,
	ttl time.Duration,
	keyToString func(K) string,
) *redisCache[K, V] {
	return &redisCache[K, V]{
		client:      client,
		ttl:         ttl,
		keyToString: keyToString,
	}
}

func (r *redisCache[K, V]) Get(ctx context.Context, key K) (V, bool) {
	var value V
	redisKey := r.keyToString(key)

	val, err := r.client.Get(ctx, redisKey).Result()
	if err != nil {
		return value, false
	}

	if err := json.Unmarshal([]byte(val), &value); err != nil {
		return value, false
	}

	return value, true
}

func (r *redisCache[K, V]) Set(ctx context.Context, key K, value V) {
	redisKey := r.keyToString(key)

	data, err := json.Marshal(value)
	if err != nil {
		return
	}

	r.client.Set(ctx, redisKey, data, r.ttl)
}

func (r *redisCache[K, V]) Delete(ctx context.Context, key K) {
	r.client.Del(ctx, r.keyToString(key))
}
