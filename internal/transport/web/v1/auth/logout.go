package auth

import (
	"log"
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

type HandlerLogout struct {
	Log       *log.Logger
	Tokens    domain.TokenManager
	Blacklist domain.TokenBlacklist
}

type logoutResponse struct {
	Revoked string `json:"revoked"` // jti
}

// Logout godoc
// @Summary     Logout (revoke token)
// @Description Завершает сессию: помечает токен как отозванный до истечения exp.
// @Tags        auth
// @Produce     json
// @Param       token path string true "JWT token (raw)"
// @Success     200 {object} domain.APIEnvelope{response=logoutResponse}
// @Failure     400 {object} domain.APIEnvelope
// @Failure     401 {object} domain.APIEnvelope
// @Failure     500 {object} domain.APIEnvelope
// @Router      /api/auth/{token} [delete]
func (h *HandlerLogout) Logout(w http.ResponseWriter, r *http.Request) {
	const op = "auth.logout"
	reqID := mw.RequestIDFromCtx(r.Context())
	logx.Info(h.Log, reqID, op, "start", "method", r.Method, "path", r.URL.Path)

	if r.Method != http.MethodDelete {
		logx.Error(h.Log, reqID, op, "method not allowed", domain.ErrMethodNotAllowed, "method", r.Method)
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}

	// извлекаем токен: при роутинге "DELETE /api/auth/" — хвост после префикса
	raw := getTokenFromPathOrHeader(r)
	if raw == "" {
		logx.Error(h.Log, reqID, op, "missing token", domain.ErrBadParams)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	claims, err := h.Tokens.Parse(r.Context(), raw)
	if err != nil {
		logx.Error(h.Log, reqID, op, "parse token failed", err)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// ревокация до exp
	if err := h.Blacklist.Revoke(r.Context(), claims.JTI, claims.ExpiresAt); err != nil {
		logx.Error(h.Log, reqID, op, "revoke failed", err, "jti", claims.JTI)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	logx.Info(h.Log, reqID, op, "ok", "jti", claims.JTI)
	v1.WriteOKResponse(w, r, logoutResponse{Revoked: claims.JTI})
}

func getTokenFromPathOrHeader(r *http.Request) string {
	// 1) DELETE /api/auth/{token}
	const pfx = "/api/auth/"
	if strings.HasPrefix(r.URL.Path, pfx) && len(r.URL.Path) > len(pfx) {
		return r.URL.Path[len(pfx):]
	}
	// 2) query ?token=...
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	// 3) Authorization: Bearer ...
	return v1.TokenFromRequest(r)
}
