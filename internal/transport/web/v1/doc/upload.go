package doc

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

// MetaDTO описывает meta JSON.
type MetaDTO struct {
	Name   string   `json:"name"`
	File   bool     `json:"file"`
	Public bool     `json:"public"`
	Token  string   `json:"token"` // игнорируем
	Mime   string   `json:"mime"`
	Grant  []string `json:"grant"`
}

// Upload godoc
// @Summary     Upload new document
// @Description multipart/form-data: meta(JSON), json(JSON, optional), file(binary, optional)
// @Tags        docs
// @Accept      multipart/form-data
// @Produce     json
// @Param       token query string false "Auth token (alternative to Authorization: Bearer)"
// @Param       meta  formData string  true  "JSON meta"
// @Param       json  formData string false "Any JSON"
// @Param       file  formData file  false "Документ (макс. 1ГБ)"
// @Success     200 {object} domain.APIEnvelope{data=object}
// @Failure     400 {object} domain.APIEnvelope
// @Failure     401 {object} domain.APIEnvelope
// @Failure     500 {object} domain.APIEnvelope
// @Router      /api/docs [post]
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	const op = "docs.upload"
	reqID := mw.RequestIDFromCtx(r.Context())
	logx.Info(h.Log, reqID, op, "start", "method", r.Method, "path", r.URL.Path)

	if r.Method != http.MethodPost {
		logx.Error(h.Log, reqID, op, "method not allowed", domain.ErrMethodNotAllowed)
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}
	// аутентификация (требуем Bearer)
	me, ok := mw.UserFromCtx(r.Context())
	if !ok {
		logx.Error(h.Log, reqID, op, "unauthorized", domain.ErrUnauth)
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		logx.Error(h.Log, reqID, op, "parse multipart failed", err)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	var metaIn MetaDTO
	if s := r.FormValue("meta"); s != "" {
		if err := json.Unmarshal([]byte(s), &metaIn); err != nil {
			logx.Error(h.Log, reqID, op, "meta json invalid", err)
			v1.WriteDomainError(w, r, domain.ErrBadParams)
			return
		}
	}

	// json — опциональный документ
	var jsonBody domain.DocJSON
	if js := r.FormValue("json"); js != "" {
		if err := json.Unmarshal([]byte(js), &jsonBody); err != nil {
			logx.Error(h.Log, reqID, op, "body json invalid", err)
			v1.WriteDomainError(w, r, domain.ErrBadParams)
			return
		}
	}

	// file — опционально
	var (
		filename   string
		mime       = metaIn.Mime
		size       int64
		shaSum     []byte
		storageKey string
	)
	if fh, hdr, err := r.FormFile("file"); err == nil {
		defer fh.Close()
		metaIn.File = true
		filename = hdr.Filename
		if mime == "" {
			mime = hdr.Header.Get("Content-Type")
			if mime == "" {
				mime = "application/octet-stream"
			}
		}
		// загрузка в хранилище
		res, err := h.Storage.Put(r.Context(), fh, filename, mime)
		if err != nil {
			logx.Error(h.Log, reqID, op, "storage put failed", err, "filename", filename, "mime", mime)
			v1.WriteDomainError(w, r, domain.ErrUnexpected)
			return
		}
		storageKey, size, shaSum = res.StorageKey, res.Size, res.SHA256
		_ = bytes.NewReader(nil)
	} else {
		metaIn.File = false
	}

	if metaIn.Name == "" {
		metaIn.Name = filename
	}
	if metaIn.Name == "" {
		metaIn.Name = "document"
	}
	if mime == "" {
		mime = "application/octet-stream"
	}

	// создаём мету в БД
	doc, err := h.Docs.CreateDoc(r.Context(), domain.Document{
		OwnerID:    me.ID,
		Name:       metaIn.Name,
		MIME:       mime,
		File:       metaIn.File,
		Public:     metaIn.Public,
		SizeBytes:  size,
		StorageKey: storageKey,
		SHA256:     shaSum,
	}, jsonBody)
	if err != nil {
		logx.Error(h.Log, reqID, op, "db create doc failed", err, "name", metaIn.Name, "mime", mime, "file", metaIn.File)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	// шаринг (grant)
	for _, login := range metaIn.Grant {
		_ = h.Shares.UpsertReadGrant(r.Context(), doc.ID, login, true)
	}

	// инвалидация кэша списков владельца
	_ = h.Cache.Del(r.Context(), domain.CacheKeyDocList(me.ID.String(), "*"))

	// ответ по ТЗ
	out := map[string]any{"json": jsonBody}
	if metaIn.File {
		out["file"] = doc.Name
	}
	logx.Info(h.Log, reqID, op, "ok", "doc_id", doc.ID, "name", doc.Name, "size", size)
	v1.WriteOKData(w, r, out)
}
