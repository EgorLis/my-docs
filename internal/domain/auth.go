package domain

import (
	"context"
	"time"
)

// Токен и клеймы
type Token = string

type TokenClaims struct {
	JTI       string
	UserID    UserID
	Login     string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Hash/Verify — строковые (argon2id)
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(plain, encodedHash string) (bool, error)
}

// JWT/PASETO менеджер — TTL конфигурируется при создании менеджера
type TokenManager interface {
	Issue(ctx context.Context, userID UserID, login string) (Token, TokenClaims, error)
	Parse(ctx context.Context, raw Token) (TokenClaims, error)
}

// Блэклист (logout)
type TokenBlacklist interface {
	Revoke(ctx context.Context, jti string, exp time.Time) error
	IsRevoked(ctx context.Context, jti string) (bool, error)
}
