package doc

import (
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/logx"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
	"github.com/google/uuid"
)

// Delete godoc
// @Summary     Delete document (owner only)
// @Tags        docs
// @Param token query string false "Auth token (alternative to Authorization: Bearer)"
// @Param       id path string true "document id"
// @Success     200 {object} domain.APIEnvelope{response=object}
// @Failure     401 {object} domain.APIEnvelope
// @Failure     403 {object} domain.APIEnvelope
// @Failure     404 {object} domain.APIEnvelope
// @Router      /api/docs/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	const op = "docs.delete"
	reqID := mw.RequestIDFromCtx(r.Context())
	logx.Info(h.Log, reqID, op, "start", "method", r.Method, "path", r.URL.Path)

	if r.Method != http.MethodDelete {
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

	idStr := strings.TrimPrefix(r.URL.Path, "/api/docs/")
	idStr = unescape(idStr)
	docID, err := uuid.Parse(idStr)
	if err != nil {
		logx.Error(h.Log, reqID, op, "bad doc id", err, "doc_id_raw", idStr)
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// для удаления нам нужен storageKey → подтянем метаданные без ACL (или с ACL как владелец)
	d, _, err := h.Docs.DocByID(r.Context(), docID, &me)
	if err != nil {
		logx.Error(h.Log, reqID, op, "doc not found", err, "doc_id", docID)
		v1.WriteDomainError(w, r, domain.ErrNotFound)
		return
	}
	if d.OwnerID != me.ID {
		logx.Error(h.Log, reqID, op, "forbidden (not owner)", domain.ErrForbidden, "doc_id", d.ID, "owner_id", d.OwnerID, "me", me.ID)
		v1.WriteDomainError(w, r, domain.ErrForbidden)
		return
	}

	// сначала удаляем из storage (не критично, если объекта нет)
	_ = h.Storage.Delete(r.Context(), d.StorageKey)

	// затем из БД
	if err := h.Docs.DocDelete(r.Context(), d.ID, me.ID); err != nil {
		logx.Error(h.Log, reqID, op, "db delete failed", err, "doc_id", d.ID)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	// инвалидация кэша
	_ = h.Cache.Del(r.Context(),
		domain.CacheKeyDocMeta(d.ID),
		domain.CacheKeyDocJSON(d.ID),
	)
	_ = h.Cache.Del(r.Context(), domain.CacheKeyDocList(me.ID.String(), "*"))

	logx.Info(h.Log, reqID, op, "ok", "doc_id", d.ID)
	v1.WriteOKResponse(w, r, map[string]bool{d.ID.String(): true})
}
