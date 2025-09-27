package mw

import (
	"log"
	"net/http"
	"time"
)

// Logging — middleware: старт/финиш запроса, статус, размер, длительность
func Logging(l *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqID := RequestIDFromCtx(r.Context())
			start := time.Now()

			mw := &metaWriter{ResponseWriter: w}

			next.ServeHTTP(mw, r)

			dur := time.Since(start)
			l.Printf("lvl=info req_id=%s method=%s path=%q status=%d size=%d duration_ms=%d",
				reqID, r.Method, r.URL.Path, mw.status, mw.size, dur.Milliseconds())
		})
	}
}
