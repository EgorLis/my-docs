package domain

import (
	"time"

	"github.com/google/uuid"
)

// Базовые идентификаторы
type UserID = uuid.UUID
type DocID = uuid.UUID

// Пользователь
type User struct {
	ID        UserID    `json:"id"`
	Login     string    `json:"login"`
	PassHash  []byte    `json:"-"` // никогда не отдаём наружу
	CreatedAt time.Time `json:"created_at"`
}

// Метаданные документа (без тела файла)
type Document struct {
	ID        DocID     `json:"id"`
	OwnerID   UserID    `json:"owner_id"`
	Name      string    `json:"name"`
	MIME      string    `json:"mime"`
	File      bool      `json:"file"`   // true: есть бинарный файл; false: только JSON
	Public    bool      `json:"public"` // общий доступ (чтение всем)
	CreatedAt time.Time `json:"created"`
	UpdatedAt time.Time `json:"updated"`

	// Технические поля для выдачи/кеша/ETag
	SizeBytes int64  `json:"size_bytes"`
	SHA256    []byte `json:"-"` // контент-хэш (для ETag)
	Version   int64  `json:"-"` // версионирование метаданных

	// Где лежит контент (локально/S3/MinIO)
	StorageKey string `json:"-"`
}

// Шаринг: доступ на чтение конкретному пользователю
type DocShare struct {
	DocID DocID  `json:"doc_id"`
	Login string `json:"login"` // логин пользователя, которому дан доступ
}

// Произвольный JSON документа (если File=false или в дополнение к файлу)
type DocJSON map[string]any
