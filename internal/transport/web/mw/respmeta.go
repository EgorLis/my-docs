package mw

import "net/http"

// metaWriter перехватывает статус/размер ответа
type metaWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (m *metaWriter) WriteHeader(code int) {
	m.status = code
	m.ResponseWriter.WriteHeader(code)
}

func (m *metaWriter) Write(b []byte) (int, error) {
	if m.status == 0 {
		m.status = http.StatusOK
	}
	n, err := m.ResponseWriter.Write(b)
	m.size += n
	return n, err
}
