package domain

import "context"

// Ключи кеша — единое место, чтобы не расползались по коду.
func CacheKeyDocMeta(id DocID) string                    { return "docmeta:" + id.String() }
func CacheKeyDocJSON(id DocID) string                    { return "docjson:" + id.String() }
func CacheKeyDocList(user string, pageKey string) string { return "list:" + user + ":" + pageKey } // pageKey = хэш фильтров/сортировки
func CacheKeyTokenJTI(jti string) string                 { return "jti:" + jti }

// Простой k/v интерфейс. Реализация — Redis.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val []byte, ttlSeconds int) error
	Del(ctx context.Context, keys ...string) error
	// Для инкрементируемых версий списков (выборочная инвалидация)
	Incr(ctx context.Context, key string) (int64, error)
	Ping(context.Context) error
	Close()
}
