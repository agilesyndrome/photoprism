package storage

import (
	"errors"
	"net/url"
	"path/filepath"
	"strings"
)

// ErrNotSupported is returned when a URL scheme is not supported.
var ErrNotSupported = errors.New("scheme not supported")

// ParseURL parses a storage URL and returns a Config.
// Supported URL formats:
// - file:///path/to/dir
// - s3://bucket/key
// - s3://access_key:secret_key@bucket/key?region=us-east-1&endpoint=...
// - https://s3.region.amazonaws.com/bucket/key (path-style)
// - https://bucket.s3.region.amazonaws.com/key (virtual-hosted-style)
// - https://s3-compatible-endpoint.com/bucket/key
// - /path/to/dir (treated as file:///path/to/dir)
func ParseURL(storageURL string) (Config, error) {
	// Special case: handle "file:relative/path" as a relative path, not a file URL
	if strings.HasPrefix(storageURL, "file:") && !strings.HasPrefix(storageURL, "file://") {
		// Treat as a relative path after the "file:" prefix
		relPath := strings.TrimPrefix(storageURL, "file:")
		abspath, err := filepath.Abs(relPath)
		if err != nil {
			return Config{}, err
		}
		return Config{
			Type: StorageTypeFS,
			Path: abspath,
		}, nil
	}

	// If it's not a URL with a scheme, treat it as a filesystem path
	if !strings.Contains(storageURL, "://") {
		if filepath.IsAbs(storageURL) {
			// Handle absolute paths
			return Config{
				Type: StorageTypeFS,
				Path: storageURL,
			}, nil
		} else {
			// Handle relative paths
			abspath, err := filepath.Abs(storageURL)
			if err != nil {
				return Config{}, err
			}
			return Config{
				Type: StorageTypeFS,
				Path: abspath,
			}, nil
		}
	}

	u, err := url.Parse(storageURL)
	if err != nil {
		return Config{}, err
	}

	switch u.Scheme {
	case "file":
		path := u.Path
		if path == "" {
			path = u.Opaque
		}
		// Convert to absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return Config{}, err
		}

		return Config{
			Type: StorageTypeFS,
			Path: absPath,
		}, nil

	case "http", "https":
		// Handle S3-compatible HTTP/HTTPS URLs
		bucket, key, region, endpoint, forcePathStyle, err := parseS3HTTPURL(u)
		if err != nil {
			return Config{}, err
		}

		// Extract credentials from URL if present
		accessKeyID := ""
		secretAccessKey := ""
		if u.User != nil {
			accessKeyID = u.User.Username()
			if password, ok := u.User.Password(); ok {
				secretAccessKey = password
			}
		}

		// Get additional parameters from query
		query := u.Query()

		// Use region from URL if not already set
		if region == "" {
			region = query.Get("region")
		}

		// Use endpoint from URL if not already set
		if endpoint == "" {
			endpoint = query.Get("endpoint")
		}

		// If no explicit endpoint but we have a region, use the default S3 endpoint
		if endpoint == "" && region != "" {
			endpoint = u.Scheme + "://s3." + region + ".amazonaws.com"
		}

		return Config{
			Type: StorageTypeS3,
			S3: &S3Config{
				Endpoint:          endpoint,
				AccessKeyID:       accessKeyID,
				SecretAccessKey:   secretAccessKey,
				Region:            region,
				Bucket:            bucket,
				UsePathStyle:      forcePathStyle || query.Get("path_style") == "true",
				DisableSSL:        u.Scheme == "http" || query.Get("disable_ssl") == "true",
				UseAccelerate:     query.Get("use_accelerate") == "true",
				UseDualStack:      query.Get("use_dualstack") == "true",
				UseTransferAccel:  query.Get("use_transfer_accel") == "true",
			},
			Path: key,
		}, nil

	case "s3", "s3n", "s3a":
		// Extract bucket and key from URL path
		path := strings.TrimPrefix(u.Path, "/")
		bucket := u.Host
		key := ""

		// Handle case where bucket is in the path (s3:///bucket/key)
		if bucket == "" {
			parts := strings.SplitN(path, "/", 2)
			if len(parts) > 0 {
				bucket = parts[0]
				if len(parts) > 1 {
					key = parts[1]
				}
			}
		} else {
			key = path
		}

		// Get region and endpoint from query parameters
		query := u.Query()
		region := query.Get("region")
		endpoint := query.Get("endpoint")

		// If no endpoint is provided but we have a region, construct the default endpoint
		if endpoint == "" && region != "" {
			endpoint = "https://s3." + region + ".amazonaws.com"
		}

		// Extract credentials from URL if present
		accessKeyID := ""
		secretAccessKey := ""
		if u.User != nil {
			accessKeyID = u.User.Username()
			secretAccessKey, _ = u.User.Password()
		}

		return Config{
			Type: StorageTypeS3,
			S3: &S3Config{
				Endpoint:          endpoint,
				AccessKeyID:       accessKeyID,
				SecretAccessKey:   secretAccessKey,
				Region:            region,
				Bucket:            bucket,
				UsePathStyle:      query.Get("path_style") == "true",
				DisableSSL:        query.Get("disable_ssl") == "true",
				UseAccelerate:     query.Get("use_accelerate") == "true",
				UseDualStack:      query.Get("use_dualstack") == "true",
				UseTransferAccel:  query.Get("use_transfer_accel") == "true",
			},
			Path: key,
		}, nil

	default:
		return Config{}, &url.Error{Op: "parse", URL: storageURL, Err: ErrNotSupported}
	}
}

