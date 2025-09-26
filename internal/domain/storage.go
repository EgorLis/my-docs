package domain

import (
	"context"
	"io"
)

// Хранилище бинарного контента (локальный диск или S3/MinIO)
type BlobPutResult struct {
	StorageKey string
	Size       int64
	SHA256     []byte
}

type BlobStorage interface {
	// Сохранение нового файла (возвращает ключ/размер/хэш)
	Put(ctx context.Context, r io.Reader, hintName string, mime string) (BlobPutResult, error)
	// Получение контента для отдачи клиенту (stream)
	Get(
		ctx context.Context,
		storageKey string,
		rangeHeader string,
	) (rc io.ReadCloser, contentLen int64, contentRange, contentType, etag string, err error)
	// Удаление
	Delete(ctx context.Context, storageKey string) error
}
