// Package container provides dependency injection and lifecycle management
// for the AI Reimbursement system following Clean Architecture principles.
package container

import (
	"fmt"
	"time"
)

// Config holds all configuration for the Container.
// It aggregates configurations for all subsystems.
type Config struct {
	// Database configuration
	Database DatabaseConfig

	// Lark API configuration
	Lark LarkConfig

	// OpenAI configuration
	OpenAI OpenAIConfig

	// Storage configuration
	Storage StorageConfig

	// Server configuration
	Server ServerConfig

	// Worker configuration
	Worker WorkerConfig
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	// Path to SQLite database file
	Path string

	// MaxOpenConns is the maximum number of open connections
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int

	// ConnMaxLifetime is the maximum connection lifetime
	ConnMaxLifetime time.Duration

	// MigrationsDir is the path to migration files
	MigrationsDir string
}

// LarkConfig holds Lark API settings.
type LarkConfig struct {
	// AppID is the Lark application ID
	AppID string

	// AppSecret is the Lark application secret
	AppSecret string

	// ApprovalCode is the approval definition code
	ApprovalCode string

	// APITimeout is the timeout for API calls
	APITimeout time.Duration
}

// OpenAIConfig holds OpenAI API settings.
type OpenAIConfig struct {
	// APIKey is the OpenAI API key
	APIKey string

	// Model is the model to use (e.g., "gpt-4o")
	Model string

	// Temperature controls randomness (0.0-1.0)
	Temperature float32

	// MaxTokens limits response length
	MaxTokens int

	// Timeout for API calls
	Timeout time.Duration

	// PriceDeviationThreshold for audit
	PriceDeviationThreshold float64

	// Policies for audit validation
	Policies map[string]interface{}
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	// AttachmentDir is the base directory for attachments
	AttachmentDir string

	// VoucherOutputDir is the directory for generated vouchers
	VoucherOutputDir string

	// TemplatePath is the path to voucher templates
	TemplatePath string

	// FontPath is the path to CJK font file
	FontPath string

	// CompanyName for vouchers
	CompanyName string

	// CompanyTaxID for vouchers
	CompanyTaxID string
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	// Host to bind to
	Host string

	// Port to listen on
	Port int

	// ReadTimeout for HTTP server
	ReadTimeout time.Duration

	// WriteTimeout for HTTP server
	WriteTimeout time.Duration
}

// WorkerConfig holds background worker settings.
type WorkerConfig struct {
	// Download worker settings
	DownloadPollInterval    time.Duration
	DownloadBatchSize       int
	DownloadTimeout         time.Duration

	// Invoice worker settings
	InvoicePollInterval     time.Duration
	InvoiceBatchSize        int
	InvoiceProcessTimeout   time.Duration
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Path:            "data/reimbursement.db",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
			MigrationsDir:   "migrations",
		},
		Lark: LarkConfig{
			APITimeout: 30 * time.Second,
		},
		OpenAI: OpenAIConfig{
			Model:                   "gpt-4o",
			Temperature:             0.3,
			MaxTokens:               1000,
			Timeout:                 60 * time.Second,
			PriceDeviationThreshold: 0.3,
			Policies:                make(map[string]interface{}),
		},
		Storage: StorageConfig{
			AttachmentDir:    "attachments",
			VoucherOutputDir: "generated_vouchers",
			TemplatePath:     "templates/reimbursement_form.xlsx",
			FontPath:         "configs/NotoSansCJKsc-Regular.otf",
		},
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Worker: WorkerConfig{
			DownloadPollInterval:  5 * time.Second,
			DownloadBatchSize:     10,
			DownloadTimeout:       30 * time.Second,
			InvoicePollInterval:   10 * time.Second,
			InvoiceBatchSize:      5,
			InvoiceProcessTimeout: 120 * time.Second,
		},
	}
}

// Validate checks that required configuration values are present.
func (c *Config) Validate() error {
	// Validate Lark configuration
	if c.Lark.AppID == "" {
		return fmt.Errorf("lark.app_id is required")
	}
	if c.Lark.AppSecret == "" {
		return fmt.Errorf("lark.app_secret is required")
	}
	if c.Lark.ApprovalCode == "" {
		return fmt.Errorf("lark.approval_code is required")
	}

	// Validate OpenAI configuration
	if c.OpenAI.APIKey == "" {
		return fmt.Errorf("openai.api_key is required")
	}

	// Validate storage configuration
	if c.Storage.AttachmentDir == "" {
		return fmt.Errorf("storage.attachment_dir is required")
	}

	return nil
}
