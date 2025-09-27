package doc

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
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
	const op = "docs.get_one"
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

	// id из path
	idStr := strings.TrimPrefix(r.URL.Path, "/api/docs/")
	idStr = unescape(idStr)
	docID, err := uuid.Parse(idStr)
	if err != nil {
		logx.Error(h.Log, reqID, op, "bad doc id", err, "doc_id_raw", idStr)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// кэш метаданных → ETag short-circuit
	if b, err := h.Cache.Get(r.Context(), domain.CacheKeyDocMeta(docID)); err == nil && len(b) > 0 {
		var cached domain.Document
		if err := json.Unmarshal(b, &cached); err == nil {
			etag := weakETag(cached.Version, cached.SHA256)
			if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
				w.Header().Set("ETag", etag)
				w.Header().Set("Last-Modified", httpTime(cached.UpdatedAt))
				w.WriteHeader(http.StatusNotModified)
				logx.Info(h.Log, reqID, op, "not modified by etag", "doc_id", cached.ID)
				return
			}
		}
	} else if err != nil {
		logx.Error(h.Log, reqID, op, "cache get docmeta failed", err, "doc_id", docID)
	}

	// Достаём актуальные метаданные (с ACL) и JSON
	meUser := me
	d, dj, err := h.Docs.DocByID(r.Context(), docID, &meUser)
	if err != nil {
		logx.Error(h.Log, reqID, op, "db doc not found/acl", err, "doc_id", docID)
		v1.WriteDomainError(w, r, domain.ErrNotFound)
		return
	}

	// Кэшируем мету
	if buf, err := json.Marshal(d); err == nil {
		_ = h.Cache.Set(r.Context(), domain.CacheKeyDocMeta(d.ID), buf, h.DocTTL)
	}

	// Готовим общие заголовки
	etag := weakETag(d.Version, d.SHA256)
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", httpTime(d.UpdatedAt))
	w.Header().Set("Cache-Control", "private, max-age=60")

	// Conditional по ETag (If-None-Match)
	if inm := r.Header.Get("If-None-Match"); inm != "" && inm == etag {
		w.WriteHeader(http.StatusNotModified)
		logx.Info(h.Log, reqID, op, "not modified by etag (db)", "doc_id", d.ID)
		return
	}

	// Если документ — файл: поддерживаем Range и HEAD
	if d.File {
		// HEAD: отдаём только заголовки (MIME знаем из метаданных)
		if r.Method == http.MethodHead {
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Content-Type", d.MIME)
			w.WriteHeader(http.StatusOK)
			logx.Info(h.Log, reqID, op, "head file ok", "doc_id", d.ID, "mime", d.MIME)
			return
		}

		// GET: поддержка Range
		rangeHdr := r.Header.Get("Range")
		rc, contentLen, contentRange, contentType, _, err := h.Storage.Get(r.Context(), d.StorageKey, rangeHdr)
		if err != nil {
			logx.Error(h.Log, reqID, op, "storage get failed", err, "doc_id", d.ID, "range", rangeHdr)
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
			w.WriteHeader(http.StatusPartialContent)
			logx.Info(h.Log, reqID, op, "partial content", "doc_id", d.ID, "range", contentRange, "len", contentLen)
		} else {
			w.WriteHeader(http.StatusOK)
			logx.Info(h.Log, reqID, op, "file ok", "doc_id", d.ID, "len", contentLen)
		}

		_, _ = io.Copy(w, rc)
		return
	}

	// Иначе — JSON-контент
	if dj != nil {
		// Кэшируем готовый конверт data
		env := domain.OkData(dj)
		if buf, err := json.Marshal(env); err == nil {
			_ = h.Cache.Set(r.Context(), domain.CacheKeyDocJSON(d.ID), buf, h.DocTTL)
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusOK)
				logx.Info(h.Log, reqID, op, "head json ok", "doc_id", d.ID)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(buf)
			logx.Info(h.Log, reqID, op, "json ok", "doc_id", d.ID, "bytes", len(buf))
			return
		}
		// fallback, если маршал не удался
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			logx.Info(h.Log, reqID, op, "head json ok (fallback)", "doc_id", d.ID)
			return
		}
		logx.Info(h.Log, reqID, op, "json ok (fallback)", "doc_id", d.ID)
		v1.WriteOKData(w, r, dj)
		return
	}

	// Нет JSON-тела — возвращаем пустой data
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		logx.Info(h.Log, reqID, op, "head ok (empty data)", "doc_id", d.ID)
		return
	}
	logx.Info(h.Log, reqID, op, "ok (empty data)", "doc_id", d.ID)
	v1.WriteOKData(w, r, map[string]any{})
}
