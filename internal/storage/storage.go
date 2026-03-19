// Package storage provides artifact storage backends for run artifacts.
// Supports S3/MinIO and local filesystem.
package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Backend is the interface for artifact storage.
type Backend interface {
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	GetURL(key string) string
}

// S3Config holds S3/MinIO connection settings.
type S3Config struct {
	Endpoint  string
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
}

// S3Backend stores artifacts in S3 or MinIO.
type S3Backend struct {
	client *s3.Client
	bucket string
	url    string
}

// NewS3Backend creates an S3/MinIO storage backend.
func NewS3Backend(cfg S3Config) (*S3Backend, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = &cfg.Endpoint
			o.UsePathStyle = true
		}
	})

	return &S3Backend{
		client: client,
		bucket: cfg.Bucket,
		url:    cfg.Endpoint,
	}, nil
}

func (b *S3Backend) Upload(ctx context.Context, key string, reader io.Reader, contentType string) error {
	_, err := b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      &b.bucket,
		Key:         &key,
		Body:        reader,
		ContentType: &contentType,
	})
	return err
}

func (b *S3Backend) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &b.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (b *S3Backend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &b.bucket,
		Key:    &key,
	})
	return err
}

func (b *S3Backend) GetURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", b.url, b.bucket, key)
}

// LocalBackend stores artifacts on the local filesystem.
type LocalBackend struct {
	basePath string
}

// NewLocalBackend creates a local filesystem storage backend.
func NewLocalBackend(basePath string) (*LocalBackend, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("create base path: %w", err)
	}
	return &LocalBackend{basePath: basePath}, nil
}

func (b *LocalBackend) Upload(_ context.Context, key string, reader io.Reader, _ string) error {
	path := filepath.Join(b.basePath, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, reader)
	return err
}

func (b *LocalBackend) Download(_ context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(b.basePath, key)
	return os.Open(path)
}

func (b *LocalBackend) Delete(_ context.Context, key string) error {
	path := filepath.Join(b.basePath, key)
	return os.Remove(path)
}

func (b *LocalBackend) GetURL(key string) string {
	return fmt.Sprintf("/artifacts/%s", key)
}

// Compile-time checks.
var _ Backend = (*S3Backend)(nil)
var _ Backend = (*LocalBackend)(nil)
