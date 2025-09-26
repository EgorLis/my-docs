package domain

import (
	"context"
	"time"
)

// Требования ТЗ:
// - /api/auth -> выдать токен
// - /api/auth/<token> [DELETE] -> завершить сессию (инвалидация)
// Предлагаем контракты менеджера токенов и хранилища «блэклиста».

type Token string

type TokenClaims struct {
	JTI       string // уникальный id токена
	UserID    UserID
	Login     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Хеширование паролей
type PasswordHasher interface {
	Hash(plain string) ([]byte, error)
	Verify(plain string, hash []byte) bool
}

// Управление токенами (JWT/PASETO — реализация где-нибудь в internal/auth)
type TokenManager interface {
	Issue(ctx context.Context, u User, ttl time.Duration) (Token, TokenClaims, error)
	Parse(ctx context.Context, t Token) (TokenClaims, error)
}

// Блэклист/ревокация токенов (например, Redis)
type TokenBlacklist interface {
	Revoke(ctx context.Context, jti string, exp time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
