package config

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/photoprism/photoprism/pkg/clean"
	"github.com/photoprism/photoprism/pkg/fs"
	"github.com/photoprism/photoprism/pkg/rnd"
	"github.com/photoprism/photoprism/pkg/storage"
)

// binPaths stores known executable paths.
var (
	binPaths = make(map[string]string, 8)
	binMu    = sync.RWMutex{}
	tempPath = ""
)

// FindBin resolves the absolute file path of external binaries.
func FindBin(configBin string, defaultBin ...string) (binPath string) {
	// Binary file paths to be checked.
	var search []string

	if configBin != "" {
		search = []string{configBin}
	} else {
		search = defaultBin
	}

	// Cache key for the binary file path.
	binKey := strings.Join(append(defaultBin, configBin), ",")

	// Check if file path is cached.
	binMu.RLock()
	cached, found := binPaths[binKey]
	binMu.RUnlock()

	// Found in cache?
	if found {
		return cached
	}

	// Check binary file paths.
	for _, binPath = range search {
		if binPath == "" {
			continue
		} else if path, err := exec.LookPath(binPath); err == nil {
			binPath = path
			break
		}
	}

	// Found?
	if !fs.FileExists(binPath) {
		binPath = ""
	} else {
		// Cache result if exists.
		binMu.Lock()
		binPaths[binKey] = binPath
		binMu.Unlock()
	}

	// Return result.
	return binPath
}

