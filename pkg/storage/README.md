# Storage Package

This package provides an abstraction layer for different storage backends in PhotoPrism, allowing seamless integration with various storage systems like local filesystem and S3-compatible object storage.

## Features

- **Unified Interface**: Single interface for all storage operations
- **Multiple Backends**: Support for local filesystem and S3-compatible storage
- **Flexible Configuration**: Configure storage through URLs or structured config
- **Backward Compatibility**: Easy migration from legacy filesystem-based storage
- **S3-Compatible**: Full support for AWS S3 and compatible services (MinIO, Ceph, etc.)
- **URL-Based Configuration**: Simple configuration using standard URL format
- **Environment Variable Support**: Easy configuration through environment variables

## Supported Storage Backends

## Supported Storage Backends

### 1. Local Filesystem

Store files on the local filesystem. This is the default storage backend.

#### Configuration Options

- **Type**: `fs` (StorageTypeFS)
- **Path**: Absolute path to the storage directory

#### Example

```go
import "github.com/photoprism/photoprism/pkg/storage"

// Using URL format
storage, err := storage.ParseURL("file:///path/to/storage")

// Or using config
storage, err := storage.New(context.Background(), storage.Config{
    Type: storage.StorageTypeFS,
    Path: "/path/to/storage",
})
```

### 2. S3-Compatible Storage

Store files in an S3-compatible object storage service (AWS S3, MinIO, Ceph, etc.).

#### Configuration Options

- **Type**: `s3` (StorageTypeS3)
- **Path**: S3 path in format `s3://bucket/prefix`
- **S3 Configuration**:
  - `Endpoint`: S3 service endpoint URL (default: AWS endpoint based on region)
  - `Region`: AWS region (e.g., `us-east-1`)
  - `AccessKeyID`: AWS access key ID
  - `SecretAccessKey`: AWS secret access key
  - `SessionToken`: Optional session token for temporary credentials
  - `UsePathStyle`: Use path-style addressing (default: false for virtual-hosted style)
  - `DisableSSL`: Disable SSL (not recommended for production)
  - `UseAccelerate`: Use S3 Transfer Acceleration
  - `UseDualStack`: Use dual-stack endpoint for IPv6
  - `UseTransferAccel`: Use S3 Transfer Acceleration (deprecated, use UseAccelerate)

#### URL Format

```
s3://[access_key:secret_key[:session_token]@]bucket/prefix[?param=value&...]
```

Supported URL parameters:
- `region`: AWS region (e.g., `us-east-1`)
- `endpoint`: Custom endpoint URL
- `path_style`: Set to `true` to use path-style addressing
- `disable_ssl`: Set to `true` to disable SSL
- `use_accelerate`: Set to `true` to enable S3 Transfer Acceleration
- `use_dualstack`: Set to `true` to use dual-stack endpoint

#### Examples

```go
import "github.com/photoprism/photoprism/pkg/storage"

// Basic configuration with credentials in URL
storage, err := storage.ParseURL("s3://access_key:secret_key@my-bucket/photos?region=us-east-1")

// Using config with additional options
storage, err := storage.New(context.Background(), storage.Config{
    Type: storage.StorageTypeS3,
    Path: "s3://my-bucket/photos",
    S3: &storage.S3Config{
        Endpoint:        "https://s3.us-east-1.amazonaws.com",
        Region:         "us-east-1",
        AccessKeyID:    "your-access-key",
        SecretAccessKey: "your-secret-key",
        UsePathStyle:   false,
        UseAccelerate:  true,
    },
})

// Using environment variables
// PHOTOPRISM_STORAGE_TYPE=s3
// PHOTOPRISM_STORAGE_PATH=s3://my-bucket/photos
// PHOTOPRISM_STORAGE_S3_REGION=us-east-1
// PHOTOPRISM_STORAGE_S3_ACCESS_KEY_ID=your-access-key
// PHOTOPRISM_STORAGE_S3_SECRET_ACCESS_KEY=your-secret-key
storage, err := storage.NewFromEnv(context.Background())
```

#### Supported S3-Compatible Services

The S3 storage backend is compatible with:
- AWS S3
- MinIO
- Ceph Object Gateway
- DigitalOcean Spaces
- Google Cloud Storage (interoperability mode)
- Alibaba Cloud OSS
- And other S3-compatible services

## Advanced Configuration

### Environment Variables

All storage configuration can be set via environment variables:

