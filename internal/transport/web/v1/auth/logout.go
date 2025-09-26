package auth

import (
	"log"
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
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
	if r.Method != http.MethodDelete {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}

	// извлекаем токен: при роутинге "DELETE /api/auth/" — хвост после префикса
	raw := getTokenFromPathOrHeader(r)
	if raw == "" {
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	claims, err := h.Tokens.Parse(r.Context(), raw)
	if err != nil {
		h.Log.Printf("logout: parse token: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// ревокация до exp
	if err := h.Blacklist.Revoke(r.Context(), claims.JTI, claims.ExpiresAt); err != nil {
		h.Log.Printf("logout: revoke: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

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
