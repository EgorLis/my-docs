package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/EgorLis/my-docs/internal/domain"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
	PathStyle bool
}

type Storage struct {
	cl     *minio.Client
	bucket string
}

func New(ctx context.Context, cfg Config) (*Storage, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	}
	if cfg.PathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}
	cl, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return nil, err
	}
	return &Storage{cl: cl, bucket: cfg.Bucket}, nil
}

// Put загружает поток и возвращает итоговый ключ вида "sha256/<hex>" и размер.
func (s *Storage) Put(ctx context.Context, r io.Reader, hintName string, mime string) (domain.BlobPutResult, error) {
	h := sha256.New()
	pr, pw := io.Pipe()
	mw := io.MultiWriter(h, pw)

	// копируем в пайп и считаем sha параллельно
	go func() {
		_, copyErr := io.Copy(mw, r)
		pw.CloseWithError(copyErr)
	}()

	tmpKey := "tmp/" + sanitize(hintName)
	info, err := s.cl.PutObject(ctx, s.bucket, tmpKey, pr, -1, minio.PutObjectOptions{
		ContentType: mime,
	})
	if err != nil {
		return domain.BlobPutResult{StorageKey: "", Size: 0, SHA256: nil}, err
	}

	sha := h.Sum(nil)
	finalKey := fmt.Sprintf("sha256/%x", sha)
	if finalKey != tmpKey {
		src := minio.CopySrcOptions{Bucket: s.bucket, Object: tmpKey}
		dst := minio.CopyDestOptions{Bucket: s.bucket, Object: finalKey}
		if _, err := s.cl.CopyObject(ctx, dst, src); err != nil {
			_ = s.cl.RemoveObject(ctx, s.bucket, tmpKey, minio.RemoveObjectOptions{})
			return domain.BlobPutResult{StorageKey: "", Size: 0, SHA256: nil}, err
		}
		_ = s.cl.RemoveObject(ctx, s.bucket, tmpKey, minio.RemoveObjectOptions{})
	}
	return domain.BlobPutResult{StorageKey: finalKey, Size: info.Size, SHA256: sha}, nil
}

// Get открывает поток для чтения.
// rangeHeader в формате "bytes=START-END" (опционально).
// Возвращает поток, длину отдаваемого тела (полного или диапазона),
// Content-Range (если был запрошен диапазон), Content-Type и ETag.
func (s *Storage) Get(
	ctx context.Context,
	storageKey string,
	rangeHeader string,
) (rc io.ReadCloser, contentLen int64, contentRange, contentType, etag string, err error) {

	// 1) HEAD: базовая мета (размер всего объекта, content-type, etag)
	info, err := s.cl.StatObject(ctx, s.bucket, storageKey, minio.StatObjectOptions{})
	if err != nil {
		return nil, 0, "", "", "", err
	}
	totalSize := info.Size
	contentType = info.ContentType
	etag = info.ETag

	// 2) Парс диапазона (если есть)
	var (
		start, end int64
		useRange   bool
	)
	if strings.HasPrefix(rangeHeader, "bytes=") {
		spec := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.SplitN(spec, "-", 2)

		switch {
		// bytes=A-B
		case len(parts) == 2 && parts[0] != "" && parts[1] != "":
			if a, e1 := strconv.ParseInt(parts[0], 10, 64); e1 == nil {
				if b, e2 := strconv.ParseInt(parts[1], 10, 64); e2 == nil && a >= 0 && b >= a {
					start, end, useRange = a, b, true
				}
			}

		// bytes=A-  (от A до конца)
		case len(parts) == 2 && parts[0] != "" && parts[1] == "":
			if a, e := strconv.ParseInt(parts[0], 10, 64); e == nil && a >= 0 {
				start, end, useRange = a, totalSize-1, true
			}

		// bytes=-N  (последние N байт)
		case len(parts) == 2 && parts[0] == "" && parts[1] != "":
			if n, e := strconv.ParseInt(parts[1], 10, 64); e == nil && n > 0 {
				if n > totalSize {
					n = totalSize
				}
				start, end, useRange = totalSize-n, totalSize-1, true
			}
		}
	}

	opts := minio.GetObjectOptions{}
	if useRange {
		// NB: SetRange принимает включающие границы [start, end]
		if e := opts.SetRange(start, end); e != nil {
			return nil, 0, "", "", "", e
		}
		contentLen = (end - start + 1)
		contentRange = fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize)
	} else {
		contentLen = totalSize
	}

	// 3) Получаем поток
	obj, err := s.cl.GetObject(ctx, s.bucket, storageKey, opts)
	if err != nil {
		return nil, 0, "", "", "", err
	}
	// (не вызываем Stat на объекте — вся нужная мета уже есть из HEAD)

	return obj, contentLen, contentRange, contentType, etag, nil
}

func (s *Storage) Delete(ctx context.Context, storageKey string) error {
	return s.cl.RemoveObject(ctx, s.bucket, storageKey, minio.RemoveObjectOptions{})
}

func sanitize(name string) string {
	u := url.PathEscape(name)
	return strings.ReplaceAll(u, "%2F", "_")
}
func getVal(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}
