package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

type HandlerLogin struct {
	Log    *log.Logger
	Users  domain.UsersRepo
	Hasher domain.PasswordHasher
	Tokens domain.TokenManager
}

type loginRequest struct {
	Login string `json:"login"`
	Pswd  string `json:"pswd"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login godoc
// @Summary     Authenticate user
// @Description Возвращает JWT при валидных логине и пароле.
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body loginRequest true "login, pswd"
// @Success     200 {object} domain.APIEnvelope{response=loginResponse}
// @Failure     400 {object} domain.APIEnvelope
// @Failure     401 {object} domain.APIEnvelope
// @Failure     500 {object} domain.APIEnvelope
// @Router      /api/auth [post]
func (h *HandlerLogin) Login(w http.ResponseWriter, r *http.Request) {
	const op = "auth.login"
	reqID := mw.RequestIDFromCtx(r.Context())
	logx.Info(h.Log, reqID, op, "start", "method", r.Method, "path", r.URL.Path)

	if r.Method != http.MethodPost {
		logx.Error(h.Log, reqID, op, "method not allowed", domain.ErrMethodNotAllowed, "method", r.Method)
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}

	var req loginRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logx.Error(h.Log, reqID, op, "bad json", err)
			v1.WriteDomainError(w, r, domain.ErrBadParams)
			return
		}
	} else {
		_ = r.ParseForm()
		req.Login = r.FormValue("login")
		req.Pswd = r.FormValue("pswd")
	}

	// простая проверка наличия полей (строгая валидация логина/пароля была на регистрации)
	if req.Login == "" || req.Pswd == "" {
		logx.Error(h.Log, reqID, op, "empty login or pswd", domain.ErrBadParams)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// достаём пользователя
	u, err := h.Users.UserByLogin(r.Context(), req.Login)
	if err != nil {
		logx.Error(h.Log, reqID, op, "user not found", err, "login", req.Login)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// сверяем пароль
	ok, err := h.Hasher.Verify(req.Pswd, string(u.PassHash))
	if err != nil || !ok {
		logx.Error(h.Log, reqID, op, "password verify failed", err, "login", req.Login)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// выдаём токен
	token, _, err := h.Tokens.Issue(r.Context(), u.ID, u.Login)
	if err != nil {
		logx.Error(h.Log, reqID, op, "issue token failed", err, "user_id", u.ID, "login", u.Login)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	logx.Info(h.Log, reqID, op, "ok", "user_id", u.ID, "login", u.Login)
	v1.WriteOKResponse(w, r, loginResponse{Token: token})
}
