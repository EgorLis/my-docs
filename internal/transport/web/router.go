package web

import (
	"log"
	"net/http"

	_ "github.com/EgorLis/my-docs/internal/docs"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/blob"
	"github.com/EgorLis/my-docs/internal/transport/web/v1/health"
	httpSwagger "github.com/swaggo/http-swagger"
)

func newRouter(hh *health.Handler, bh *blob.Handler, logger *log.Logger) http.Handler {
	mux := http.NewServeMux()

	// health
	mux.HandleFunc("GET /v1/healthz", hh.Liveness)
	mux.HandleFunc("GET /v1/readyz", hh.Readiness)

	// blob test
	mux.HandleFunc("POST /v1/blob", limitBody(64<<20, bh.Upload)) // 64MB лимит
	mux.HandleFunc("DELETE /v1/blob", bh.Delete)                  // ?key=sha256%2F...

	// swagger
	mux.Handle("GET /swagger/", httpSwagger.WrapHandler)

	// 🔗 middleware
	return mw.WithRequestID(mw.Logging(logger)(mux))
}

func limitBody(n int64, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, n)
		h(w, r)
	}
}
