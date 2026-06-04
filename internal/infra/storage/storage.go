package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ObjectStorage is the interface the rest of the app uses.
// Never reference MinIO directly outside this package.
type ObjectStorage interface {
	PresignedPutURL(ctx context.Context, key string, expires time.Duration) (string, error)
	PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

type minioStorage struct {
	client *minio.Client
	bucket string
}

// stripScheme removes http:// or https:// prefix; minio.New expects host:port only.
func stripScheme(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}

func NewMinIO(endpoint, accessKey, secretKey, bucket string, useSSL bool) (ObjectStorage, error) {
	client, err := minio.New(stripScheme(endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: failed to create minio client: %w", err)
	}
	return &minioStorage{client: client, bucket: bucket}, nil
}

func (s *minioStorage) PresignedPutURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expires)
	if err != nil {
		return "", fmt.Errorf("storage: presign put %s: %w", key, err)
	}
	return u.String(), nil
}

func (s *minioStorage) PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expires, nil)
	if err != nil {
		return "", fmt.Errorf("storage: presign get %s: %w", key, err)
	}
	return u.String(), nil
}

func (s *minioStorage) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("storage: delete %s: %w", key, err)
	}
	return nil
}
