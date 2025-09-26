package blob

import (
	"context"
	"io"
	"log"
	"mime/multipart"
	"net/http"

	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
)

type Storage interface {
	Put(ctx context.Context, r io.Reader, hintName, mime string) (string, int64, []byte, error)
	Delete(ctx context.Context, storageKey string) error
}

type Handler struct {
	Log     *log.Logger
	Storage Storage
}

// POST /v1/blob
// multipart/form-data: file=<file>
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		h.Log.Printf("upload parse form: %v", err)
		v1.WriteError(w, http.StatusBadRequest, "invalid multipart")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		h.Log.Printf("upload form file: %v", err)
		v1.WriteError(w, http.StatusBadRequest, "missing file")
		return
	}
	defer file.Close()

	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = detectMime(header)
	}
	key, size, _, err := h.Storage.Put(r.Context(), file, header.Filename, mime)
	if err != nil {
		h.Log.Printf("upload put: %v", err)
		v1.WriteError(w, http.StatusInternalServerError, "put failed")
		return
	}
	v1.WriteJSON(w, http.StatusOK, map[string]any{
		"key":  key,
		"size": size,
	})
}

func detectMime(h *multipart.FileHeader) string {
	// можно улучшить: прочитать первые байты
	ct := h.Header.Get("Content-Type")
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

// DELETE /v1/blob/{key}
// key передаём как query (?key=...) или path-параметр — здесь сделаем через query.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		v1.WriteError(w, http.StatusBadRequest, "missing key")
		return
	}
	if err := h.Storage.Delete(r.Context(), key); err != nil {
		h.Log.Printf("delete: %v", err)
		v1.WriteError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	v1.WriteJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
		"key":     key,
	})
}
