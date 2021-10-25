package cache

import (
	"context"
	"time"
)

// CacheData cache data struct
type CacheData struct {
	ExpireAt int64  // ExpireAt 失效时间
	Data     string // Data 真实数据
}

// LoadFunc define load data func
type LoadFunc func(ctx context.Context, target interface{}) (err error)

type Cacher interface {
	// GetData load data from cache
	// if cache not exist load data by LoadFunc
	// expiration is redis server expiration
	// ttl is developer expiration
	GetData(ctx context.Context, key string, expiration time.Duration, ttl time.Duration, f LoadFunc, data interface{}) (err error)

	// FlushCache flush cache
	// if cache not exist, load data and save cache
	FlushCache(ctx context.Context, key string, expiration time.Duration, ttl time.Duration, f LoadFunc, data interface{}) (err error)
}