// CreateDirectories creates directories for storing photos, metadata and cache files.
func (c *Config) CreateDirectories() error {
	// Error if the originals and storage path are identical.
	if c.OriginalsPath() == c.StoragePath() {
		return fmt.Errorf("originals and storage folder must be different directories")
	}

	// Make sure that the configured storage path exists and initialize it with
	// ".ppstorage" and ".ppignore files" so that it is not accidentally indexed.
	if dir := c.StoragePath(); dir == "" {
		return notFoundError("storage")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	} else if _, err = fs.WriteUnixTime(filepath.Join(dir, fs.PPStorageFilename)); err != nil {
		return fmt.Errorf("%s file in %s could not be created", fs.PPStorageFilename, clean.Log(dir))
	} else if err = fs.WriteString(filepath.Join(dir, fs.PPIgnoreFilename), fs.PPIgnoreAll); err != nil {
		return fmt.Errorf("%s file in %s could not be created", fs.PPIgnoreFilename, clean.Log(dir))
	}

	// Create originals path if it does not exist yet and return an error if it could be a storage folder.
	if dir := c.OriginalsPath(); dir == "" {
		return notFoundError("originals")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	} else if fs.FileExists(filepath.Join(dir, fs.PPStorageFilename)) {
		return fmt.Errorf("found a %s file in the originals path", fs.PPStorageFilename)
	}

	// Create import path if it does not exist yet and return an error if it could be a storage folder.
	if dir := c.ImportPath(); dir == "" {
		return notFoundError("import")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	} else if fs.FileExists(filepath.Join(dir, fs.PPStorageFilename)) {
		return fmt.Errorf("found a %s file in the import path", fs.PPStorageFilename)
	}

	// Create storage path if it doesn't exist yet.
	if dir := c.UsersStoragePath(); dir == "" {
		return notFoundError("users storage")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create assets path if it doesn't exist yet.
	if dir := c.AssetsPath(); dir == "" {
		return notFoundError("assets")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create command cache storage path if it doesn't exist yet.
	if dir := c.CmdCachePath(); dir == "" {
		return notFoundError("cmd cache")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create backup base path if it doesn't exist yet.
	if dir := c.BackupBasePath(); dir == "" {
		return notFoundError("backup")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create sidecar storage path if it doesn't exist yet.
	if dir := c.SidecarPath(); filepath.IsAbs(dir) {
		if err := fs.MkdirAll(dir); err != nil {
			return createError(dir, err)
		}
	}

	// Create and initialize cache storage directory.
	if dir := c.CachePath(); dir == "" {
		return notFoundError("cache")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(c.CachePath(), err)
	} else if err = fs.WriteString(filepath.Join(dir, fs.PPIgnoreFilename), fs.PPIgnoreAll); err != nil {
		return fmt.Errorf("%s file in %s could not be created", fs.PPIgnoreFilename, clean.Log(dir))
	}

	// Create media cache storage path if it doesn't exist yet.
	if dir := c.MediaCachePath(); dir == "" {
		return notFoundError("media")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create thumbnail cache storage path if it doesn't exist yet.
	thumbPath := c.ThumbCachePath()
	if thumbPath == "" {
		return notFoundError("thumbs")
	}

	// Only create the directory if we're using the filesystem
	if u, err := url.Parse(thumbPath); err == nil && (u.Scheme == "" || u.Scheme == "file") {
		if err := fs.MkdirAll(thumbPath); err != nil {
			return createError(thumbPath, err)
		}
	}

	// Create and initialize config directory.
	if dir := c.ConfigPath(); dir == "" {
		return notFoundError("config")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	} else if err = fs.WriteString(filepath.Join(dir, fs.PPIgnoreFilename), fs.PPIgnoreAll); err != nil {
		return fmt.Errorf("%s file in %s could not be created", fs.PPIgnoreFilename, clean.Log(dir))
	}

	// Create certificates config path if it doesn't exist yet.
	if dir := c.CertificatesPath(); dir == "" {
		return notFoundError("certificates")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create temporary file path if it doesn't exist yet.
	if dir := c.TempPath(); dir == "" {
		return notFoundError("temp")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create albums backup path if it doesn't exist yet.
	if dir := c.BackupAlbumsPath(); dir == "" {
		return notFoundError("albums")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create TensorFlow model path if it doesn't exist yet.
	if dir := c.NasnetModelPath(); dir == "" {
		return notFoundError("tensorflow model")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	// Create frontend build path if it doesn't exist yet.
	if dir := c.BuildPath(); dir == "" {
		return notFoundError("build")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	if dir := filepath.Dir(c.PIDFilename()); dir == "" {
		return notFoundError("pid file")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	if dir := filepath.Dir(c.LogFilename()); dir == "" {
		return notFoundError("log file")
	} else if err := fs.MkdirAll(dir); err != nil {
		return createError(dir, err)
	}

	if c.DarktableEnabled() {
		if dir, err := c.CreateDarktableCachePath(); err != nil {
			return fmt.Errorf("could not create darktable cache path %s", clean.Log(dir))
		}

		if dir, err := c.CreateDarktableConfigPath(); err != nil {
			return fmt.Errorf("could not create darktable cache path %s", clean.Log(dir))
		}
	}

	return nil
}

// Config represents a configuration entity with shared application settings.
type Config struct {
	once      sync.Once
	options   *Options
	settings  *Settings
	settingsMutex sync.RWMutex

	sessionAge time.Duration
	sessionTimeout time.Duration
	sessionCache time.Duration

	// Caches
	fileMutex sync.Mutex
	fileCache map[string]string

	// Storage backends
	storageMutex    sync.RWMutex
	storageBackends map[string]storage.Storage // Cache for storage backends
}

// ConfigPath returns the config path.
func (c *Config) ConfigPath() string {
	if c.options.ConfigPath == "" {
		if fs.PathExists(filepath.Join(c.StoragePath(), "settings")) {
			return filepath.Join(c.StoragePath(), "settings")
		}

		return filepath.Join(c.StoragePath(), "config")
	} else if fs.FileExists(c.options.ConfigPath) {
		if c.options.OptionsYaml == "" {
			c.options.OptionsYaml = c.options.ConfigPath
		}

		c.options.ConfigPath = filepath.Dir(c.options.ConfigPath)
	}

	return fs.Abs(c.options.ConfigPath)
}

// OptionsYaml returns the config options YAML filename.
func (c *Config) OptionsYaml() string {
	configPath := c.ConfigPath()

	if c.options.OptionsYaml == "" {
		return filepath.Join(configPath, "options.yml")
	}

	return fs.Abs(c.options.OptionsYaml)
}

// DefaultsYaml returns the default options YAML filename.
func (c *Config) DefaultsYaml() string {
	return fs.Abs(c.options.DefaultsYaml)
}

// HubConfigFile returns the backend api config file name.
func (c *Config) HubConfigFile() string {
	return filepath.Join(c.ConfigPath(), "hub.yml")
}

// SettingsYaml returns the settings YAML filename.
func (c *Config) SettingsYaml() string {
	return filepath.Join(c.ConfigPath(), "settings.yml")
}

// SettingsYamlDefaults returns the default settings YAML filename.
func (c *Config) SettingsYamlDefaults(settingsYml string) string {
	if settingsYml != "" && fs.FileExists(settingsYml) {
		// Use regular settings YAML file.
	} else if defaultsYml := c.DefaultsYaml(); defaultsYml == "" {
		// Use regular settings YAML file.
	} else if dir := filepath.Dir(defaultsYml); dir == "" || dir == "." {
		// Use regular settings YAML file.
	} else if fileName := filepath.Join(dir, "settings.yml"); settingsYml == "" || fs.FileExistsNotEmpty(fileName) {
		// Use default settings YAML file.
		return fileName
	}

	return settingsYml
}

// PIDFilename returns the filename for storing the server process id (pid).
func (c *Config) PIDFilename() string {
	if c.options.PIDFilename == "" {
		return filepath.Join(c.StoragePath(), "photoprism.pid")
	}

	return fs.Abs(c.options.PIDFilename)
}

// LogFilename returns the filename for storing server logs.
func (c *Config) LogFilename() string {
	if c.options.LogFilename == "" {
		return filepath.Join(c.StoragePath(), "photoprism.log")
	}

	return fs.Abs(c.options.LogFilename)
}

// CaseInsensitive checks if the storage path is case-insensitive.
func (c *Config) CaseInsensitive() (result bool, err error) {
	storagePath := c.StoragePath()
	return fs.CaseInsensitive(storagePath)
}

// OriginalsStorage returns the storage backend for originals.
// It creates a new storage backend if one doesn't exist for the current configuration.
func (c *Config) OriginalsStorage() (storage.Storage, error) {
	// Initialize storage backends map if needed
	c.storageMutex.Lock()
	if c.storageBackends == nil {
		c.storageBackends = make(map[string]storage.Storage)
	}
	c.storageMutex.Unlock()

	// Check if we already have a storage backend for originals
	c.storageMutex.RLock()
	if backend, exists := c.storageBackends["originals"]; exists {
		c.storageMutex.RUnlock()
		return backend, nil
	}
	c.storageMutex.RUnlock()

	// Get the originals path or URL
	path := c.OriginalsPath()

	// Parse the path/URL to determine the storage type
	u, err := url.Parse(path)
	if err != nil {
		// If parsing as URL fails, treat it as a filesystem path
		u = &url.URL{
			Scheme: "file",
			Path:   path,
		}
	}

	// Create a new storage backend based on the URL scheme
	var backend storage.Storage

	switch u.Scheme {
	case "s3":
		// For S3, we need to extract bucket and path from the URL
		bucket := u.Host
		path := strings.TrimPrefix(u.Path, "/")

		// Create S3 storage backend
		backend, err = storage.NewS3Storage(
			c.options.S3Endpoint,
			c.options.S3AccessKey,
			c.options.S3SecretKey,
			bucket,
			path,
			c.options.S3Region,
			storage.S3Options{
				PathStyle:            c.options.S3PathStyle,
				DisableSSL:           c.options.S3DisableSSL,
				UseAccelerate:        c.options.S3UseAccelerate,
				UseDualStack:         c.options.S3UseDualStack,
				UseTransferAccel:     c.options.S3UseTransferAccel,
				UseCustomCA:          c.options.S3UseCustomCA,
				UseCustomCABundle:    c.options.S3UseCustomCABundle,
				UseSharedConfig:      c.options.S3UseSharedConfig,
				UseLegacyListObjects: c.options.S3UseLegacyListObjects,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 storage backend: %v", err)
		}

	case "file", "":
		// For filesystem storage
		backend = storage.NewFSStorage(u.Path)

	default:
		return nil, fmt.Errorf("unsupported storage scheme: %s", u.Scheme)
	}

	// Cache the storage backend
	c.storageMutex.Lock()
	c.storageBackends["originals"] = backend
	c.storageMutex.Unlock()

	return backend, nil
}

// OriginalsPath returns the originals path or URL.
// For backward compatibility, it returns a filesystem path by default.
func (c *Config) OriginalsPath() string {
	if c.options.OriginalsPath == "" {
		// Try to find the right directory by iterating through a list.
		c.options.OriginalsPath = fs.FindDir(fs.OriginalPaths)
	}

	// If the path is already a URL, return it as-is
	if u, err := url.Parse(c.options.OriginalsPath); err == nil && u.Scheme != "" {
		return c.options.OriginalsPath
	}

	// For backward compatibility, return an absolute filesystem path
	return fs.Abs(c.options.OriginalsPath)
}

// OriginalsDeletable checks if originals can be deleted.
// It verifies that the storage is writable and the delete feature is enabled in settings.
func (c *Config) OriginalsDeletable() bool {
	if c.ReadOnly() || !c.Settings().Features.Delete {
		return false
	}

	// Get the storage backend
	storage, err := c.OriginalsStorage()
	if err != nil {
		log.Errorf("config: failed to get storage backend: %v", err)
		return false
	}

	// Check if the storage is writable
	writable, err := storage.IsWritable()
	if err != nil {
		log.Errorf("config: failed to check if storage is writable: %v", err)
		return false
	}

	return writable
}

// ImportStorage returns the storage backend for imports.
// It creates a new storage backend if one doesn't exist for the current configuration.
func (c *Config) ImportStorage() (storage.Storage, error) {
	// Initialize storage backends map if needed
	c.storageMutex.Lock()
	if c.storageBackends == nil {
		c.storageBackends = make(map[string]storage.Storage)
	}
	c.storageMutex.Unlock()

	// Check if we already have a storage backend for imports
	c.storageMutex.RLock()
	if backend, exists := c.storageBackends["imports"]; exists {
		c.storageMutex.RUnlock()
		return backend, nil
	}
	c.storageMutex.RUnlock()

	// Get the import path or URL
	path := c.ImportPath()

	// Parse the path/URL to determine the storage type
	u, err := url.Parse(path)
	if err != nil {
		// If parsing as URL fails, treat it as a filesystem path
		u = &url.URL{
			Scheme: "file",
			Path:   path,
		}
	}

	// Create a new storage backend based on the URL scheme
	var backend storage.Storage

	switch u.Scheme {
	case "s3":
		// For S3, we need to extract bucket and path from the URL
		bucket := u.Host
		path := strings.TrimPrefix(u.Path, "/")

		// Create S3 storage backend
		backend, err = storage.NewS3Storage(
			c.options.S3Endpoint,
			c.options.S3AccessKey,
			c.options.S3SecretKey,
			bucket,
			path,
			c.options.S3Region,
			storage.S3Options{
				PathStyle:            c.options.S3PathStyle,
				DisableSSL:           c.options.S3DisableSSL,
				UseAccelerate:        c.options.S3UseAccelerate,
				UseDualStack:         c.options.S3UseDualStack,
				UseTransferAccel:     c.options.S3UseTransferAccel,
				UseCustomCA:          c.options.S3UseCustomCA,
				UseCustomCABundle:    c.options.S3UseCustomCABundle,
				UseSharedConfig:      c.options.S3UseSharedConfig,
				UseLegacyListObjects: c.options.S3UseLegacyListObjects,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 storage backend for imports: %v", err)
		}

	case "file", "":
		// For filesystem storage
		backend = storage.NewFSStorage(u.Path)

	default:
		return nil, fmt.Errorf("unsupported storage scheme for imports: %s", u.Scheme)
	}

	// Cache the storage backend
	c.storageMutex.Lock()
	c.storageBackends["imports"] = backend
	c.storageMutex.Unlock()

	return backend, nil
}

// ImportPath returns the import directory path or URL.
// For backward compatibility, it returns a filesystem path by default.
func (c *Config) ImportPath() string {
	if c.options.ImportPath == "" {
		// Try to find the right directory by iterating through a list.
		c.options.ImportPath = fs.FindDir(fs.ImportPaths)
	}

	// If the path is already a URL, return it as-is
	if u, err := url.Parse(c.options.ImportPath); err == nil && u.Scheme != "" {
		return c.options.ImportPath
	}

	// For backward compatibility, return an absolute filesystem path
	return fs.Abs(c.options.ImportPath)
}

// ImportDest returns the relative originals path to which the files should be imported by default.
// For S3 storage, this returns the path component of the S3 URL.
// For filesystem storage, it returns a cleaned user path.
func (c *Config) ImportDest() string {
	// If the import destination is an S3 URL, return the path component
	if u, err := url.Parse(c.options.ImportDest); err == nil && u.Scheme == "s3" {
		// Remove leading slash from path
		return strings.TrimPrefix(u.Path, "/")
	}

	// For filesystem paths, use the existing cleaning logic
	return clean.UserPath(c.options.ImportDest)
}

// ImportAllow returns the file extensions that users are allowed to import.
func (c *Config) ImportAllow() fs.ExtList {
	return fs.NewExtList(c.options.ImportAllow)
}

// SidecarPath returns the storage path for generated sidecar files (relative or absolute).
func (c *Config) SidecarPath() string {
	if c.options.SidecarPath == "" {
		c.options.SidecarPath = filepath.Join(c.StoragePath(), "sidecar")
	}

	return c.options.SidecarPath
}

// SidecarPathIsAbs checks if sidecar path is absolute.
func (c *Config) SidecarPathIsAbs() bool {
	return filepath.IsAbs(c.SidecarPath())
}

// SidecarWritable checks if sidecar files can be created.
func (c *Config) SidecarWritable() bool {
	return !c.ReadOnly() || c.SidecarPathIsAbs()
}

// UsersPath returns the relative base path for user assets.
func (c *Config) UsersPath() string {
	// Set default.
	if c.options.UsersPath == "" {
		return "users"
	}

	return clean.UserPath(c.options.UsersPath)
}

// UsersOriginalsPath returns the users originals base path.
func (c *Config) UsersOriginalsPath() string {
	return filepath.Join(c.OriginalsPath(), c.UsersPath())
}

// UsersStoragePath returns the users storage base path.
func (c *Config) UsersStoragePath() string {
	return filepath.Join(c.StoragePath(), "users")
}

// UserStoragePath returns the storage path for user assets.
func (c *Config) UserStoragePath(userUid string) string {
	if !rnd.IsUID(userUid, 0) {
		return ""
	}

	dir := filepath.Join(c.UsersStoragePath(), userUid)

	if err := fs.MkdirAll(dir); err != nil {
		return ""
	}

	return dir
}

// UserUploadPath returns the upload path for the specified user.
func (c *Config) UserUploadPath(userUid, token string) (string, error) {
	if !rnd.IsUID(userUid, 0) {
		return "", fmt.Errorf("invalid uid")
	}

	dir := filepath.Join(c.UserStoragePath(userUid), "upload", clean.Token(token))

	if err := fs.MkdirAll(dir); err != nil {
		return "", err
	}

	return dir, nil
}

// TempPath returns the cached temporary directory name e.g. for uploads and downloads.
func (c *Config) TempPath() string {
	// Return cached value?
	if tempPath == "" {
		tempPath = c.tempPath()
	}

	return tempPath
}

// tempPath determines the temporary directory name e.g. for uploads and downloads.
func (c *Config) tempPath() string {
	osTempDir := os.TempDir()

	// Empty default?
	if osTempDir == "" {
		switch runtime.GOOS {
		case "android":
			osTempDir = "/data/local/tmp"
		case "windows":
			osTempDir = "C:/Windows/Temp"
		default:
			osTempDir = "/tmp"
		}

		log.Infof("config: empty default temp folder path, using %s", clean.Log(osTempDir))
	}

	// Check configured temp path first.
	if c.options.TempPath != "" {
		if dir := fs.Abs(c.options.TempPath); dir == "" {
			// Ignore.
		} else if err := fs.MkdirAll(dir); err != nil {
			// Ignore.
		} else if fs.PathWritable(dir) {
			return dir
		}
	}

	// Find alternative temp path based on storage serial checksum.
	if dir := filepath.Join(osTempDir, "photoprism_"+c.SerialChecksum()); dir == "" {
		// Ignore.
	} else if err := fs.MkdirAll(dir); err != nil {
		// Ignore.
	} else if fs.PathWritable(dir) {
		return dir
	}

	// Find alternative temp path based on built-in TempDir() function.
	if dir, err := os.MkdirTemp(osTempDir, "photoprism_"); err != nil || dir == "" {
		// Ignore.
	} else if err = fs.MkdirAll(dir); err != nil {
		// Ignore.
	} else if fs.PathWritable(dir) {
		return dir
	}

	return osTempDir
}

// CachePath returns the path for cache files.
func (c *Config) CachePath() string {
	if c.options.CachePath == "" {
		return filepath.Join(c.StoragePath(), "cache")
	}

	return fs.Abs(c.options.CachePath)
}

// CmdCachePath returns a path that external CLI tools can use as cache directory.
func (c *Config) CmdCachePath() string {
	return filepath.Join(c.CachePath(), "cmd")
}

// CmdLibPath returns the dynamic loader path that external CLI tools should use.
func (c *Config) CmdLibPath() string {
	if dir := os.Getenv("LD_LIBRARY_PATH"); dir != "" {
		return dir
	}

	return "/usr/local/lib:/usr/lib"
}

// MediaCachePath returns the main media cache path.
func (c *Config) MediaCachePath() string {
	return filepath.Join(c.CachePath(), "media")
}

// MediaFileCachePath returns the cache subdirectory path for a given file hash.
func (c *Config) MediaFileCachePath(hash string) string {
	dir := c.MediaCachePath()

	switch len(hash) {
	case 0:
		return dir
	case 1:
		dir = filepath.Join(dir, hash[0:1])
	case 2:
		dir = filepath.Join(dir, hash[0:1], hash[1:2])
	default:
		dir = filepath.Join(dir, hash[0:1], hash[1:2], hash[2:3])
	}

	// Ensure the subdirectory exists, or log an error otherwise.
	if err := fs.MkdirAll(dir); err != nil {
		log.Errorf("cache: failed to create subdirectory for media file")
	}

	return dir
}

// ThumbStorage returns the storage backend for thumbnails.
// It creates a new storage backend if one doesn't exist for the current configuration.
func (c *Config) ThumbStorage() (storage.Storage, error) {
	// Initialize storage backends map if needed
	c.storageMutex.Lock()
	if c.storageBackends == nil {
		c.storageBackends = make(map[string]storage.Storage)
	}
	c.storageMutex.Unlock()

	// Check if we already have a storage backend for thumbnails
	c.storageMutex.RLock()
	if backend, exists := c.storageBackends["thumbnails"]; exists {
		c.storageMutex.RUnlock()
		return backend, nil
	}
	c.storageMutex.RUnlock()

	// Get the thumbnails path or URL
	path := c.ThumbCachePath()

	// Parse the path/URL to determine the storage type
	u, err := url.Parse(path)
	if err != nil {
		// If parsing as URL fails, treat it as a filesystem path
		u = &url.URL{
			Scheme: "file",
			Path:   path,
		}
	}

	// Create a new storage backend based on the URL scheme
	var backend storage.Storage

	switch u.Scheme {
	case "s3":
		// For S3, we need to extract bucket and path from the URL
		bucket := u.Host
		path := strings.TrimPrefix(u.Path, "/")

		// Create S3 storage backend
		backend, err = storage.NewS3Storage(
			c.options.S3Endpoint,
			c.options.S3AccessKey,
			c.options.S3SecretKey,
			bucket,
			path,
			c.options.S3Region,
			storage.S3Options{
				PathStyle:            c.options.S3PathStyle,
				DisableSSL:           c.options.S3DisableSSL,
				UseAccelerate:        c.options.S3UseAccelerate,
				UseDualStack:         c.options.S3UseDualStack,
				UseTransferAccel:     c.options.S3UseTransferAccel,
				UseCustomCA:          c.options.S3UseCustomCA,
				UseCustomCABundle:    c.options.S3UseCustomCABundle,
				UseSharedConfig:      c.options.S3UseSharedConfig,
				UseLegacyListObjects: c.options.S3UseLegacyListObjects,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 storage backend for thumbnails: %v", err)
		}

	case "file", "":
		// For filesystem storage
		backend = storage.NewFSStorage(u.Path)

	default:
		return nil, fmt.Errorf("unsupported storage scheme for thumbnails: %s", u.Scheme)
	}

	// Cache the storage backend
	c.storageMutex.Lock()
	c.storageBackends["thumbnails"] = backend
	c.storageMutex.Unlock()

	return backend, nil
}

// ThumbCachePath returns the thumbnail storage path or URL.
// For backward compatibility, it returns a filesystem path by default.
func (c *Config) ThumbCachePath() string {
	// If a custom thumbnails path is not set, use the default path under the cache directory
	if c.options.ThumbPath == "" {
		return filepath.Join(c.CachePath(), "thumbnails")
	}

	// If the path is already a URL, return it as-is
	if u, err := url.Parse(c.options.ThumbPath); err == nil && u.Scheme != "" {
		return c.options.ThumbPath
	}

	// For backward compatibility, return an absolute filesystem path
	return fs.Abs(c.options.ThumbPath)
}

// StoragePath returns the path for generated files like cache and index.
func (c *Config) StoragePath() string {
	if c.options.StoragePath == "" {
		const dirName = "storage"

		// Default directories.
		originalsDir := fs.Abs(filepath.Join(c.OriginalsPath(), fs.PPHiddenPathname, dirName))
		storageDir := fs.Abs(dirName)

		// Find existing directories.
		if fs.PathWritable(originalsDir) && !c.ReadOnly() {
			return originalsDir
		} else if fs.PathWritable(storageDir) && c.ReadOnly() {
			return storageDir
		}

		// Fallback to backup storage path.
		if fs.PathWritable(c.options.BackupPath) {
			return fs.Abs(filepath.Join(c.options.BackupPath, dirName))
		}

		// Use .photoprism in home directory?
		if usr, _ := user.Current(); usr.HomeDir != "" {
			p := fs.Abs(filepath.Join(usr.HomeDir, fs.PPHiddenPathname, dirName))

			if fs.PathWritable(p) || c.ReadOnly() {
				return p
			}
		}

		// Fallback directory in case nothing else works.
		if c.ReadOnly() {
			return fs.Abs(filepath.Join(fs.PPHiddenPathname, dirName))
		}

		// Store cache and index in "originals/.photoprism/storage".
		return originalsDir
	}

	return fs.Abs(c.options.StoragePath)
}

// AssetsPath returns the path to static assets for models and templates.
func (c *Config) AssetsPath() string {
	if c.options.AssetsPath == "" {
		// Try to find the right directory by iterating through a list.
		c.options.AssetsPath = fs.FindDir(fs.AssetPaths)
	}

	return fs.Abs(c.options.AssetsPath)
}

// CustomAssetsPath returns the path to custom assets such as icons, models and translations.
func (c *Config) CustomAssetsPath() string {
	if c.options.CustomAssetsPath != "" {
		return fs.Abs(c.options.CustomAssetsPath)
	}

	return ""
}

// CustomStaticPath returns the custom static assets' path.
func (c *Config) CustomStaticPath() string {
	if dir := c.CustomAssetsPath(); dir == "" {
		return ""
	} else if dir = filepath.Join(dir, "static"); !fs.PathExists(dir) {
		return ""
	} else {
		return dir
	}
}

// CustomStaticFile returns the path to a custom static file.
func (c *Config) CustomStaticFile(fileName string) string {
	if dir := c.CustomStaticPath(); dir == "" {
		return ""
	} else {
		return filepath.Join(dir, fileName)
	}
}

// CustomStaticUri returns the URI to a custom static resource.
func (c *Config) CustomStaticUri() string {
	if dir := c.CustomAssetsPath(); dir == "" {
		return ""
	} else {
		return c.CdnUrl(c.BaseUri(CustomStaticUri))
	}
}

// CustomStaticAssetUri returns the resource URI of the custom static file asset.
func (c *Config) CustomStaticAssetUri(res string) string {
	if dir := c.CustomAssetsPath(); dir == "" {
		return ""
	} else {
		return c.CdnUrl(c.BaseUri(CustomStaticUri)) + "/" + res
	}
}

// LocalesPath returns the translation locales path.
func (c *Config) LocalesPath() string {
	return filepath.Join(c.AssetsPath(), "locales")
}

// ExamplesPath returns the example files path.
func (c *Config) ExamplesPath() string {
	return filepath.Join(c.AssetsPath(), "examples")
}

// TestdataPath returns the test files path.
func (c *Config) TestdataPath() string {
	return filepath.Join(c.StoragePath(), "testdata")
}

// MariadbBin returns the mariadb executable file name.
func (c *Config) MariadbBin() string {
	return FindBin("", "mariadb", "mysql")
}

// MariadbDumpBin returns the mariadb-dump executable file name.
func (c *Config) MariadbDumpBin() string {
	return FindBin("", "mariadb-dump", "mysqldump")
}

// SqliteBin returns the sqlite executable file name.
func (c *Config) SqliteBin() string {
	return FindBin("", "sqlite3")
}

// OriginalsAlbumsPath returns the optional album YAML file path inside originals.
func (c *Config) OriginalsAlbumsPath() string {
	return filepath.Join(c.OriginalsPath(), "albums")
}
