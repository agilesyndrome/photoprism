package storage

import (
	"context"
	"os"
	"strings"
)

// LegacyConfig represents the legacy configuration for storage.
type LegacyConfig struct {
	// OriginalsPath is the path to the originals directory.
	OriginalsPath string

	// StoragePath is the path to the storage directory.
	StoragePath string

	// CachePath is the path to the cache directory.
	CachePath string

	// BackupPath is the path to the backup directory.
	BackupPath string

	// AssetsPath is the path to the assets directory.
	AssetsPath string
}

// MigrateLegacy migrates from the legacy filesystem-based storage to the new storage abstraction.
// It creates storage backends for each path in the legacy configuration.
func MigrateLegacy(ctx context.Context, cfg LegacyConfig) (map[string]Storage, error) {
	result := make(map[string]Storage)

	// Create storage for originals
	if cfg.OriginalsPath != "" {
		storageCfg, err := ParseURL(cfg.OriginalsPath)
		if err != nil {
			return nil, err
		}

		storage, err := New(ctx, storageCfg)
		if err != nil {
			return nil, err
		}

		// Ensure the storage is accessible
		if _, err := storage.Stat(ctx, "."); err != nil {
			return nil, err
		}

		result["originals"] = storage
	}

	// Create storage for storage directory (thumbnails, etc.)
	if cfg.StoragePath != "" {
		storageCfg, err := ParseURL(cfg.StoragePath)
		if err != nil {
			return nil, err
		}

		storage, err := New(ctx, storageCfg)
		if err != nil {
			return nil, err
		}

		// Ensure the storage is accessible
		if _, err := storage.Stat(ctx, "."); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		result["storage"] = storage
	}

	// Create storage for cache
	if cfg.CachePath != "" {
		cacheCfg, err := ParseURL(cfg.CachePath)
		if err != nil {
			return nil, err
		}

		// Always use local filesystem for cache
		cacheCfg.Type = StorageTypeFS

		storage, err := New(ctx, cacheCfg)
		if err != nil {
			return nil, err
		}

		// Ensure the storage is accessible
		if _, err := storage.Stat(ctx, "."); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		result["cache"] = storage
	}

	// Create storage for backups
	if cfg.BackupPath != "" {
		backupCfg, err := ParseURL(cfg.BackupPath)
		if err != nil {
			return nil, err
		}

		storage, err := New(ctx, backupCfg)
		if err != nil {
			return nil, err
		}

		// Ensure the storage is accessible
		if _, err := storage.Stat(ctx, "."); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		result["backup"] = storage
	}

	// Create storage for assets
	if cfg.AssetsPath != "" {
		assetsCfg, err := ParseURL(cfg.AssetsPath)
		if err != nil {
			return nil, err
		}

		// Always use local filesystem for assets
		assetsCfg.Type = StorageTypeFS

		storage, err := New(ctx, assetsCfg)
		if err != nil {
			return nil, err
		}

		// Ensure the storage is accessible
		if _, err := storage.Stat(ctx, "."); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		result["assets"] = storage
	}

	return result, nil
}

// LegacyPath returns the legacy path for a given storage type.
// This is used for backward compatibility with code that still expects filesystem paths.
func LegacyPath(storageType string, cfg LegacyConfig) string {
	switch storageType {
	case "originals":
		return cfg.OriginalsPath
	case "storage":
		return cfg.StoragePath
	case "cache":
		return cfg.CachePath
	case "backup":
		return cfg.BackupPath
	case "assets":
		return cfg.AssetsPath
	default:
		return ""
	}
}

// IsLocalPath checks if the given path is a local filesystem path.
func IsLocalPath(path string) bool {
	return !strings.HasPrefix(path, "s3://") &&
		!strings.HasPrefix(path, "s3n://") &&
		!strings.HasPrefix(path, "s3a://") &&
		!strings.HasPrefix(path, "http://") &&
		!strings.HasPrefix(path, "https://")
}

// EnsureDir ensures that the directory exists and is accessible.
// For S3 storage, this creates a zero-byte object with a trailing slash.
func EnsureDir(ctx context.Context, storage Storage, path string) error {
	// Check if the path exists
	_, err := storage.Stat(ctx, path)
	if err == nil {
		// Path exists
		return nil
	}

	// Path doesn't exist, create it
	return storage.MkdirAll(ctx, path)
}
