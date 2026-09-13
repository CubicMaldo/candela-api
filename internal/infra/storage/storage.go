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
	// presignClient signs URLs against the PUBLIC endpoint (the host the app's
	// devices can reach, e.g. 10.0.2.2:9000 for the Android emulator). SigV4
	// signs the Host header, so the host must be baked in at signing time —
	// it cannot be rewritten afterwards without invalidating the signature.
	presignClient *minio.Client
	bucket        string
}

// stripScheme removes http:// or https:// prefix; minio.New expects host:port only.
func stripScheme(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")
	return endpoint
}

// NewMinIO creates the storage backend. `endpoint` is the internal address the
// API uses (bucket ops); `publicEndpoint` is the address embedded in presigned
// URLs handed to clients. Pass them equal when there is no split network.
func NewMinIO(endpoint, publicEndpoint, accessKey, secretKey, bucket string, useSSL bool) (ObjectStorage, error) {
	creds := credentials.NewStaticV4(accessKey, secretKey, "")
	client, err := minio.New(stripScheme(endpoint), &minio.Options{
		Creds:  creds,
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: failed to create minio client: %w", err)
	}
	if publicEndpoint == "" {
		publicEndpoint = endpoint
	}
	// Region is pinned so minio-go never issues a GetBucketLocation call —
	// the public endpoint may not be reachable from inside the API's network
	// (e.g. 10.0.2.2 only resolves from the Android emulator).
	presignClient, err := minio.New(stripScheme(publicEndpoint), &minio.Options{
		Creds:  creds,
		Secure: strings.HasPrefix(publicEndpoint, "https://"),
		Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("storage: failed to create presign client: %w", err)
	}
	return &minioStorage{client: client, presignClient: presignClient, bucket: bucket}, nil
}

func (s *minioStorage) PresignedPutURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	u, err := s.presignClient.PresignedPutObject(ctx, s.bucket, key, expires)
	if err != nil {
		return "", fmt.Errorf("storage: presign put %s: %w", key, err)
	}
	return u.String(), nil
}

func (s *minioStorage) PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	u, err := s.presignClient.PresignedGetObject(ctx, s.bucket, key, expires, nil)
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
