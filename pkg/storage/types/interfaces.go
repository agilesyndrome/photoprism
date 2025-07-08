// Package types contains common types and interfaces for the storage package.
package types

import (
	"context"
	"io"
	"os"
	"time"
)

// FileInfo contains metadata about a file in storage.
type FileInfo interface {
	Name() string       // Base name of the file
	Path() string       // Full path including name
	Size() int64        // Length in bytes
	Mode() os.FileMode  // File mode bits
	ModTime() time.Time // Modification time
	IsDir() bool        // Whether it's a directory
}

// Storage defines the interface that all storage backends must implement.
type Storage interface {
	// Type returns the type of the storage backend (e.g., "fs", "s3").
	Type() string

	// Stat returns file info for the given path.
	Stat(ctx context.Context, path string) (FileInfo, error)

	// Read reads the file at the given path.
	Read(ctx context.Context, path string) (io.ReadCloser, error)

	// Write writes data to the given path.
	Write(ctx context.Context, path string, r io.Reader) error

	// Delete removes the file at the given path.
	Delete(ctx context.Context, path string) error

	// List lists files in the given directory path.
	List(ctx context.Context, path string, recursive bool) ([]FileInfo, error)

	// MkdirAll creates all necessary directories for the given path.
	MkdirAll(ctx context.Context, path string) error

	// Join joins path elements into a storage-specific path.
	Join(elem ...string) string

	// URL returns a URL to access the given path.
	URL(ctx context.Context, path string) (string, error)
}
