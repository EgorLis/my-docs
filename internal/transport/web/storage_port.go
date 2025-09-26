package web

import (
	"context"
	"io"
)

type BlobStorage interface {
	Put(ctx context.Context, r io.Reader, hintName, mime string) (storageKey string, size int64, sha []byte, err error)
	Delete(ctx context.Context, storageKey string) error
}
