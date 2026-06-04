package storage

import (
	"context"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// EnsureBucket creates the bucket if it does not exist.
// Call once at application startup before serving requests.
func EnsureBucket(endpoint, accessKey, secretKey, bucket string, useSSL bool) error {
	client, err := minio.New(stripScheme(endpoint), &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return err
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}

	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
		log.Printf("storage: bucket %q created", bucket)
	}

	return nil
}
