package doc

import (
	"net/http"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/EgorLis/my-docs/internal/transport/web/mw"
	v1 "github.com/EgorLis/my-docs/internal/transport/web/v1"
	"github.com/google/uuid"
)

// Delete godoc
// @Summary     Delete document (owner only)
// @Tags        docs
// @Param       id path string true "document id"
// @Success     200 {object} domain.APIEnvelope{response=object}
// @Failure     401 {object} domain.APIEnvelope
// @Failure     403 {object} domain.APIEnvelope
// @Failure     404 {object} domain.APIEnvelope
// @Router      /api/docs/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		v1.WriteDomainError(w, r, domain.ErrMethodNotAllowed)
		return
	}
	me, ok := mw.UserFromCtx(r.Context())
	if !ok {
		v1.WriteDomainError(w, r, domain.ErrUnauth)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/api/docs/")
	idStr = unescape(idStr)
	docID, err := uuid.Parse(idStr)
	if err != nil {
		v1.WriteDomainError(w, r, domain.ErrBadParams)
		return
	}

	// для удаления нам нужен storageKey → подтянем метаданные без ACL (или с ACL как владелец)
	d, _, err := h.Docs.DocByID(r.Context(), docID, &me)
	if err != nil {
		v1.WriteDomainError(w, r, domain.ErrNotFound)
		return
	}
	if d.OwnerID != me.ID {
		v1.WriteDomainError(w, r, domain.ErrForbidden)
		return
	}

	// сначала удаляем из storage (не критично, если объекта нет)
	_ = h.Storage.Delete(r.Context(), d.StorageKey)

	// затем из БД
	if err := h.Docs.DocDelete(r.Context(), d.ID, me.ID); err != nil {
		h.Log.Printf("delete db: %v", err)
		v1.WriteDomainError(w, r, domain.ErrUnexpected)
		return
	}

	// инвалидация кэша
	_ = h.Cache.Del(r.Context(),
		docMetaKey(d.ID),
		docJSONKey(d.ID),
	)
	_ = h.Cache.Del(r.Context(), "list:"+me.ID.String()+":*")

	v1.WriteOKResponse(w, r, map[string]bool{d.ID.String(): true})
}
