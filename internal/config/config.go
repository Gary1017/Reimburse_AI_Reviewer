package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Lark     LarkConfig     `mapstructure:"lark"`
	OpenAI   OpenAIConfig   `mapstructure:"openai"`
	Email    EmailConfig    `mapstructure:"email"`
	Voucher  VoucherConfig  `mapstructure:"voucher"`
	Logger   LoggerConfig   `mapstructure:"logger"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Path            string        `mapstructure:"path"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	MigrationsDir   string        `mapstructure:"migrations_dir"`
}

// LarkConfig holds Lark API configuration
type LarkConfig struct {
	AppID        string        `mapstructure:"app_id"`
	AppSecret    string        `mapstructure:"app_secret"`
	ApprovalCode string        `mapstructure:"approval_code"` // Changed from verify_token and encrypt_key
	WebhookPath  string        `mapstructure:"webhook_path"`
	APITimeout   time.Duration `mapstructure:"api_timeout"`
}

// OpenAIConfig holds OpenAI API configuration
type OpenAIConfig struct {
	APIKey      string        `mapstructure:"api_key"`
	Model       string        `mapstructure:"model"`
	Temperature float32       `mapstructure:"temperature"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

// EmailConfig holds email configuration
type EmailConfig struct {
	AccountantEmail string `mapstructure:"accountant_email"`
	SenderName      string `mapstructure:"sender_name"`
}

// VoucherConfig holds voucher generation configuration
type VoucherConfig struct {
	TemplatePath   string  `mapstructure:"template_path"`
	OutputDir      string  `mapstructure:"output_dir"`
	AttachmentDir  string  `mapstructure:"attachment_dir"`
	CompanyName    string  `mapstructure:"company_name"`
	CompanyTaxID   string  `mapstructure:"company_tax_id"`
	PriceDeviation float64 `mapstructure:"price_deviation_threshold"`
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      string `mapstructure:"level"`
	OutputPath string `mapstructure:"output_path"`
	Format     string `mapstructure:"format"`
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	// Set defaults
	setDefaults()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Override with environment variables
	bindEnvVars()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)

	// Database defaults
	viper.SetDefault("database.path", "data/reimbursement.db")
	viper.SetDefault("database.max_open_conns", 25)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime", 5*time.Minute)
	viper.SetDefault("database.migrations_dir", "migrations")

	// Lark defaults
	viper.SetDefault("lark.webhook_path", "/webhook/approval")
	viper.SetDefault("lark.api_timeout", 30*time.Second)

	// OpenAI defaults
	viper.SetDefault("openai.model", "gpt-4")
	viper.SetDefault("openai.temperature", 0.3)
	viper.SetDefault("openai.max_tokens", 1000)
	viper.SetDefault("openai.timeout", 60*time.Second)

	// Voucher defaults
	viper.SetDefault("voucher.template_path", "templates/reimbursement_form.xlsx")
	viper.SetDefault("voucher.output_dir", "generated_vouchers")
	viper.SetDefault("voucher.attachment_dir", "attachments")
	viper.SetDefault("voucher.price_deviation_threshold", 0.3)

	// Logger defaults
	viper.SetDefault("logger.level", "info")
	viper.SetDefault("logger.output_path", "stdout")
	viper.SetDefault("logger.format", "json")
}

// bindEnvVars binds environment variables to configuration
func bindEnvVars() {
	// Sensitive credentials from environment
	viper.BindEnv("lark.app_id", "LARK_APP_ID")
	viper.BindEnv("lark.app_secret", "LARK_APP_SECRET")
	viper.BindEnv("lark.approval_code", "LARK_APPROVAL_CODE")
	viper.BindEnv("openai.api_key", "OPENAI_API_KEY")
	viper.BindEnv("email.accountant_email", "ACCOUNTANT_EMAIL")
	viper.BindEnv("voucher.company_name", "COMPANY_NAME")
	viper.BindEnv("voucher.company_tax_id", "COMPANY_TAX_ID")
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Lark credentials
	if c.Lark.AppID == "" {
		return fmt.Errorf("lark.app_id is required")
	}
	if c.Lark.AppSecret == "" {
		return fmt.Errorf("lark.app_secret is required")
	}
	if c.Lark.ApprovalCode == "" {
		return fmt.Errorf("lark.approval_code is required")
	}

	// Validate OpenAI credentials
	if c.OpenAI.APIKey == "" {
		return fmt.Errorf("openai.api_key is required")
	}

	// Validate email
	if c.Email.AccountantEmail == "" {
		return fmt.Errorf("email.accountant_email is required")
	}

	// Validate voucher config
	if c.Voucher.TemplatePath == "" {
		return fmt.Errorf("voucher.template_path is required")
	}
	if c.Voucher.CompanyName == "" {
		return fmt.Errorf("voucher.company_name is required")
	}

	return nil
}
