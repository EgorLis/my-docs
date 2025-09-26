package web

import (
	"log"
	"net/http"
	"time"
)

func loggingMiddleware(next http.Handler, logger *log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// оборачиваем ResponseWriter чтобы перехватить код ответа
		lrw := &loggingResponseWriter{w, http.StatusOK}
		next.ServeHTTP(lrw, r)

		logger.Printf("%s %s %d %s %s",
			r.Proto,
			r.Method,
			lrw.statusCode,
			r.URL.Path,
			time.Since(start),
		)
	})
}

// helper для перехвата статуса
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
