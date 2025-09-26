package domain

// Общий конверт ответа по ТЗ
type APIError struct {
	Code int    `json:"code,omitempty"`
	Text string `json:"text,omitempty"`
}

type APIEnvelope struct {
	Error    *APIError `json:"error,omitempty"`
	Response any       `json:"response,omitempty"`
	Data     any       `json:"data,omitempty"`
}

// Утилиты для сборки конвертов
func OkResponse(resp any) APIEnvelope { return APIEnvelope{Response: resp} }
func OkData(data any) APIEnvelope     { return APIEnvelope{Data: data} }
func Fail(code int, text string) APIEnvelope {
	return APIEnvelope{Error: &APIError{Code: code, Text: text}}
}