```bash
# Storage type (fs or s3)
PHOTOPRISM_STORAGE_TYPE=fs

# For filesystem storage
PHOTOPRISM_STORAGE_PATH=/path/to/storage

# For S3 storage
PHOTOPRISM_STORAGE_PATH=s3://bucket/prefix
PHOTOPRISM_STORAGE_S3_REGION=us-east-1
PHOTOPRISM_STORAGE_S3_ACCESS_KEY_ID=your-access-key
PHOTOPRISM_STORAGE_S3_SECRET_ACCESS_KEY=your-secret-key
PHOTOPRISM_STORAGE_S3_ENDPOINT=https://s3.us-east-1.amazonaws.com
PHOTOPRISM_STORAGE_S3_USE_PATH_STYLE=false
PHOTOPRISM_STORAGE_S3_USE_ACCELERATE=false
PHOTOPRISM_STORAGE_S3_USE_DUALSTACK=false
PHOTOPRISM_STORAGE_S3_DISABLE_SSL=false
```

### HTTP/HTTPS S3 URLs

The storage package can also parse HTTP/HTTPS URLs that point to S3 objects:

```go
// Virtual-hosted style URL
storage, err := storage.ParseURL("https://my-bucket.s3.us-east-1.amazonaws.com/path/to/object.jpg")

// Path-style URL
storage, err := storage.ParseURL("https://s3.us-east-1.amazonaws.com/my-bucket/path/to/object.jpg")

// Custom endpoint
storage, err := storage.ParseURL("https://custom-endpoint.com/my-bucket/path/to/object.jpg")
```

## Usage Example

```go
package main

import (
    "context"
    "fmt"
    "io"
    "os"

    "github.com/photoprism/photoprism/pkg/storage"
)

func main() {
    // Initialize storage (local filesystem in this example)
    s, err := storage.ParseURL("file:///tmp/photoprism/originals")
    if err != nil {
        panic(err)
    }

    ctx := context.Background()

    // Create a test file
    testContent := "Hello, storage!"
    err = s.Write(ctx, "test.txt", strings.NewReader(testContent))
    if err != nil {
        panic(err)
    }

    // Read the file back
    r, err := s.Read(ctx, "test.txt")
    if err != nil {
        panic(err)
    }
    defer r.Close()

    content, err := io.ReadAll(r)
    if err != nil {
        panic(err)
    }

    fmt.Printf("File content: %s\n", string(content))

    // List files in the root directory
    files, err := s.List(ctx, "", false)
    if err != nil {
        panic(err)
    }

    fmt.Println("Files in root directory:")
    for _, file := range files {
        fmt.Printf("- %s (%d bytes)\n", file.Name(), file.Size())
    }

    // Clean up
    if err := s.Delete(ctx, "test.txt"); err != nil {
        panic(err)
    }
}
```

## Migration from Legacy Code

To migrate from the legacy filesystem-based approach to the new storage abstraction:

```go
import "github.com/photoprism/photoprism/pkg/storage"

// Legacy configuration
legacyCfg := storage.LegacyConfig{
    OriginalsPath: "/path/to/originals",
    StoragePath:   "/path/to/storage",
    CachePath:     "/path/to/cache",
    BackupPath:    "/path/to/backup",
    AssetsPath:    "/path/to/assets",
}

// Migrate to new storage abstraction
storages, err := storage.MigrateLegacy(context.Background(), legacyCfg)
if err != nil {
    panic(err)
}

// Access different storage types
originalsStorage := storages["originals"]
storageStorage := storages["storage"]
cacheStorage := storages["cache"]
backupStorage := storages["backup"]
assetsStorage := storages["assets"]
```

## Configuration

### Environment Variables

For S3 storage, the following environment variables are supported (in addition to standard AWS environment variables):

- `PHOTOPRISM_STORAGE_TYPE`: Storage type (`fs` or `s3`)
- `PHOTOPRISM_STORAGE_PATH`: Storage path or URL
- `PHOTOPRISM_S3_ENDPOINT`: S3 endpoint URL
- `PHOTOPRISM_S3_REGION`: S3 region
- `PHOTOPRISM_S3_ACCESS_KEY`: S3 access key
- `PHOTOPRISM_S3_SECRET_KEY`: S3 secret key
- `PHOTOPRISM_S3_USE_PATH_STYLE`: Use path-style URLs (true/false)
- `PHOTOPRISM_S3_DISABLE_SSL`: Disable SSL (true/false)

### URL Format

#### Filesystem

```
file:///absolute/path/to/storage
/path/to/storage  # Implicit filesystem path
```

#### S3

```
s3://access_key:secret_key@bucket/path?region=us-east-1&endpoint=https://s3.amazonaws.com&path_style=true&disable_ssl=false
```

## License

This package is part of the PhotoPrism project and is licensed under the [AGPL-3.0 License](https://github.com/photoprism/photoprism/blob/develop/LICENSE).
