package doc

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	srt "sort"
	"strings"
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

// формирует стабильный ключ для кэша списка
func listCacheKey(me domain.UserID, login, key, value, sort string, limit int) string {
	parts := []string{
		"user=" + me.String(),
		"login=" + login,
		"key=" + key,
		"value=" + value,
		"sort=" + sort,
		fmt.Sprintf("limit=%d", limit),
	}
	srt.Strings(parts)
	return "list:" + sha256hex(strings.Join(parts, "&"))
}

func docMetaKey(id domain.DocID) string { return "docmeta:" + id.String() }
func docJSONKey(id domain.DocID) string { return "docjson:" + id.String() }

func sha256hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// простая нормализация key/sort из ТЗ
func normalizeSort(s string) string {
	switch s {
	case "name", "created":
		return s
	default:
		return "name"
	}
}

// для safety: url.PathUnescape id из path-параметра
func unescape(s string) string {
	u, _ := url.PathUnescape(s)
	return u
}
