package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	log    *log.Logger
}

func New(ctx context.Context, cfg Config, logger *log.Logger) (*Storage, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	}
	if cfg.PathStyle {
		opts.BucketLookup = minio.BucketLookupPath
	}

	logger.Printf("init s3 client endpoint=%q region=%q bucket=%q ssl=%v pathStyle=%v",
		cfg.Endpoint, cfg.Region, cfg.Bucket, cfg.UseSSL, cfg.PathStyle)

	cl, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		logger.Printf("init s3 client error: %v", err)
		return nil, err
	}

	// Быстрый health-пинг: проверим список бакетов/статус бакета (не критично).
	if exists, err := cl.BucketExists(ctx, cfg.Bucket); err != nil {
		logger.Printf("bucket exists check error: %v", err)
	} else if !exists {
		logger.Printf("bucket %q does not exist (will rely on external setup)", cfg.Bucket)
	} else {
		logger.Printf("bucket %q is reachable", cfg.Bucket)
	}

	return &Storage{cl: cl, bucket: cfg.Bucket, log: logger}, nil
}

// Ping проверяет доступность S3 и существование бакета.
func (s *Storage) Ping(ctx context.Context) error {
	ok, err := s.cl.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %q does not exist", s.bucket)
	}
	return nil
}

// Put загружает поток и возвращает итоговый ключ вида "sha256/<hex>" и размер.
func (s *Storage) Put(ctx context.Context, r io.Reader, hintName string, mime string) (domain.BlobPutResult, error) {
	start := time.Now()
	s.log.Printf("put start name=%q mime=%q", hintName, mime)

	h := sha256.New()
	pr, pw := io.Pipe()
	mw := io.MultiWriter(h, pw)

	// копируем в пайп и считаем sha параллельно
	go func() {
		_, copyErr := io.Copy(mw, r)
		_ = pw.CloseWithError(copyErr)
	}()

	tmpKey := "tmp/" + sanitize(hintName)
	info, err := s.cl.PutObject(ctx, s.bucket, tmpKey, pr, -1, minio.PutObjectOptions{
		ContentType: mime,
	})
	if err != nil {
		s.log.Printf("put upload tmp_key=%q error: %v", tmpKey, err)
		return domain.BlobPutResult{}, err
	}
	sha := h.Sum(nil)
	finalKey := fmt.Sprintf("sha256/%x", sha)

	// Если ключ изменился — копируем во "финальный" и удаляем временный
	if finalKey != tmpKey {
		s.log.Printf("put uploaded tmp_key=%q size=%d, copying to final_key=%q", tmpKey, info.Size, finalKey)
		src := minio.CopySrcOptions{Bucket: s.bucket, Object: tmpKey}
		dst := minio.CopyDestOptions{Bucket: s.bucket, Object: finalKey}
		if _, err := s.cl.CopyObject(ctx, dst, src); err != nil {
			s.log.Printf("put copy tmp->final error: %v (cleanup tmp attempted)", err)
			_ = s.cl.RemoveObject(ctx, s.bucket, tmpKey, minio.RemoveObjectOptions{})
			return domain.BlobPutResult{}, err
		}
		if err := s.cl.RemoveObject(ctx, s.bucket, tmpKey, minio.RemoveObjectOptions{}); err != nil {
			s.log.Printf("put remove tmp_key=%q warn: %v", tmpKey, err)
		}
	} else {
		s.log.Printf("put tmp_key equals final_key=%q (no copy)", finalKey)
	}

	s.log.Printf("put done final_key=%q size=%d elapsed=%s", finalKey, info.Size, time.Since(start))
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

	start := time.Now()
	s.log.Printf("get start key=%q range=%q", storageKey, rangeHeader)

	// 1) HEAD
	info, err := s.cl.StatObject(ctx, s.bucket, storageKey, minio.StatObjectOptions{})
	if err != nil {
		s.log.Printf("get stat key=%q error: %v", storageKey, err)
		return nil, 0, "", "", "", err
	}
	totalSize := info.Size
	contentType = info.ContentType
	etag = info.ETag

	// 2) Разбор Range
	var (
		useRange bool
		startB   int64
		endB     int64
	)
	if strings.HasPrefix(rangeHeader, "bytes=") {
		spec := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.SplitN(spec, "-", 2)

		switch {
		// bytes=A-B
		case len(parts) == 2 && parts[0] != "" && parts[1] != "":
			if a, e1 := strconv.ParseInt(parts[0], 10, 64); e1 == nil {
				if b, e2 := strconv.ParseInt(parts[1], 10, 64); e2 == nil && a >= 0 && b >= a {
					startB, endB, useRange = a, b, true
				}
			}
		// bytes=A- (от A до конца)
		case len(parts) == 2 && parts[0] != "" && parts[1] == "":
			if a, e := strconv.ParseInt(parts[0], 10, 64); e == nil && a >= 0 {
				startB, endB, useRange = a, totalSize-1, true
			}
		// bytes=-N (последние N байт)
		case len(parts) == 2 && parts[0] == "" && parts[1] != "":
			if n, e := strconv.ParseInt(parts[1], 10, 64); e == nil && n > 0 {
				if n > totalSize {
					n = totalSize
				}
				startB, endB, useRange = totalSize-n, totalSize-1, true
			}
		}
	}

	opts := minio.GetObjectOptions{}
	if useRange {
		// NB: SetRange принимает включающие границы [start, end]
		if e := opts.SetRange(startB, endB); e != nil {
			s.log.Printf("get set range [%d-%d] error: %v", startB, endB, e)
			return nil, 0, "", "", "", e
		}
		contentLen = (endB - startB + 1)
		contentRange = fmt.Sprintf("bytes %d-%d/%d", startB, endB, totalSize)
	} else {
		contentLen = totalSize
	}

	// 3) Получаем поток
	obj, err := s.cl.GetObject(ctx, s.bucket, storageKey, opts)
	if err != nil {
		s.log.Printf("get object key=%q error: %v", storageKey, err)
		return nil, 0, "", "", "", err
	}

	if useRange {
		s.log.Printf("get done key=%q range=%q len=%d type=%q etag=%q elapsed=%s",
			storageKey, contentRange, contentLen, contentType, etag, time.Since(start))
	} else {
		s.log.Printf("get done key=%q len=%d type=%q etag=%q elapsed=%s",
			storageKey, contentLen, contentType, etag, time.Since(start))
	}

	return obj, contentLen, contentRange, contentType, etag, nil
}

func (s *Storage) Delete(ctx context.Context, storageKey string) error {
	start := time.Now()
	s.log.Printf("delete start key=%q", storageKey)
	err := s.cl.RemoveObject(ctx, s.bucket, storageKey, minio.RemoveObjectOptions{})
	if err != nil {
		s.log.Printf("delete key=%q error: %v", storageKey, err)
		return err
	}
	s.log.Printf("delete done key=%q elapsed=%s", storageKey, time.Since(start))
	return nil
}

func sanitize(name string) string {
	u := url.PathEscape(name)
	return strings.ReplaceAll(u, "%2F", "_")
}
