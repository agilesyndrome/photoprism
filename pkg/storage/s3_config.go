package storage

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
)

// S3Config holds the configuration for S3 storage.
type S3Config struct {
	// Connection settings
	Endpoint        string `yaml:"endpoint"`
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token,omitempty"`
	Bucket          string `yaml:"bucket"`

	// Advanced settings
	UsePathStyle    bool   `yaml:"use_path_style"`
	DisableSSL      bool   `yaml:"disable_ssl"`
	UseAccelerate   bool   `yaml:"use_accelerate"`
	UseDualStack    bool   `yaml:"use_dual_stack"`
	UseTransferAccel bool   `yaml:"use_transfer_accel"`
	UseCustomCA     string `yaml:"custom_ca,omitempty"`
	UseSharedConfig bool   `yaml:"use_shared_config"`
}

// LoadS3ConfigFromEnv loads S3 configuration from environment variables.
// It follows the standard AWS environment variable naming conventions.
func LoadS3ConfigFromEnv() S3Config {
	return S3Config{
		Endpoint:        os.Getenv("S3_ENDPOINT"),
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		SessionToken:    os.Getenv("AWS_SESSION_TOKEN"),
		Region:          os.Getenv("AWS_REGION"),
		Bucket:          os.Getenv("S3_BUCKET"),
		UsePathStyle:    strings.ToLower(os.Getenv("S3_FORCE_PATH_STYLE")) == "true",
		DisableSSL:      strings.ToLower(os.Getenv("S3_DISABLE_SSL")) == "true",
		UseAccelerate:   strings.ToLower(os.Getenv("S3_ACCELERATE")) == "true",
		UseDualStack:    strings.ToLower(os.Getenv("S3_USE_DUALSTACK")) == "true",
		UseTransferAccel: strings.ToLower(os.Getenv("S3_USE_TRANSFER_ACCELERATION")) == "true",
		UseCustomCA:     os.Getenv("S3_CUSTOM_CA"),
		UseSharedConfig: strings.ToLower(os.Getenv("AWS_SDK_LOAD_CONFIG")) != "false",
	}
}

// MergeWithAwsDefaults merges the S3Config with AWS SDK default configuration.
// This allows using the AWS default credential provider chain if no explicit
// credentials are provided.
func (c *S3Config) MergeWithAwsDefaults() {
	// If no explicit credentials are provided, use the default credential chain
	if c.AccessKeyID == "" || c.SecretAccessKey == "" {
		// Load the default AWS configuration using AWS SDK v2
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx,
			// Enable loading from shared config files if configured
			config.WithSharedConfigProfile(""),
		)

		if err != nil {
			log.Printf("s3: failed to load AWS default config: %v", err)
			return
		}

		// Get credentials from the default chain
		creds, err := cfg.Credentials.Retrieve(ctx)
		if err != nil {
			log.Printf("s3: failed to retrieve AWS credentials: %v", err)
			return
		}

		// Only set credentials if they're not already set
		if c.AccessKeyID == "" && creds.AccessKeyID != "" {
			c.AccessKeyID = creds.AccessKeyID
		}
		if c.SecretAccessKey == "" && creds.SecretAccessKey != "" {
			c.SecretAccessKey = creds.SecretAccessKey
		}
		if c.SessionToken == "" && creds.SessionToken != "" {
			c.SessionToken = creds.SessionToken
		}
		if c.Region == "" && cfg.Region != "" {
			c.Region = cfg.Region
		}

		// Shared config is automatically handled by the SDK v2's LoadDefaultConfig
	}

	// If region is still not set, try to get it from the environment
	if c.Region == "" {
		c.Region = os.Getenv("AWS_REGION")
		if c.Region == "" {
			c.Region = os.Getenv("AWS_DEFAULT_REGION")
		}
	}
}

// ApplyToConfig applies the S3 configuration to a Config struct.
func (c *S3Config) ApplyToConfig(cfg *Config) {
	if cfg == nil {
		return
	}

	// Only apply S3 config if the storage type is S3
	if cfg.Type == StorageTypeS3 {
		if c != nil {
			// Create a new S3 config if it doesn't exist
			if cfg.S3 == nil {
				cfg.S3 = &S3Config{}
			}

			// Apply non-empty values from this config to the target config
			if c.Endpoint != "" {
				cfg.S3.Endpoint = c.Endpoint
			}
			if c.AccessKeyID != "" {
				cfg.S3.AccessKeyID = c.AccessKeyID
			}
			if c.SecretAccessKey != "" {
				cfg.S3.SecretAccessKey = c.SecretAccessKey
			}
			if c.SessionToken != "" {
				cfg.S3.SessionToken = c.SessionToken
			}
			if c.Region != "" {
				cfg.S3.Region = c.Region
			}
			if c.Bucket != "" {
				cfg.S3.Bucket = c.Bucket
			}

			// Apply boolean flags
			cfg.S3.UsePathStyle = c.UsePathStyle
			cfg.S3.DisableSSL = c.DisableSSL
			cfg.S3.UseAccelerate = c.UseAccelerate
			cfg.S3.UseDualStack = c.UseDualStack
			cfg.S3.UseTransferAccel = c.UseTransferAccel
			if c.UseCustomCA != "" {
				cfg.S3.UseCustomCA = c.UseCustomCA
			}
			cfg.S3.UseSharedConfig = c.UseSharedConfig
		}
	}
}
