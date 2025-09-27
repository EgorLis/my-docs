package domain

import (
	"context"
	"time"
)

// Фильтры и пагинация списков
type ListSort string

const (
	SortByNameAsc     ListSort = "name_asc"
	SortByNameDesc    ListSort = "name_desc"
	SortByCreatedDesc ListSort = "created_desc"
	SortByCreatedAsc  ListSort = "created_asc"
)

// Фильтрация по произвольному key=value (из ТЗ)
type ListFilter struct {
	Login string // если пусто — свои; если задан — по указанному пользователю (учитывая ACL/public)
	Key   string // имя колонки
	Value string // значение
	Limit int    // ограничение количества
	Sort  ListSort
	// Кейсет пагинация (рекомендовано под нагрузку)
	AfterName    string
	AfterCreated time.Time
	AfterID      DocID
}

type UsersRepo interface {
	Close()
	Ping(context.Context) error
	CreateUser(ctx context.Context, login string, passHash []byte) (User, error)
	UserByLogin(ctx context.Context, login string) (User, error)
	UserByID(ctx context.Context, id UserID) (User, error)
}

type DocsRepo interface {
	CreateDoc(ctx context.Context, meta Document, json DocJSON) (Document, error)
	// Возвращает метаданные и JSON (если есть). Контент — через BlobStorage.
	DocByID(ctx context.Context, id DocID, forUser *User) (Document, DocJSON, error)
	DocDelete(ctx context.Context, id DocID, owner UserID) error

	// Список: свои + расшаренные + публичные (в зависимости от фильтров)
	DocsList(ctx context.Context, me User, f ListFilter) ([]Document, error)

	// Обновления (для повышения версии/etag)
	Touch(ctx context.Context, id DocID) error
}

type SharesRepo interface {
	UpsertReadGrant(ctx context.Context, docID DocID, login string, canRead bool) error
	RemoveGrant(ctx context.Context, docID DocID, login string) error
	ListGrantedLogins(ctx context.Context, docID DocID) ([]string, error)
}
