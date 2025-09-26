package auth

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/EgorLis/my-docs/internal/domain"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

// HandlerRegister обрабатывает POST /api/register
type HandlerRegister struct {
	Log        *log.Logger
	Users      domain.UsersRepo
	Hasher     domain.PasswordHasher
	AdminToken string
}

type registerRequest struct {
	Token string `json:"token"` // админ-токен (из конфига)
	Login string `json:"login"`
	Pswd  string `json:"pswd"`
}

type registerResponse struct {
	Login string `json:"login"`
}

// Register godoc
// @Summary     Register new user
// @Description Регистрация нового пользователя (доступно только по admin-token из конфига).
// @Tags        auth
// @Accept      json
// @Produce     json
// @Param       request body registerRequest true "token, login, pswd"
// @Success     200 {object} domain.APIEnvelope{response=registerResponse}
// @Failure     400 {object} domain.APIEnvelope
// @Failure     401 {object} domain.APIEnvelope
// @Failure     405 {object} domain.APIEnvelope
// @Failure     500 {object} domain.APIEnvelope
// @Router      /api/register [post]
func (h *HandlerRegister) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}

	// Принимаем JSON, но поддержим и форму (на случай ручного теста).
	var req registerRequest
	ct := r.Header.Get("Content-Type")
	if ct != "" && (ct == "application/json" || ct[:16] == "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.Log.Printf("register: bad json: %v", err)
			v1.WriteDomainError(w, r, domain.ErrBadParams)
			return
		}
	} else {
		// form / query
		_ = r.ParseForm()
		req.Token = r.FormValue("token")
		req.Login = r.FormValue("login")
		req.Pswd = r.FormValue("pswd")
	}

	// 1) Проверка admin token
	if req.Token == "" || req.Token != h.AdminToken {
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// 2) Валидация логина/пароля (домен)
	if !domain.ValidLogin(req.Login) || !domain.ValidPassword(req.Pswd) {
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// 3) Хэш пароля
	hashStr, err := h.Hasher.Hash(req.Pswd)
	if err != nil {
		h.Log.Printf("register: hash err: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	// 4) Создаём пользователя
	u, err := h.Users.CreateUser(r.Context(), req.Login, []byte(hashStr))
	if err != nil {
		// возможен уникальный конфликт по login — маппим как bad params
		h.Log.Printf("register: create err: %v", err)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// 5) Ответ по конверту
	v1.WriteOKResponse(w, r, registerResponse{Login: u.Login})
}
