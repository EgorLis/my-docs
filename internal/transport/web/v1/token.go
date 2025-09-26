package v1

import (
	"net/http"
	"strings"
)

func TokenFromRequest(r *http.Request) string {
	// 1) form/URL param "token" по ТЗ
	if t := r.FormValue("token"); t != "" {
		return t
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	// 2) Authorization: Bearer ...
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
