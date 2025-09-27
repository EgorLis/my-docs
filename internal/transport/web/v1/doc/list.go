package doc

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

// List godoc
// @Summary     List documents
// @Tags        docs
// @Produce     json
// @Param token query string false "Auth token (alternative to Authorization: Bearer)"
// @Param       login query string false "owner login (optional)"
// @Param       key   query string false "filter key (name|mime)"
// @Param       value query string false "filter value"
// @Param       limit query int    false "limit"
// @Param       sort  query string false "Sort order" Enums(name_asc, name_desc, created_asc, created_desc)
// @Success     200 {object} domain.APIEnvelope{data=object}
// @Failure     401 {object} domain.APIEnvelope
// @Router      /api/docs [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	const op = "docs.list"
	reqID := mw.RequestIDFromCtx(r.Context())
	logx.Info(h.Log, reqID, op, "start", "method", r.Method, "path", r.URL.Path)

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		logx.Error(h.Log, reqID, op, "method not allowed", domain.ErrMethodNotAllowed)
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}
	me, ok := mw.UserFromCtx(r.Context())
	if !ok {
		logx.Error(h.Log, reqID, op, "unauthorized", domain.ErrUnauth)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	login := r.URL.Query().Get("login")
	key := r.URL.Query().Get("key")
	val := r.URL.Query().Get("value")

	// Новые значения сортировки
	sortRaw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort")))
	sortVal := normalizeSort(sortRaw) // -> domain.ListSort

	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	// кэш-ключ теперь включает новое значение сортировки
	pageKey := makeListPageKey(login, key, val, string(sortVal), limit)
	ckey := domain.CacheKeyDocList(me.ID.String(), pageKey)
	// кеш-хит
	if b, err := h.Cache.Get(r.Context(), ckey); err == nil && b != nil {
		w.Header().Set("Cache-Control", "private, max-age=60")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			logx.Info(h.Log, reqID, op, "head from cache ok", "user_id", me.ID, "bytes", len(b))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		logx.Info(h.Log, reqID, op, "from cache ok", "user_id", me.ID, "bytes", len(b))
		return
	}

	// запрос к БД
	f := domain.ListFilter{
		Login: login, Key: key, Value: val, Limit: limit, Sort: sortVal,
	}

	docs, err := h.Docs.DocsList(r.Context(), me, f)
	if err != nil {
		logx.Error(h.Log, reqID, op, "db list failed", err, "user_id", me.ID)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	type docOut struct {
		ID      string   `json:"id"`
		Name    string   `json:"name"`
		Mime    string   `json:"mime"`
		File    bool     `json:"file"`
		Public  bool     `json:"public"`
		Created string   `json:"created"`
		Grant   []string `json:"grant"`
	}
	out := struct {
		Docs []docOut `json:"docs"`
	}{Docs: make([]docOut, 0, len(docs))}

	for _, d := range docs {
		gr, _ := h.Shares.ListGrantedLogins(r.Context(), d.ID)
		out.Docs = append(out.Docs, docOut{
			ID: d.ID.String(), Name: d.Name, Mime: d.MIME,
			File: d.File, Public: d.Public,
			Created: d.CreatedAt.Format("2006-01-02 15:04:05"),
			Grant:   gr,
		})
	}

	env := domain.OkData(out)
	if buf, err := json.Marshal(env); err == nil {
		_ = h.Cache.Set(r.Context(), ckey, buf, h.ListTTL)
	}

	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		logx.Info(h.Log, reqID, op, "head ok", "user_id", me.ID, "count", len(out.Docs), "sort", sortVal)
		return
	}
	logx.Info(h.Log, reqID, op, "ok", "user_id", me.ID, "count", len(out.Docs), "sort", sortVal)
	v1.WriteEnvelope(w, r, http.StatusOK, env)
}
