package backend

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/photoprism/photoprism/pkg/storage/types"
)

// S3Config holds configuration for the S3 storage backend.
type S3Config struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Bucket          string
	PathPrefix      string
	UsePathStyle    bool
	DisableSSL      bool
}

// S3Storage implements the Storage interface for S3-compatible storage.
type S3Storage struct {
	client *s3.Client
	config S3Config
}

// NewS3Storage creates a new S3 storage backend.
func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	// Create AWS config with retry options
	retryer := retry.AddWithMaxAttempts(retry.NewStandard(), 3)

	// Initialize AWS config with credentials from environment if not provided
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithRetryer(func() aws.Retryer { return retryer }),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override credentials if provided in config
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		credProvider := credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			cfg.SessionToken,
		)
		awsCfg.Credentials = aws.NewCredentialsCache(credProvider)
	}

	// Create S3 client with custom endpoint if provided
	options := func(o *s3.Options) {
		// Configure path style if needed
		o.UsePathStyle = cfg.UsePathStyle

		// Configure custom endpoint if provided
		if cfg.Endpoint != "" {
			endpointURL, err := url.Parse(cfg.Endpoint)
			if err != nil {
				// If we can't parse the endpoint, just use it as is
				o.BaseEndpoint = aws.String(cfg.Endpoint)
			} else {
				// Ensure the endpoint has the correct scheme based on DisableSSL
				if cfg.DisableSSL {
					endpointURL.Scheme = "http"
				} else if endpointURL.Scheme == "" {
					endpointURL.Scheme = "https"
				}
				o.BaseEndpoint = aws.String(endpointURL.String())
			}

			// For S3-compatible services, we need to force path style
			o.UsePathStyle = true

			// Disable SSL if requested (must be set after BaseEndpoint)
			o.EndpointOptions.DisableHTTPS = cfg.DisableSSL
		}
	}

	// Create the S3 client with our options
	client := s3.NewFromConfig(awsCfg, options)

	return &S3Storage{
		client: client,
		config: cfg,
	}, nil
}

// Type returns the storage type.
func (s *S3Storage) Type() string {
	return "s3"
}

// Stat returns file info for the given path.
func (s *S3Storage) Stat(ctx context.Context, path string) (types.FileInfo, error) {
	path = s.normalizePath(path)

	// For the root path, return a directory
	if path == "" {
		return &s3FileInfo{
			name:    "",
			path:    "",
			size:    0,
			mode:    os.ModeDir | 0755,
			modTime: time.Now(),
			isDir:   true,
		}, nil
	}

	// Check if it's a directory (ends with /)
	if strings.HasSuffix(path, "/") {
		return &s3FileInfo{
			name:    filepath.Base(strings.TrimSuffix(path, "/")),
			path:    path,
			size:    0,
			mode:    os.ModeDir | 0755,
			modTime: time.Now(),
			isDir:   true,
		}, nil
	}

	// Try to get object metadata
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	}

	result, err := s.client.HeadObject(ctx, input)
	if err != nil {
		// If object not found, check if it's a prefix (directory)
		if isNotFound(err) {
			// Check if there are any objects with this prefix
			listInput := &s3.ListObjectsV2Input{
				Bucket:    aws.String(s.config.Bucket),
				Prefix:    aws.String(path + "/"),
				MaxKeys:   aws.Int32(1),
				Delimiter: aws.String("/"),
			}

			listResult, listErr := s.client.ListObjectsV2(ctx, listInput)
			if listErr == nil && (len(listResult.Contents) > 0 || len(listResult.CommonPrefixes) > 0) {
				return &s3FileInfo{
					name:    filepath.Base(path),
					path:    path,
					size:    0,
					mode:    os.ModeDir | 0755,
					modTime: time.Now(),
					isDir:   true,
				}, nil
			}
		}
		return nil, err
	}

	return &s3FileInfo{
		name:    filepath.Base(path),
		path:    path,
		size:    *result.ContentLength,
		mode:    0644,
		modTime: *result.LastModified,
		isDir:   false,
	}, nil
}

// Read reads the file at the given path.
func (s *S3Storage) Read(ctx context.Context, path string) (io.ReadCloser, error) {
	path = s.normalizePath(path)

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", path, err)
	}

	return result.Body, nil
}

