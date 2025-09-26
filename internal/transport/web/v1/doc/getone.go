package doc

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
	"github.com/google/uuid"
)

// GetOne godoc
// @Summary     Get single document or file
// @Tags        docs
// @Produce     json
// @Param token query string false "Auth token (alternative to Authorization: Bearer)"
// @Param       id path string true "document id"
// @Success     200 {object} domain.APIEnvelope
// @Success     200 {file}  []byte "when file"
// @Failure     401 {object} domain.APIEnvelope
// @Failure     404 {object} domain.APIEnvelope
// @Router      /api/docs/{id} [get]
func (h *Handler) GetOne(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}
	me, ok := mw.UserFromCtx(r.Context())
	if !ok {
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	// id из path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/docs/")
	idStr = unescape(idStr)
	docID, err := uuid.Parse(idStr)
	if err != nil {
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// Быстрая проверка по кэшу метаданных: если ETag совпал — 304
	if b, err := h.Cache.Get(r.Context(), docMetaKey(docID)); err == nil && len(b) > 0 {
		var cached domain.Document
		if err := json.Unmarshal(b, &cached); err == nil {
			etag := weakETag(cached.Version, cached.SHA256)
			if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
				w.Header().Set("ETag", etag)
				w.Header().Set("Last-Modified", httpTime(cached.UpdatedAt))
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	} else if err != nil {
		h.Log.Printf("cache get docmeta: %v", err)
	}

	// Достаём актуальные метаданные (с ACL) и JSON
	meUser := me
	d, dj, err := h.Docs.DocByID(r.Context(), docID, &meUser)
	if err != nil {
		h.Log.Printf("get doc by id: %v", err)
		v1.WriteDomainError(w, r, domain.ErrNotFound)
		return
	}

	// Кэшируем мету
	if buf, err := json.Marshal(d); err == nil {
		_ = h.Cache.Set(r.Context(), docMetaKey(d.ID), buf, h.DocTTL)
	}

	// Готовим общие заголовки
	etag := weakETag(d.Version, d.SHA256)
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", httpTime(d.UpdatedAt))
	w.Header().Set("Cache-Control", "private, max-age=60")

	// Conditional по ETag (If-None-Match)
	if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Если документ — файл: поддерживаем Range и HEAD
	if d.File {
		// HEAD: отдаём только заголовки (MIME знаем из метаданных)
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Type", d.MIME)
			w.WriteHeader(http.StatusOK)
			return
		}

		// GET: поддержка Range
		rangeHdr := r.Header.Get("Range")
		rc, contentLen, contentRange, contentType, _ /*s3etag*/, err := h.Storage.Get(r.Context(), d.StorageKey, rangeHdr)
		if err != nil {
			h.Log.Printf("storage get: %v", err)
			v1.WriteDomainError(w, r, domain.ErrUnexpected)
			return
		}
		defer rc.Close()

		w.Header().Set("Accept-Ranges", "bytes")
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		} else {
			w.Header().Set("Content-Type", d.MIME)
		}
		w.Header().Set("Content-Length", strconv.FormatInt(contentLen, 10))

		if contentRange != "" {
			w.Header().Set("Content-Range", contentRange)
			w.WriteHeader(http.StatusPartialContent) // 206
		} else {
			w.WriteHeader(http.StatusOK) // 200
		}

		_, _ = io.Copy(w, rc)
		return
	}

	// Иначе — JSON-контент
	if dj != nil {
		// Кэшируем готовый конверт data
		env := domain.OkData(dj)
		if buf, err := json.Marshal(env); err == nil {
			_ = h.Cache.Set(r.Context(), docJSONKey(d.ID), buf, h.DocTTL)
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(buf)
			return
		}
		// fallback, если маршал не удался
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		v1.WriteOKData(w, r, dj)
		return
	}

	// Нет JSON-тела — возвращаем пустой data
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	v1.WriteOKData(w, r, map[string]any{})
}
