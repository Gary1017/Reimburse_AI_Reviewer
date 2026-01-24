package config

import (
	"time"

	"github.com/garyjia/ai-reimbursement/internal/container"
)

// ToContainerConfig converts the application Config to a container.Config.
// This provides a bridge between the file-based config loaded by viper
// and the container's configuration structure.
func (c *Config) ToContainerConfig() *container.Config {
	return &container.Config{
		Database: container.DatabaseConfig{
			Path:            c.Database.Path,
			MaxOpenConns:    c.Database.MaxOpenConns,
			MaxIdleConns:    c.Database.MaxIdleConns,
			ConnMaxLifetime: c.Database.ConnMaxLifetime,
			MigrationsDir:   c.Database.MigrationsDir,
		},
		Lark: container.LarkConfig{
			AppID:        c.Lark.AppID,
			AppSecret:    c.Lark.AppSecret,
			ApprovalCode: c.Lark.ApprovalCode,
			APITimeout:   c.Lark.APITimeout,
		},
		OpenAI: container.OpenAIConfig{
			APIKey:                  c.OpenAI.APIKey,
			Model:                   c.OpenAI.Model,
			Temperature:             c.OpenAI.Temperature,
			MaxTokens:               c.OpenAI.MaxTokens,
			Timeout:                 c.OpenAI.Timeout,
			PriceDeviationThreshold: c.Voucher.PriceDeviation,
			Policies:                make(map[string]interface{}),
		},
		Storage: container.StorageConfig{
			AttachmentDir:    c.Voucher.AttachmentDir,
			VoucherOutputDir: c.Voucher.OutputDir,
			TemplatePath:     c.Voucher.TemplatePath,
			FontPath:         c.Voucher.FontPath,
			CompanyName:      c.Voucher.CompanyName,
			CompanyTaxID:     c.Voucher.CompanyTaxID,
		},
		Server: container.ServerConfig{
			Host:         c.Server.Host,
			Port:         c.Server.Port,
			ReadTimeout:  c.Server.ReadTimeout,
			WriteTimeout: c.Server.WriteTimeout,
		},
		Worker: container.WorkerConfig{
			DownloadPollInterval:  5 * time.Second,
			DownloadBatchSize:     10,
			DownloadTimeout:       30 * time.Second,
			InvoicePollInterval:   10 * time.Second,
			InvoiceBatchSize:      5,
			InvoiceProcessTimeout: 120 * time.Second,
		},
	}
}
