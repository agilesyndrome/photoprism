package storage

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		expected    Config
		expectError bool
	}{
		// File URLs
		{
			name: "File URL with absolute path",
			url:  "file:///path/to/dir",
			expected: Config{
				Type: StorageTypeFS,
				Path: "/path/to/dir",
			},
			expectError: false,
		},
		{
			name: "File URL with relative path",
			url:  "file:relative/path",
			expected: Config{
				Type: StorageTypeFS,
				Path: filepath.Join(mustGetwd(), "relative/path"),
			},
			expectError: false,
		},
		{
			name: "Local path without scheme",
			url:  "/absolute/path",
			expected: Config{
				Type: StorageTypeFS,
				Path: "/absolute/path",
			},
			expectError: false,
		},

		// S3 URLs with s3:// scheme
		{
			name: "S3 URL with bucket and key",
			url:  "s3://my-bucket/path/to/object",
			expected: Config{
				Type: StorageTypeS3,
				S3: &S3Config{
					Bucket: "my-bucket",
				},
				Path: "path/to/object",
			},
			expectError: false,
		},
		{
			name: "S3 URL with credentials",
			url:  "s3://access:secret@my-bucket/path/to/object?region=us-west-2",
			expected: Config{
				Type: StorageTypeS3,
				S3: &S3Config{
					AccessKeyID:     "access",
					SecretAccessKey: "secret",
					Bucket:         "my-bucket",
					Region:         "us-west-2",
					Endpoint:       "https://s3.us-west-2.amazonaws.com",
					UsePathStyle:   false,
				},
				Path: "path/to/object",
			},
			expectError: false,
		},

		// HTTP/HTTPS S3 URLs
		{
			name: "S3 virtual-hosted-style URL",
			url:  "https://my-bucket.s3.us-west-2.amazonaws.com/path/to/object",
			expected: Config{
				Type: StorageTypeS3,
				S3: &S3Config{
					Bucket:       "my-bucket",
					Region:       "us-west-2",
					Endpoint:     "https://s3.us-west-2.amazonaws.com",
					UsePathStyle: false,
					DisableSSL:   false,
				},
				Path: "path/to/object",
			},
			expectError: false,
		},
		{
			name: "S3 path-style URL",
			url:  "https://s3.us-west-2.amazonaws.com/my-bucket/path/to/object",
			expected: Config{
				Type: StorageTypeS3,
				S3: &S3Config{
					Bucket:       "my-bucket",
					Region:       "us-west-2",
					Endpoint:     "https://s3.us-west-2.amazonaws.com",
					UsePathStyle: true,
					DisableSSL:   false,
				},
				Path: "path/to/object",
			},
			expectError: false,
		},

		// Custom S3-compatible endpoints
		{
			name: "Custom S3-compatible endpoint",
			url:  "https://storage.example.com/my-bucket/path/to/object?region=us-east-1",
			expected: Config{
				Type: StorageTypeS3,
				S3: &S3Config{
					Bucket:       "my-bucket",
					Region:       "us-east-1",
					Endpoint:     "https://storage.example.com",
					UsePathStyle: true,
					DisableSSL:   false,
				},
				Path: "path/to/object",
			},
			expectError: false,
		},

		// Error cases
		{
			name:        "Invalid URL",
			url:         "://invalid-url",
			expectError: true,
		},
		{
			name:        "Unsupported scheme",
			url:         "ftp://example.com/path",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := ParseURL(tc.url)
			if tc.expectError {
				assert.Error(t, err)
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			// For file URLs, just check the type and path
			if config.Type == StorageTypeFS {
				assert.Equal(t, tc.expected.Type, config.Type)
				assert.Equal(t, tc.expected.Path, config.Path)
				return
			}

			// For S3 URLs, check the S3 config
			assert.NotNil(t, config.S3)
			assert.Equal(t, tc.expected.S3.Bucket, config.S3.Bucket)
			assert.Equal(t, tc.expected.S3.Region, config.S3.Region)
			assert.Equal(t, tc.expected.S3.Endpoint, config.S3.Endpoint)
			assert.Equal(t, tc.expected.S3.UsePathStyle, config.S3.UsePathStyle)
			assert.Equal(t, tc.expected.S3.DisableSSL, config.S3.DisableSSL)
			
			// Check path
			assert.Equal(t, tc.expected.Path, config.Path)
			
			// Only check access/secret key if they were in the test case
			if tc.expected.S3.AccessKeyID != "" {
				assert.Equal(t, tc.expected.S3.AccessKeyID, config.S3.AccessKeyID)
			}
			if tc.expected.S3.SecretAccessKey != "" {
				assert.Equal(t, tc.expected.S3.SecretAccessKey, config.S3.SecretAccessKey)
			}
		})
	}
}

func TestParseS3HTTPURL(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		expectedBucket   string
		expectedKey      string
		expectedRegion   string
		expectedPathStyle bool
		expectError      bool
	}{
		{
			name:             "Virtual hosted style URL",
			url:              "https://my-bucket.s3.us-west-2.amazonaws.com/path/to/object",
			expectedBucket:   "my-bucket",
			expectedKey:      "path/to/object",
			expectedRegion:   "us-west-2",
			expectedPathStyle: false,
			expectError:      false,
		},
		{
			name:             "Path style URL",
			url:              "https://s3.us-west-2.amazonaws.com/my-bucket/path/to/object",
			expectedBucket:   "my-bucket",
			expectedKey:      "path/to/object",
			expectedRegion:   "us-west-2",
			expectedPathStyle: true,
			expectError:      false,
		},
		{
			name:             "Custom S3-compatible endpoint",
			url:              "https://storage.example.com/my-bucket/path/to/object",
			expectedBucket:   "my-bucket",
			expectedKey:      "path/to/object",
			expectedRegion:   "",
			expectedPathStyle: true,
			expectError:      false,
		},
		{
			name:             "Invalid URL",
			url:              "://invalid-url",
			expectedBucket:   "",
			expectedKey:      "",
			expectedRegion:   "",
			expectedPathStyle: false,
			expectError:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.url)
			if tc.expectError {
				assert.Error(t, err)
				return
			}

			if !assert.NoError(t, err) {
				return
			}

			bucket, key, region, _, pathStyle, err := parseS3HTTPURL(u)
			if tc.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedBucket, bucket)
			assert.Equal(t, tc.expectedKey, key)
			assert.Equal(t, tc.expectedRegion, region)
			assert.Equal(t, tc.expectedPathStyle, pathStyle)
		})
	}
}

// Helper function to get the current working directory
func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
