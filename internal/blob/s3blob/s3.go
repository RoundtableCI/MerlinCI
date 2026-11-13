// Package s3blob stores artifact content in an S3-compatible bucket.
package s3blob

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config addresses an S3-compatible bucket: AWS S3, Cloudflare R2, MinIO,
// SeaweedFS and others.
type Config struct {
	// Endpoint is the service URL, such as http://localhost:8333.
	Endpoint string
	Region   string
	Bucket   string
	// AccessKeyID and SecretAccessKey may both be empty for a local service
	// that needs no credentials.
	AccessKeyID     string
	SecretAccessKey string
}

// Store is a blob.Store backed by an S3-compatible bucket.
type Store struct {
	client *minio.Client
	bucket string
}

// New connects to the bucket, creating it when it does not exist.
func New(ctx context.Context, cfg Config) (*Store, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("object storage endpoint %q is not a URL", cfg.Endpoint)
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("object storage needs a bucket name")
	}
	opts := &minio.Options{Secure: u.Scheme == "https", Region: cfg.Region}
	if cfg.AccessKeyID != "" {
		opts.Creds = credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, "")
	}
	client, err := minio.New(u.Host, opts)
	if err != nil {
		return nil, fmt.Errorf("object storage: %w", err)
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("object storage: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, fmt.Errorf("create bucket %q: %w", cfg.Bucket, err)
		}
	}
	return &Store{client: client, bucket: cfg.Bucket}, nil
}

func (s *Store) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
	return err
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(obj)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, fmt.Errorf("blob %q not found", key)
		}
		return nil, err
	}
	return data, nil
}
