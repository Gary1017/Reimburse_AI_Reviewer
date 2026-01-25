package lark

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Downloader implements port.LarkAttachmentDownloader interface
type Downloader struct {
	handler *AttachmentHandler
	logger  *zap.Logger
}

// NewDownloader creates a new Lark attachment downloader adapter
func NewDownloader(attachmentDir string, logger *zap.Logger) *Downloader {
	return &Downloader{
		handler: NewAttachmentHandler(logger, attachmentDir),
		logger:  logger,
	}
}

// Download downloads a file from the given URL
// Implements port.LarkAttachmentDownloader interface
func (d *Downloader) Download(ctx context.Context, url string) ([]byte, int64, error) {
	// Extract token from context or use empty string
	// In the original implementation, token is obtained from the Lark client
	// For now, we'll pass empty token and let the handler deal with it
	token := ""

	file, err := d.handler.DownloadAttachment(ctx, url, token)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to download attachment: %w", err)
	}

	if file == nil {
		return nil, 0, fmt.Errorf("download returned nil file")
	}

	return file.Content, file.Size, nil
}

// DownloadWithRetry downloads a file with retry logic
// Implements port.LarkAttachmentDownloader interface
func (d *Downloader) DownloadWithRetry(ctx context.Context, url string, maxAttempts int) ([]byte, int64, error) {
	if maxAttempts <= 0 {
		maxAttempts = 3 // Default to 3 attempts
	}

	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		content, size, err := d.Download(ctx, url)
		if err == nil {
			return content, size, nil
		}

		lastErr = err

		// Don't retry on permanent errors (4xx except 429)
		if isPermanentError(err) {
			d.logger.Info("Permanent error, not retrying",
				zap.Int("attempt", attempt),
				zap.Error(err))
			return nil, 0, err
		}

		if attempt < maxAttempts {
			// Exponential backoff with jitter
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			d.logger.Info("Retrying download",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err))

			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(backoff):
				// Continue to next attempt
			}
		}
	}

	d.logger.Error("Failed to download after retries",
		zap.Int("max_attempts", maxAttempts),
		zap.Error(lastErr))
	return nil, 0, fmt.Errorf("download failed after %d attempts: %w", maxAttempts, lastErr)
}

// isPermanentError checks if an error is permanent (should not retry)
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Don't retry on 404, 401, 403 errors
	return strings.Contains(errStr, "status 404") ||
		strings.Contains(errStr, "status 401") ||
		strings.Contains(errStr, "status 403")
}
