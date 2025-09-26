package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
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
	if r.Method != http.MethodPost {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}

	var req loginRequest
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.Log.Printf("auth: bad json: %v", err)
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
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// достаём пользователя
	u, err := h.Users.UserByLogin(r.Context(), req.Login)
	if err != nil {
		h.Log.Printf("auth: user not found: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// сверяем пароль
	ok, err := h.Hasher.Verify(req.Pswd, string(u.PassHash))
	if err != nil || !ok {
		if err != nil {
			h.Log.Printf("auth: verify err: %v", err)
		}
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// выдаём токен
	token, _, err := h.Tokens.Issue(r.Context(), u.ID, u.Login)
	if err != nil {
		h.Log.Printf("auth: issue token err: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	v1.WriteOKResponse(w, r, loginResponse{Token: token})
}
