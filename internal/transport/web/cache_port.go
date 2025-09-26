package web

import "context"

type Cache interface {
	Ping(ctx context.Context) error
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, val []byte, ttlSeconds int) error
	Del(ctx context.Context, keys ...string) error
}
