package mw

import (
	"context"
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
)

const userKey ctxKey = "auth_user"

type AuthDeps struct {
	Tokens    domain.TokenManager
	Blacklist domain.TokenBlacklist
}

func OptionalAuth(deps AuthDeps, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := extractBearer(r.Header.Get("Authorization"))
		if raw == "" {
			next.ServeHTTP(w, r) // без пользователя
			return
		}
		claims, err := deps.Tokens.Parse(r.Context(), raw)
		if err != nil {
			next.ServeHTTP(w, r) // не валидный — просто идём как неавторизованный
			return
		}
		if revoked, _ := deps.Blacklist.IsRevoked(r.Context(), claims.JTI); revoked {
			next.ServeHTTP(w, r)
			return
		}
		u := domain.User{ID: claims.UserID, Login: claims.Login}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAuth(deps AuthDeps, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := extractBearer(r.Header.Get("Authorization"))
		if raw == "" {
			http.Error(w, `{"error":{"code":1001,"text":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		claims, err := deps.Tokens.Parse(r.Context(), raw)
		if err != nil {
			http.Error(w, `{"error":{"code":1001,"text":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		if revoked, _ := deps.Blacklist.IsRevoked(r.Context(), claims.JTI); revoked {
			http.Error(w, `{"error":{"code":1001,"text":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		u := domain.User{ID: claims.UserID, Login: claims.Login}
		ctx := context.WithValue(r.Context(), userKey, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromCtx(ctx context.Context) (domain.User, bool) {
	u, ok := ctx.Value(userKey).(domain.User)
	return u, ok
}

func extractBearer(h string) string {
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
