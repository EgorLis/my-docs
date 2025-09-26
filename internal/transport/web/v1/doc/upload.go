package doc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

// Upload godoc
// @Summary     Upload new document
// @Description multipart: meta(json), json(optional), file(optional)
// @Tags        docs
// @Accept      multipart/form-data
// @Produce     json
// @Param token query string false "Auth token (alternative to Authorization: Bearer)"
// @Param       meta formData string true  "JSON meta"
// @Param       json formData string false "JSON body"
// @Param       file formData file   false "file"
// @Success     200 {object} domain.APIEnvelope{data=object}
// @Failure     400 {object} domain.APIEnvelope
// @Failure     401 {object} domain.APIEnvelope
// @Failure     500 {object} domain.APIEnvelope
// @Router      /api/docs [post]
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}
	// аутентификация (требуем Bearer)
	me, ok := mw.UserFromCtx(r.Context())
	if !ok {
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		h.Log.Printf("upload parse form: %v", err)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// meta — JSON с полями из ТЗ
	var metaIn struct {
		Name   string   `json:"name"`
		File   bool     `json:"file"`
		Public bool     `json:"public"`
		Token  string   `json:"token"` // игнорируем (Bearer уже проверен)
		Mime   string   `json:"mime"`
		Grant  []string `json:"grant"`
	}
	if s := r.FormValue("meta"); s != "" {
		if err := json.Unmarshal([]byte(s), &metaIn); err != nil {
			h.Log.Printf("upload meta json: %v", err)
			v1.WriteDomainError(w, r, domain.ErrBadParams)
			return
		}
	}

	// json — опциональный документ
	var jsonBody domain.DocJSON
	if js := r.FormValue("json"); js != "" {
		if err := json.Unmarshal([]byte(js), &jsonBody); err != nil {
			h.Log.Printf("upload json body: %v", err)
			v1.WriteDomainError(w, r, domain.ErrBadParams)
			return
		}
	}

	// file — опционально
	var (
		fileReader io.Reader
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
		// загрузка в сторидж
		res, err := h.Storage.Put(r.Context(), fh, filename, mime)
		if err != nil {
			h.Log.Printf("upload storage put: %v", err)
			v1.WriteDomainError(w, r, domain.ErrUnexpected)
			return
		}
		storageKey, size, shaSum = res.StorageKey, res.Size, res.SHA256
		fileReader = bytes.NewReader(nil) // не нужен дальше
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

	// создаём мета в БД
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
		h.Log.Printf("upload db create: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	// шаринг (grant)
	for _, login := range metaIn.Grant {
		_ = h.Shares.UpsertReadGrant(r.Context(), doc.ID, login, true)
	}

	// инвалидация кэша списков владельца
	_ = h.Cache.Del(r.Context(), "list:"+me.ID.String()+":*") // если поддерживаешь pattern — хорошо; иначе можно версионировать префикс

	// ответ по ТЗ
	out := map[string]any{
		"json": jsonBody,
	}
	if metaIn.File {
		out["file"] = doc.Name
	}
	v1.WriteOKData(w, r, out)
	_ = fileReader // только чтобы не ругался линтер
}
