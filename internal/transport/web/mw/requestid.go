package mw

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey string

const (
	CtxRequestID ctxKey = "req_id"
	HeaderReqID  string = "X-Request-ID"
)

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(HeaderReqID)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set(HeaderReqID, reqID)
		ctx := context.WithValue(r.Context(), CtxRequestID, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(CtxRequestID).(string); ok {
		return v
	}
	return ""
}
