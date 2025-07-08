package backend

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/photoprism/photoprism/pkg/storage/types"
)

// FSConfig holds configuration for the filesystem storage backend.
type FSConfig struct {
	// Root is the root directory for the filesystem storage.
	Root string
}

// FSStorage implements the Storage interface for local filesystem storage.
type FSStorage struct {
	config FSConfig
}

// NewFSStorage creates a new filesystem storage backend.
func NewFSStorage(cfg FSConfig) (*FSStorage, error) {
	// Ensure the root directory exists
	if err := os.MkdirAll(cfg.Root, 0755); err != nil {
		return nil, err
	}

	return &FSStorage{
		config: cfg,
	}, nil
}

// Type returns the storage type.
func (s *FSStorage) Type() string {
	return "fs"
}

// Stat returns file info for the given path.
func (s *FSStorage) Stat(ctx context.Context, path string) (types.FileInfo, error) {
	fullPath := s.fullPath(path)
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	return &fsFileInfo{
		name:    info.Name(),
		path:    path,
		size:    info.Size(),
		mode:    info.Mode(),
		modTime: info.ModTime(),
		isDir:   info.IsDir(),
	}, nil
}

// Read reads the file at the given path.
func (s *FSStorage) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := s.fullPath(path)
	return os.Open(fullPath)
}

// Write writes data to the given path.
func (s *FSStorage) Write(ctx context.Context, path string, r io.Reader) error {
	fullPath := s.fullPath(path)

	// Ensure the directory exists
	if err := s.MkdirAll(ctx, filepath.Dir(path)); err != nil {
		return err
	}

	// Create or truncate the file
	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Copy the data
	_, err = io.Copy(f, r)
	return err
}

// Delete removes the file or directory at the given path.
func (s *FSStorage) Delete(ctx context.Context, path string) error {
	fullPath := s.fullPath(path)
	return os.RemoveAll(fullPath)
}

// List lists files in the given directory path.
func (s *FSStorage) List(ctx context.Context, path string, recursive bool) ([]types.FileInfo, error) {
	fullPath := s.fullPath(path)
	var result []types.FileInfo

	// Check if the path exists and is a directory
	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrNotExist
	}

	// Define the walk function
	walkFn := func(currentPath string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory
		if currentPath == fullPath {
			return nil
		}

		// Get the relative path
		relPath, err := filepath.Rel(fullPath, currentPath)
		if err != nil {
			return err
		}

		// Convert to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)

		// For non-recursive mode, only include direct children
		if !recursive {
			if strings.Contains(relPath, "/") {
				if info.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
		}

		// Get file info
		fileInfo, err := info.Info()
		if err != nil {
			return err
		}

		result = append(result, &fsFileInfo{
			name:    filepath.Base(relPath),
			path:    filepath.ToSlash(filepath.Join(path, relPath)),
			size:    fileInfo.Size(),
			mode:    fileInfo.Mode(),
			modTime: fileInfo.ModTime(),
			isDir:   fileInfo.IsDir(),
		})

		return nil
	}

	// Walk the directory
	if recursive {
		err = filepath.WalkDir(fullPath, walkFn)
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := walkFn(filepath.Join(fullPath, entry.Name()), entry, nil); err != nil {
				return nil, err
			}
		}
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

// MkdirAll creates all necessary directories for the given path.
func (s *FSStorage) MkdirAll(ctx context.Context, path string) error {
	fullPath := s.fullPath(path)
	return os.MkdirAll(fullPath, 0755)
}

// Join joins path elements into a filesystem path.
func (s *FSStorage) Join(elem ...string) string {
	return filepath.Join(elem...)
}

// URL returns a file:// URL for the given path.
func (s *FSStorage) URL(ctx context.Context, path string) (string, error) {
	fullPath := s.fullPath(path)
	return "file://" + fullPath, nil
}

// fullPath returns the full filesystem path for the given storage path.
func (s *FSStorage) fullPath(path string) string {
	return filepath.Join(s.config.Root, filepath.FromSlash(path))
}

// fsFileInfo implements the types.FileInfo interface for local files.
type fsFileInfo struct {
	name    string
	path    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

// Name returns the base name of the file.
func (f *fsFileInfo) Name() string {
	return f.name
}

// Path returns the full path of the file.
func (f *fsFileInfo) Path() string {
	return f.path
}

// Size returns the length in bytes.
func (f *fsFileInfo) Size() int64 {
	return f.size
}

// Mode returns the file mode bits.
func (f *fsFileInfo) Mode() os.FileMode {
	return f.mode
}

// ModTime returns the modification time.
func (f *fsFileInfo) ModTime() time.Time {
	return f.modTime
}

// IsDir reports whether the file is a directory.
func (f *fsFileInfo) IsDir() bool {
	return f.isDir
}

// Ensure fsFileInfo implements types.FileInfo
var _ types.FileInfo = (*fsFileInfo)(nil)
