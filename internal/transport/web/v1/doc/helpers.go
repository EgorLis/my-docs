package doc

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/EgorLis/my-docs/internal/domain"
)

func weakETag(version int64, sha []byte) string {
	pref := hex.EncodeToString(sha)
	if len(pref) > 8 {
		pref = pref[:8]
	}
	return fmt.Sprintf(`W/"%d-%s"`, version, pref)
}

func httpTime(t time.Time) string { return t.UTC().Format(time.RFC1123) }

// pageKey = хэш фильтров/сортировки/лимита, чтобы был компактный и стабильный
func makeListPageKey(login, key, val, sort string, limit int) string {
	h := sha1.New()
	// важно: явно разделять поля
	io.WriteString(h, "login="+login+";")
	io.WriteString(h, "key="+key+";")
	io.WriteString(h, "val="+val+";")
	io.WriteString(h, "sort="+sort+";")
	io.WriteString(h, fmt.Sprintf("limit=%d;", limit))
	return hex.EncodeToString(h.Sum(nil))
}

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// принимает строку из query (?sort=...) и мапит к domain.ListSort.
// дефолт: created_desc
func normalizeSort(s string) domain.ListSort {
	switch s {
	case "name_asc":
		return domain.SortByNameAsc
	case "name_desc":
		return domain.SortByNameDesc
	case "created_asc":
		return domain.SortByCreatedAsc
	case "created_desc":
		return domain.SortByCreatedDesc
	default:
		return domain.SortByCreatedDesc
	}
}

// для safety: url.PathUnescape id из path-параметра
func unescape(s string) string {
	u, _ := url.PathUnescape(s)
	return u
}
