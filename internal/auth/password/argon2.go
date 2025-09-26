package password

import (
	"errors"

	"github.com/alexedwards/argon2id"
)

type Hasher struct {
	params *argon2id.Params
}

func NewDefault() *Hasher {
	// параметры по умолчанию (достаточно безопасны и не слишком тяжёлые)
	return &Hasher{params: argon2id.DefaultParams}
}

func New(p *argon2id.Params) *Hasher { return &Hasher{params: p} }

// Hash возвращает закодированную строку формата $argon2id$v=19$m=..., которую можно хранить в БД.
func (h *Hasher) Hash(plain string) (string, error) {
	if h == nil || h.params == nil {
		return "", errors.New("argon2id params not set")
	}
	return argon2id.CreateHash(plain, h.params)
}

// Verify сравнивает пароль с сохранённым хэшем.
func (h *Hasher) Verify(plain, encodedHash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(plain, encodedHash)
}
