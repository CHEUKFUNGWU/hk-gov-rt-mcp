package hkapi

import (
	"context"
	"sync"
	"time"
)

// TTL presets shared by both implementations (mirrored in TypeScript).
const (
	TTLEta         = 20 * time.Second
	TTLWeatherNow  = 60 * time.Second
	TTLWarnings    = 60 * time.Second
	TTLForecast    = 10 * time.Minute
	TTLTraffic     = 2 * time.Minute
	TTLParking     = 60 * time.Second
	TTLStaticRoute = 6 * time.Hour
)

type cacheEntry struct {
	expires time.Time
	value   any
	err     error
}

var cache sync.Map // string -> cacheEntry

// Cached returns the cached value for key if fresh, otherwise calls load.
// A failed load is also cached briefly (30s) to avoid hammering a failing
// upstream on every call.
func Cached[T any](ctx context.Context, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	_ = ctx
	now := time.Now()
	if v, ok := cache.Load(key); ok {
		e := v.(cacheEntry)
		if e.expires.After(now) {
			if e.err != nil {
				var zero T
				return zero, e.err
			}
			return e.value.(T), nil
		}
	}
	value, err := load()
	ttlUsed := ttl
	if err != nil {
		ttlUsed = 30 * time.Second
		var zero T
		cache.Store(key, cacheEntry{expires: now.Add(ttlUsed), value: zero, err: err})
		return zero, err
	}
	cache.Store(key, cacheEntry{expires: now.Add(ttlUsed), value: value})
	return value, nil
}
