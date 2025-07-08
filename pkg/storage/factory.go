package storage

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/photoprism/photoprism/pkg/storage/backend"
	"github.com/photoprism/photoprism/pkg/storage/types"
)

// StorageType represents the type of storage backend.
type StorageType string

const (
	// StorageTypeFS represents a local filesystem storage backend.
	StorageTypeFS StorageType = "fs"
	// StorageTypeS3 represents an S3-compatible storage backend.
	StorageTypeS3 StorageType = "s3"
)

// Config holds the configuration for creating a storage backend.
type Config struct {
	// Type specifies the storage type (fs, s3). If empty, it will be detected from the path.
	Type StorageType

	// Path is the storage path or URL (e.g., "/path/to/storage" or "s3://bucket/path").
	Path string

	// S3 specific configuration (only used when Type is "s3" or Path is an S3 URL).
	S3 *S3Config
}



// New creates a new storage backend based on the provided configuration.
func New(ctx context.Context, cfg Config) (types.Storage, error) {
	// If type is not specified, try to detect it from the path
	if cfg.Type == "" {
		if isS3Path(cfg.Path) {
			cfg.Type = StorageTypeS3
		} else {
			cfg.Type = StorageTypeFS
		}
	}

	switch cfg.Type {
	case StorageTypeFS:
		return newFSStorage(cfg)
	case StorageTypeS3:
		return newS3Storage(ctx, cfg)
	default:
		return nil, errors.New("unsupported storage type: " + string(cfg.Type))
	}
}

// newFSStorage creates a new filesystem storage backend.
func newFSStorage(cfg Config) (types.Storage, error) {
	path := cfg.Path
	if path == "" {
		path = "."
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return backend.NewFSStorage(backend.FSConfig{
		Root: absPath,
	})
}

// newS3Storage creates a new S3 storage backend.
func newS3Storage(ctx context.Context, cfg Config) (types.Storage, error) {
	var bucket, pathPrefix string
	var err error

	// Parse S3 URL if path is an S3 URL
	if isS3Path(cfg.Path) {
		bucket, pathPrefix, err = parseS3URL(cfg.Path)
		if err != nil {
			return nil, err
		}
	}

	// Ensure bucket is set
	if bucket == "" {
		return nil, errors.New("bucket name is required for S3 storage")
	}

	s3Cfg := backend.S3Config{
		Bucket:          bucket,
		PathPrefix:      pathPrefix,
		Endpoint:        cfg.S3.Endpoint,
		Region:          cfg.S3.Region,
		AccessKeyID:     cfg.S3.AccessKeyID,
		SecretAccessKey: cfg.S3.SecretAccessKey,
		SessionToken:    cfg.S3.SessionToken,
		UsePathStyle:    cfg.S3.UsePathStyle,
		DisableSSL:      cfg.S3.DisableSSL,
	}

	return backend.NewS3Storage(s3Cfg)
}

// isS3Path checks if the given path is an S3 URL.
func isS3Path(path string) bool {
	return strings.HasPrefix(path, "s3://") || strings.HasPrefix(path, "s3n://") || strings.HasPrefix(path, "s3a://")
}

// parseS3URL parses an S3 URL and returns the bucket and path prefix.
func parseS3URL(s3URL string) (bucket, path string, err error) {
	if !isS3Path(s3URL) {
		return "", "", errors.New("not an S3 URL")
	}

	// Parse the URL
	u, err := url.Parse(s3URL)
	if err != nil {
		return "", "", err
	}

	// The bucket is the host part
	bucket = u.Host

	// The path is the path part, with leading slash removed
	path = strings.TrimPrefix(u.Path, "/")

	return bucket, path, nil
}

// DetectStorageType detects the storage type from the given path.
func DetectStorageType(path string) StorageType {
	if isS3Path(path) {
		return StorageTypeS3
	}
	return StorageTypeFS
}
