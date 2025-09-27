package redisx

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	rdb    *redis.Client
	logger *log.Logger
}

type Config struct {
	Addr     string
	DB       int
	Password string
}

func New(cfg Config, logger *log.Logger) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		DB:       cfg.DB,
		Password: cfg.Password,
	})
	return &Cache{rdb: rdb, logger: logger}
}

func (c *Cache) Ping(ctx context.Context) error {
	err := c.rdb.Ping(ctx).Err()
	if err != nil {
		c.logger.Printf("PING failed: %v", err)
	} else {
		c.logger.Println("PING ok")
	}
	return err
}

func (c *Cache) Close() {
	if c.rdb == nil {
		c.logger.Println("nothing to close")
		return
	}

	if err := c.rdb.Close(); err != nil {
		c.logger.Printf("error while closing: %v", err)
		return
	}

	c.logger.Println("closed")
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		c.logger.Printf("GET %q: not found", key)
		return nil, nil
	}
	if err != nil {
		c.logger.Printf("GET %q: error: %v", key, err)
	} else {
		c.logger.Printf("GET %q: hit (%d bytes)", key, len(b))
	}
	return b, err
}

func (c *Cache) Set(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	var ttl time.Duration
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	err := c.rdb.Set(ctx, key, val, ttl).Err()
	if err != nil {
		c.logger.Printf("SET %q failed: %v", key, err)
	} else {
		c.logger.Printf("SET %q ok (ttl=%s)", key, ttl)
	}
	return err
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	n, err := c.rdb.Del(ctx, keys...).Result()
	if err != nil {
		c.logger.Printf("DEL %v failed: %v", keys, err)
	} else {
		c.logger.Printf("DEL %v: deleted=%d", keys, n)
	}
	return err
}

func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	n, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		c.logger.Printf("INCR %q failed: %v", key, err)
	} else {
		c.logger.Printf("INCR %q -> %d", key, n)
	}
	return n, err
}

// SetNX устанавливает значение только если ключ ещё не существует.
func (c *Cache) SetNX(ctx context.Context, key string, val []byte, ttlSeconds int) (bool, error) {
	var ttl time.Duration
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	ok, err := c.rdb.SetNX(ctx, key, val, ttl).Result()
	if err != nil {
		c.logger.Printf("SETNX %q failed: %v", key, err)
	} else if ok {
		c.logger.Printf("SETNX %q ok (ttl=%s)", key, ttl)
	} else {
		c.logger.Printf("SETNX %q skipped (already exists)", key)
	}
	return ok, err
}

// Exists проверяет наличие ключа.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		c.logger.Printf("EXISTS %q failed: %v", key, err)
		return false, err
	}
	if n == 1 {
		c.logger.Printf("EXISTS %q: true", key)
		return true, nil
	}
	c.logger.Printf("EXISTS %q: false", key)
	return false, nil
}