// Write writes data to the given path.
func (s *S3Storage) Write(ctx context.Context, path string, r io.Reader) error {
	path = s.normalizePath(path)

	uploader := manager.NewUploader(s.client)
	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
		Body:   r,
	})

	return err
}

// Delete removes the file at the given path.
func (s *S3Storage) Delete(ctx context.Context, path string) error {
	path = s.normalizePath(path)

	// First check if it's a directory
	info, err := s.Stat(ctx, path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// For directories, list and delete all objects with the prefix
		var continuationToken *string
		for {
			listInput := &s3.ListObjectsV2Input{
				Bucket:            aws.String(s.config.Bucket),
				Prefix:            aws.String(path + "/"),
				ContinuationToken: continuationToken,
			}

			listResult, err := s.client.ListObjectsV2(ctx, listInput)
			if err != nil {
				return fmt.Errorf("failed to list objects: %w", err)
			}

			if len(listResult.Contents) == 0 {
				break
			}

			// Prepare batch delete
			var objectIds []s3types.ObjectIdentifier
			for _, obj := range listResult.Contents {
				objectIds = append(objectIds, s3types.ObjectIdentifier{Key: obj.Key})
			}

			// Delete objects in batch
			_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(s.config.Bucket),
				Delete: &s3types.Delete{
					Objects: objectIds,
					Quiet:   aws.Bool(true),
				},
			})

			if err != nil {
				return fmt.Errorf("failed to delete objects: %w", err)
			}

			if listResult.IsTruncated == nil || !*listResult.IsTruncated {
				break
			}

			continuationToken = listResult.NextContinuationToken
		}

		// Delete the directory marker if it exists
		_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.config.Bucket),
			Key:    aws.String(path + "/"),
		})

		return err
	}

	// For regular files, just delete the object
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	})

	return err
}

// List lists files in the given directory path.
func (s *S3Storage) List(ctx context.Context, path string, recursive bool) ([]types.FileInfo, error) {
	path = s.normalizePath(path)

	// Ensure path ends with / for directories
	if path != "" && !strings.HasSuffix(path, "/") {
		path += "/"
	}

	var result []types.FileInfo
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.config.Bucket),
			Prefix:            aws.String(path),
			ContinuationToken: continuationToken,
		}

		if !recursive {
			input.Delimiter = aws.String("/")
		}

		output, err := s.client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Process common prefixes (subdirectories in non-recursive mode)
		for _, prefix := range output.CommonPrefixes {
			prefixStr := aws.ToString(prefix.Prefix)
			// Skip the current directory itself
			if prefixStr == path {
				continue
			}

			// Remove the trailing slash for the name
			name := strings.TrimSuffix(prefixStr, "/")
			if path != "" {
				name = strings.TrimPrefix(name, path)
			}

			result = append(result, &s3FileInfo{
				name:    name,
				path:    prefixStr,
				size:    0,
				mode:    os.ModeDir | 0755,
				modTime: time.Now(),
				isDir:   true,
			})
		}

		// Process objects
		for _, obj := range output.Contents {
			// Skip the directory marker itself
			if strings.HasSuffix(*obj.Key, "/") {
				continue
			}

			// Skip the current directory
			if *obj.Key == path || *obj.Key == path+"/" {
				continue
			}

			// For non-recursive, only include direct children
			if !recursive {
				relPath := strings.TrimPrefix(*obj.Key, path)
				if strings.Contains(relPath, "/") {
					continue
				}
			}

			name := *obj.Key
			if path != "" {
				name = strings.TrimPrefix(name, path)
			}

			// Safely handle nil pointers
			var size int64 = 0
			if obj.Size != nil {
				size = *obj.Size
			}

			var modTime time.Time
			if obj.LastModified != nil {
				modTime = *obj.LastModified
			} else {
				modTime = time.Now()
			}

			result = append(result, &s3FileInfo{
				name:    name,
				path:    *obj.Key,
				size:    size,
				mode:    0644,
				modTime: modTime,
				isDir:   false,
			})
		}

		if output.IsTruncated == nil || !*output.IsTruncated {
			break
		}

		continuationToken = output.NextContinuationToken
	}

	return result, nil
}

