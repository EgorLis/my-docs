package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
)

// MapDomainError решает HTTP-статус + error.code/text для конверта
func MapDomainError(err error) (httpStatus int, env domain.APIEnvelope) {
	switch {
	case errors.Is(err, domain.ErrBadParams):
		return http.StatusBadRequest, domain.Fail(domain.ErrCodeBadParams, "bad params")
	case errors.Is(err, domain.ErrUnauth):
		return http.StatusUnauthorized, domain.Fail(domain.ErrCodeUnauth, "unauthorized")
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, domain.Fail(domain.ErrCodeForbidden, "forbidden")
	case errors.Is(err, domain.ErrMethodNotAllowed):
		return http.StatusMethodNotAllowed, domain.Fail(domain.ErrCodeMethodNotAllowed, "method not allowed")
	case errors.Is(err, domain.ErrNotImplemented):
		return http.StatusNotImplemented, domain.Fail(domain.ErrCodeNotImplemented, "not implemented")
	case errors.Is(err, domain.ErrNotFound):
		// В ТЗ нет 404, но на уровне HTTP это корректнее; если хочешь всегда 200 — можно вернуть 200 и error.
		return http.StatusNotFound, domain.Fail(domain.ErrCodeNotFound, "not found")
	default:
		// Таймауты/отмены — как 500
		return http.StatusInternalServerError, domain.Fail(domain.ErrCodeUnexpected, "unexpected")
	}
}

// WriteEnvelope пишет конверт; для HEAD — без тела
func WriteEnvelope(w http.ResponseWriter, r *http.Request, status int, env domain.APIEnvelope) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", mw.RequestIDFromCtx(r.Context()))
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(env)
}

// Шорткаты успеха
func WriteOKData(w http.ResponseWriter, r *http.Request, data any) {
	WriteEnvelope(w, r, http.StatusOK, domain.OkData(data))
}
func WriteOKResponse(w http.ResponseWriter, r *http.Request, resp any) {
	WriteEnvelope(w, r, http.StatusOK, domain.OkResponse(resp))
}

// Шорткаты ошибок
func WriteDomainError(w http.ResponseWriter, r *http.Request, err error) {
	status, env := MapDomainError(err)
	WriteEnvelope(w, r, status, env)
}

// Стандартный формат времени заголовков
func HTTPTime(t time.Time) string {
	return t.UTC().Format(http.TimeFormat)
}
