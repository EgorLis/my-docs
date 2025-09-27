package blacklist

import (
	"context"
	"time"

	"github.com/EgorLis/my-docs/internal/domain"
)

// KV — минимальный интерфейс, который нам нужен от кеша.
type KV interface {
	SetNX(ctx context.Context, key string, val []byte, ttlSeconds int) (bool, error)
	Exists(ctx context.Context, key string) (bool, error)
}

type Store struct {
	kv     KV
	prefix string
}

func NewStore(kv KV, prefix string) *Store {
	if prefix == "" {
		prefix = "jti:"
	}
	return &Store{kv: kv, prefix: prefix}
}

func (s *Store) key(jti string) string { return domain.CacheKeyTokenJTI(jti) }

// Revoke помечает jti отозванным до времени exp (TTL = exp-now).
func (s *Store) Revoke(ctx context.Context, jti string, exp time.Time) error {
	ttl := time.Until(exp)
	if ttl <= 0 {
		ttl = time.Minute // подстраховка, если exp в прошлом
	}
	_, err := s.kv.SetNX(ctx, s.key(jti), []byte("1"), int(ttl.Seconds()))
	return err
}

func (s *Store) IsRevoked(ctx context.Context, jti string) (bool, error) {
	return s.kv.Exists(ctx, s.key(jti))
}
