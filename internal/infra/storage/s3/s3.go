package s3

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

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
func (s *Storage) Put(ctx context.Context, r io.Reader, hintName, mime string) (storageKey string, size int64, sha []byte, err error) {
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
		return "", 0, nil, err
	}

	sha = h.Sum(nil)
	finalKey := fmt.Sprintf("sha256/%x", sha)
	if finalKey != tmpKey {
		src := minio.CopySrcOptions{Bucket: s.bucket, Object: tmpKey}
		dst := minio.CopyDestOptions{Bucket: s.bucket, Object: finalKey}
		if _, err := s.cl.CopyObject(ctx, dst, src); err != nil {
			_ = s.cl.RemoveObject(ctx, s.bucket, tmpKey, minio.RemoveObjectOptions{})
			return "", 0, nil, err
		}
		_ = s.cl.RemoveObject(ctx, s.bucket, tmpKey, minio.RemoveObjectOptions{})
	}
	return finalKey, info.Size, sha, nil
}

// Get открывает поток для чтения. rangeHeader формата "bytes=0-1023" (опц.).
func (s *Storage) Get(ctx context.Context, storageKey, rangeHeader string) (rc io.ReadCloser, contentLen int64, contentRange string, err error) {
	opts := minio.GetObjectOptions{}
	if strings.HasPrefix(rangeHeader, "bytes=") {
		rg := strings.TrimPrefix(rangeHeader, "bytes=")
		parts := strings.Split(rg, "-")
		var a, b *int64
		if len(parts) == 2 {
			if parts[0] != "" {
				if v, e := strconv.ParseInt(parts[0], 10, 64); e == nil {
					a = &v
				}
			}
			if parts[1] != "" {
				if v, e := strconv.ParseInt(parts[1], 10, 64); e == nil {
					b = &v
				}
			}
		}
		if a != nil || b != nil {
			if err := opts.SetRange(getVal(a), getVal(b)); err != nil {
				return nil, 0, "", err
			}
		}
	}
	obj, err := s.cl.GetObject(ctx, s.bucket, storageKey, opts)
	if err != nil {
		return nil, 0, "", err
	}
	st, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, 0, "", err
	}
	return obj, st.Size, st.Metadata.Get("Content-Range"), nil
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
