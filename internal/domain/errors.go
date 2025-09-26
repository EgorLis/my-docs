package domain

import "errors"

// Бизнес-ошибки (маппятся на HTTP коды по правилам из ТЗ)
var (
	ErrBadParams        = errors.New("bad_params")         // 400
	ErrUnauth           = errors.New("unauthorized")       // 401
	ErrForbidden        = errors.New("forbidden")          // 403
	ErrNotFound         = errors.New("not_found")          // 404 (в ТЗ нет, но удобно внутри; наружу всё равно 200 с error?)
	ErrMethodNotAllowed = errors.New("method_not_allowed") // 405
	ErrNotImplemented   = errors.New("not_implemented")    // 501
	ErrUnexpected       = errors.New("unexpected")         // 500
)
