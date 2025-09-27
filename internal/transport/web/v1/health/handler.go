package health

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

type Pinger interface {
	Ping(context.Context) error
}

type Handler struct {
	Log     *log.Logger
	DB      Pinger
	Cache   Pinger
	Storage Pinger
}

// Liveness godoc
// @Summary      Liveness probe
// @Description  Проверка, жив ли сервис (не зависит от БД/кэша)
// @Tags         health
// @Produce      json
// @Success      200  {object}  domain.APIEnvelope{data=string}
// @Router       /api/healthz [get]
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	const op = "health.liveness"
	reqID := mw.RequestIDFromCtx(r.Context())

	logx.Info(h.Log, reqID, op, "ok")
	v1.WriteOKData(w, r, "ok")
}

// Readiness godoc
// @Summary      Readiness probe
// @Description  Проверка готовности сервиса (включая пинг БД и Redis)
// @Tags         health
// @Produce      json
// @Success      200  {object}  domain.APIEnvelope{data=string}
// @Failure      503  {object}  domain.APIEnvelope
// @Router       /api/readyz [get]
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	const op = "health.readiness"
	reqID := mw.RequestIDFromCtx(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.DB.Ping(ctx); err != nil {
		logx.Error(h.Log, reqID, op, "db ping failed", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	if err := h.Cache.Ping(ctx); err != nil {
		logx.Error(h.Log, reqID, op, "cache ping failed", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	if err := h.Storage.Ping(ctx); err != nil {
		logx.Error(h.Log, reqID, op, "storage ping failed", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	logx.Info(h.Log, reqID, op, "ready")
	v1.WriteOKData(w, r, "ready")
}
