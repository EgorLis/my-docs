package doc

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

// List godoc
// @Summary     List documents
// @Tags        docs
// @Produce     json
// @Param       login query string false "owner login (optional)"
// @Param       key   query string false "filter key (name|mime)"
// @Param       value query string false "filter value"
// @Param       limit query int    false "limit"
// @Param       sort  query string false "name|created"
// @Success     200 {object} domain.APIEnvelope{data=object}
// @Failure     401 {object} domain.APIEnvelope
// @Router      /api/docs [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}
	me, ok := mw.UserFromCtx(r.Context())
	if !ok {
		// по ТЗ требуется token — подразумеваем Bearer
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	login := r.URL.Query().Get("login")
	key := r.URL.Query().Get("key")
	val := r.URL.Query().Get("value")
	sortQ := normalizeSort(r.URL.Query().Get("sort"))
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	// кэш
	ckey := listCacheKey(me.ID, login, key, val, sortQ, limit)
	b, err := h.Cache.Get(r.Context(), ckey)
	if err != nil {
		h.Log.Printf("cache get list: %v", err)
	} else if b != nil {
		w.Header().Set("Cache-Control", "private, max-age=60")
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	}

	// в БД
	f := domain.ListFilter{
		Login: login, Key: key, Value: val, Limit: limit,
	}
	switch sortQ {
	case "name":
		f.Sort = domain.SortByNameAsc
	case "created":
		f.Sort = domain.SortByCreatedDesc
	}
	docs, err := h.Docs.DocsList(r.Context(), me, f)
	if err != nil {
		h.Log.Printf("list: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	// привести к формату ТЗ (grant логины)
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
	buf, _ := json.Marshal(env)
	_ = h.Cache.Set(r.Context(), ckey, buf, h.ListTTL)

	// HEAD без тела
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	v1.WriteEnvelope(w, r, http.StatusOK, env)
}