// MkdirAll creates all necessary directories for the given path.
// In S3, directories are just prefixes, so we don't need to do anything special.
func (s *S3Storage) MkdirAll(ctx context.Context, path string) error {
	// In S3, directories are just prefixes, so we don't need to create them
	// However, we can create a zero-byte object with a trailing slash to simulate a directory
	path = s.normalizePath(path)
	if path == "" {
		return nil
	}

	// Ensure the path ends with a slash
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	// Create a zero-byte object with the directory path as the key
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(path),
	})

	return err
}

// Join joins path elements into an S3 path.
func (s *S3Storage) Join(elem ...string) string {
	// Filter out empty elements
	var nonEmpty []string
	for _, e := range elem {
		if e != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}

	// Join with forward slashes
	result := path.Join(nonEmpty...)

	// Ensure consistent forward slashes
	return filepath.ToSlash(result)
}

// URL returns a URL to access the given path.
func (s *S3Storage) URL(ctx context.Context, path string) (string, error) {
	path = s.normalizePath(path)

	if s.config.UsePathStyle || s.config.Endpoint != "" {
		// For path-style URLs or custom endpoints, use the standard URL format
		var endpoint string
		if s.config.Endpoint != "" {
			endpoint = s.config.Endpoint
		} else {
			endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", s.config.Region)
		}

		return fmt.Sprintf("%s/%s/%s", endpoint, s.config.Bucket, path), nil
	}

	// For virtual-hosted-style URLs
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.config.Bucket, s.config.Region, path), nil
}

// normalizePath normalizes the given path by removing leading/trailing slashes
// and ensuring consistent forward slashes.
func (s *S3Storage) normalizePath(p string) string {
	// Remove any path prefix if specified
	if s.config.PathPrefix != "" {
		p = strings.TrimPrefix(p, s.config.PathPrefix)
	}

	// Clean the path and ensure forward slashes
	p = filepath.ToSlash(filepath.Clean(p))

	// Remove leading slash if present
	p = strings.TrimPrefix(p, "/")

	return p
}

// s3FileInfo implements the types.FileInfo interface for S3 objects.
type s3FileInfo struct {
	name    string
	path    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

// Name returns the base name of the file.
func (f *s3FileInfo) Name() string {
	return f.name
}

// Path returns the full path of the file.
func (f *s3FileInfo) Path() string {
	return f.path
}

// Size returns the length in bytes.
func (f *s3FileInfo) Size() int64 {
	return f.size
}

// Mode returns the file mode bits.
func (f *s3FileInfo) Mode() os.FileMode {
	return f.mode
}

// ModTime returns the modification time.
func (f *s3FileInfo) ModTime() time.Time {
	return f.modTime
}

// IsDir reports whether the file is a directory.
func (f *s3FileInfo) IsDir() bool {
	return f.isDir
}

// Ensure s3FileInfo implements types.FileInfo
var _ types.FileInfo = (*s3FileInfo)(nil)

// isNotFound checks if the error is a "not found" error.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check for AWS SDK v2 error types
	var nsk *s3types.NoSuchKey
	var nfb *s3types.NotFound
	var rnfe *s3types.NoSuchKey // Using NoSuchKey as a catch-all for 404 errors

	errStr := err.Error()

	// Check for common error strings
	if strings.Contains(errStr, "NoSuchKey") ||
		strings.Contains(errStr, "NoSuchBucket") ||
		strings.Contains(errStr, "NotFound") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "404") {
		return true
	}

	// Check for specific AWS error types
	if errors.As(err, &nsk) || errors.As(err, &nfb) || errors.As(err, &rnfe) {
		return true
	}

	// Check for HTTP 404 status code in the error
	var apiErr *awshttp.ResponseError
	if errors.As(err, &apiErr) {
		if apiErr.HTTPStatusCode() == 404 {
			return true
		}
	}

	// Check for AWS operation errors
	var opErr *s3types.NoSuchKey
	if errors.As(err, &opErr) {
		return true
	}
	return errors.As(err, &nsk) || 
	       errors.As(err, &nfb) || 
	       errors.As(err, &rnfe)
}
