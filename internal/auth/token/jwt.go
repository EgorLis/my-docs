package token

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/EgorLis/my-docs/internal/domain"
)

type Manager struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func New(secret string, issuer string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// внутренний тип для подписи/парсинга с jwt.RegisteredClaims
type jwtClaims struct {
	JTI    string    `json:"jti"`
	UserID uuid.UUID `json:"uid"`
	Login  string    `json:"login"`
	jwt.RegisteredClaims
}

// Ensure: Manager implements domain.TokenManager
var _ domain.TokenManager = (*Manager)(nil)

// Issue выпускает JWT и возвращает доменные клеймы
func (m *Manager) Issue(_ context.Context, userID domain.UserID, login string) (domain.Token, domain.TokenClaims, error) {
	now := time.Now().UTC()
	jti := uuid.NewString()

	cl := jwtClaims{
		JTI:    jti,
		UserID: userID,
		Login:  login,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, cl)
	tokenStr, err := t.SignedString(m.secret)
	if err != nil {
		return "", domain.TokenClaims{}, err
	}

	return tokenStr, domain.TokenClaims{
		JTI:       cl.JTI,
		UserID:    cl.UserID,
		Login:     cl.Login,
		IssuedAt:  cl.IssuedAt.Time,
		ExpiresAt: cl.ExpiresAt.Time,
	}, nil
}

// Parse валидирует подпись/сроки и возвращает доменные клеймы
func (m *Manager) Parse(_ context.Context, raw domain.Token) (domain.TokenClaims, error) {
	var out jwtClaims
	tkn, err := jwt.ParseWithClaims(string(raw), &out, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return domain.TokenClaims{}, err
	}
	if !tkn.Valid {
		return domain.TokenClaims{}, jwt.ErrTokenInvalidClaims
	}

	return domain.TokenClaims{
		JTI:       out.JTI,
		UserID:    out.UserID,
		Login:     out.Login,
		IssuedAt:  out.IssuedAt.Time,
		ExpiresAt: out.ExpiresAt.Time,
	}, nil
}
