package health

import (
	"context"
	"log"
	"net/http"
	"time"

	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

type DBPinger interface {
	Ping(context.Context) error
}

type Handler struct {
	Log *log.Logger
	DBPinger
}

// Liveness godoc
// @Summary      Liveness probe
// @Description  Проверка, жив ли сервис (не зависит от БД)
// @Tags         health
// @Produce      json
// @Success      200  {string}  string  "ok"
// @Router       /v1/healthz [get]
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	v1.WriteJSON(w, http.StatusOK, "ok")
}

// Readiness godoc
// @Summary      Readiness probe
// @Description  Проверка готовности сервиса (включая пинг базы данных)
// @Tags         health
// @Produce      json
// @Success      200  {string}  string  "ready"
// @Failure      503  {object}  map[string]string
// @Router       /v1/readyz [get]
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5000*time.Millisecond)
	defer cancel()

	err := h.DBPinger.Ping(ctx)

	if err != nil {
		h.Log.Printf("readiness error: %v", err)
		v1.WriteError(w, http.StatusServiceUnavailable, "")
		return
	}

	v1.WriteJSON(w, http.StatusOK, "ready")
}
