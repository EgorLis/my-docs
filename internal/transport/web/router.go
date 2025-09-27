package web

import (
	"log"
	"net/http"
	"strings"

	_ "github.com/EgorLis/my-docs/internal/docs"
	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/auth"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/doc"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/health"
	httpSwagger "github.com/swaggo/http-swagger"
)

func newRouter(s *Server) http.Handler {
	healthLog := log.New(s.logger.Writer(), s.logger.Prefix()+"[health] ", s.logger.Flags())
	authLog := log.New(s.logger.Writer(), s.logger.Prefix()+"[auth] ", s.logger.Flags())
	docsLog := log.New(s.logger.Writer(), s.logger.Prefix()+"[docs] ", s.logger.Flags())

	hh := &health.Handler{
		DB:      s.repos.Users,
		Cache:   s.cache,
		Storage: s.store,
		Log:     healthLog,
	}

	reg := &auth.HandlerRegister{
		Log:        authLog,
		Users:      s.repos.Users,
		Hasher:     s.auth.Hasher,
		AdminToken: s.cfg.AdminToken,
	}

	loginH := &auth.HandlerLogin{
		Log:    authLog,
		Users:  s.repos.Users,
		Hasher: s.auth.Hasher,
		Tokens: s.auth.Tokens,
	}

	logoutH := &auth.HandlerLogout{
		Log:       authLog,
		Tokens:    s.auth.Tokens,
		Blacklist: s.auth.Blacklist,
	}

	dh := &doc.Handler{
		Log:     docsLog,
		Users:   s.repos.Users,
		Docs:    s.repos.Docs,
		Shares:  s.repos.Shares,
		Storage: s.store,
		Cache:   s.cache,
		ListTTL: 60, // сек
		DocTTL:  60,
	}

	mux := http.NewServeMux()

	// health
	mux.HandleFunc("GET /v1/healthz", hh.Liveness)
	mux.HandleFunc("GET /v1/readyz", hh.Readiness)

	// auth
	mux.HandleFunc("POST /api/register", reg.Register)
	mux.HandleFunc("POST /api/auth", loginH.Login)
	mux.HandleFunc("DELETE /api/auth/", logoutH.Logout) // DELETE /api/auth/{token}

	// защищаем Bearer-ом приватные ручки:
	// Upload, List, GetOne, Delete
	protected := mw.RequireAuth(mw.AuthDeps{Tokens: s.auth.Tokens, Blacklist: s.auth.Blacklist}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/docs":
			limitBody(1<<30, dh.Upload)(w, r) // Ограничение на 1ГБ
		case (r.Method == http.MethodGet || r.Method == http.MethodHead) && r.URL.Path == "/api/docs":
			dh.List(w, r)
		case (r.Method == http.MethodGet || r.Method == http.MethodHead) && strings.HasPrefix(r.URL.Path, "/api/docs/"):
			dh.GetOne(w, r)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/docs/"):
			dh.Delete(w, r)
		default:
			v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		}
	}))
	mux.Handle("/api/docs", protected)
	mux.Handle("/api/docs/", protected)

	// swagger
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)

	// 🔗 middleware
	return mw.WithRequestID(mw.Logging(s.logger)(mux))
}

func limitBody(n int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		h(w, r)
	}
}
