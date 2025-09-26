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
		raw := extractTokenAny(r) // <-- ТУТ
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := deps.Tokens.Parse(r.Context(), raw)
		if err != nil {
			next.ServeHTTP(w, r)
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
		raw := extractTokenAny(r) // <-- ТУТ
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

// Берём токен из query (?token=...), а если его нет — из Authorization: Bearer ...
func extractTokenAny(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" { // не трогаем тело (без ParseForm), безопасно для multipart
		return t
	}
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