// parseS3HTTPURL parses an HTTP/HTTPS URL that points to an S3-compatible service.
// It supports both virtual-hosted-style and path-style URLs.
func parseS3HTTPURL(u *url.URL) (bucket, key, region, endpoint string, forcePathStyle bool, err error) {
	host := u.Hostname()
	path := strings.TrimPrefix(u.Path, "/")
	
	// Check for virtual-hosted-style URLs (bucket.s3.region.amazonaws.com)
	if strings.HasSuffix(host, ".amazonaws.com") || strings.HasSuffix(host, ".amazonaws.com.cn") {
		parts := strings.Split(host, ".")
		if len(parts) >= 4 && (parts[1] == "s3" || strings.HasPrefix(parts[1], "s3-")) {
			// Format: bucket.s3.region.amazonaws.com
			bucket = parts[0]
			region = parts[2]
			key = path
			endpoint = u.Scheme + "://" + strings.Join(parts[1:], ".")
			return bucket, key, region, endpoint, false, nil
		}
	}

	// Check for path-style URLs (s3.region.amazonaws.com/bucket/key)
	if strings.HasPrefix(host, "s3.") || strings.HasPrefix(host, "s3-") {
		// Extract region from host (s3.region or s3-region)
		region = strings.TrimPrefix(host, "s3")
		region = strings.TrimPrefix(region, ".")
		region = strings.TrimPrefix(region, "-")
		
		// Remove .amazonaws.com from region if present
		region = strings.TrimSuffix(region, ".amazonaws.com")
		region = strings.TrimSuffix(region, ".amazonaws.com.cn")
		
		// Extract bucket and key from path
		parts := strings.SplitN(path, "/", 2)
		if len(parts) > 0 {
			bucket = parts[0]
			if len(parts) > 1 {
				key = parts[1]
			}
			endpoint = u.Scheme + "://" + host
			return bucket, key, region, endpoint, true, nil
		}
	}

	// Check for custom S3-compatible endpoints
	if strings.Contains(host, "s3") || strings.Contains(host, "storage") {
		// Assume path-style by default for custom endpoints
		parts := strings.SplitN(path, "/", 2)
		if len(parts) > 0 {
			bucket = parts[0]
			if len(parts) > 1 {
				key = parts[1]
			}
			endpoint = u.Scheme + "://" + host
			return bucket, key, "", endpoint, true, nil
		}
	}

	return "", "", "", "", false, ErrNotSupported
}
