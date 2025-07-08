# Storage Package

This package provides an abstraction layer for different storage backends in PhotoPrism, allowing seamless integration with various storage systems like local filesystem and S3-compatible object storage.

## Features

- **Unified Interface**: Single interface for all storage operations
- **Multiple Backends**: Support for local filesystem and S3-compatible storage
- **Flexible Configuration**: Configure storage through URLs or structured config
- **Backward Compatibility**: Easy migration from legacy filesystem-based storage

## Supported Storage Backends

### Local Filesystem

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

### S3-Compatible Storage

```go
import "github.com/photoprism/photoprism/pkg/storage"

// Using URL format (with credentials in URL)
storage, err := storage.ParseURL("s3://access_key:secret_key@bucket/path?region=us-east-1")

// Or using config
storage, err := storage.New(context.Background(), storage.Config{
    Type: storage.StorageTypeS3,
    Path: "s3://bucket/path",
    S3: &storage.S3Config{
        Endpoint:     "https://s3.amazonaws.com",
        Region:       "us-east-1",
        AccessKeyID:  "your-access-key",
        SecretAccessKey: "your-secret-key",
    },
})
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
