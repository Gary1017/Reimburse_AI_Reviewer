package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// HTTPClient interface for testability
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// AttachmentHandler manages attachment extraction and download
type AttachmentHandler struct {
	logger        *zap.Logger
	attachmentDir string
	httpClient    HTTPClient
	retryAttempts int
}

// NewAttachmentHandler creates a new attachment handler
func NewAttachmentHandler(logger *zap.Logger, attachmentDir string) *AttachmentHandler {
	return &AttachmentHandler{
		logger:        logger,
		attachmentDir: attachmentDir,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		retryAttempts: 3,
	}
}

// ExtractAttachmentURLs extracts attachment URLs from Lark form data
// Implements ARCH-001: Extract attachment URLs from attachmentV2 widgets
func (h *AttachmentHandler) ExtractAttachmentURLs(formData string) ([]*entity.AttachmentReference, error) {
	if formData == "" {
		return nil, fmt.Errorf("empty form data provided")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(formData), &data); err != nil {
		h.logger.Error("Failed to parse form data for attachment extraction", zap.Error(err))
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	references := make([]*entity.AttachmentReference, 0)

	// Handle form field with JSON string containing widgets
	if formStr, ok := data["form"].(string); ok {
		var widgets []interface{}
		if err := json.Unmarshal([]byte(formStr), &widgets); err != nil {
			h.logger.Error("Failed to unmarshal form widgets array", zap.Error(err))
			return nil, fmt.Errorf("failed to unmarshal widgets from form field: %w", err)
		}
		refs := h.extractFromWidgets(widgets)
		references = append(references, refs...)
	}

	// Handle direct widgets array
	if widgets, ok := data["widgets"].([]interface{}); ok {
		refs := h.extractFromWidgets(widgets)
		references = append(references, refs...)
	}

	h.logger.Debug("Extracted attachment references",
		zap.Int("count", len(references)))

	return references, nil
}

// extractFromWidgets extracts attachment references from widgets array
func (h *AttachmentHandler) extractFromWidgets(widgets []interface{}) []*entity.AttachmentReference {
	references := make([]*entity.AttachmentReference, 0)

	for _, w := range widgets {
		widget, ok := w.(map[string]interface{})
		if !ok {
			continue
		}

		widgetType, _ := widget["type"].(string)
		if widgetType != "attachmentV2" {
			continue
		}

		// Extract URLs from value array
		if value, ok := widget["value"].([]interface{}); ok {
			ext, _ := widget["ext"].(string)

			for _, v := range value {
				if urlStr, ok := v.(string); ok && urlStr != "" {
					ref := &entity.AttachmentReference{
						URL:          urlStr,
						OriginalName: ext,
					}
					references = append(references, ref)
				}
			}
		}
	}

	return references
}

// DownloadAttachment downloads a file from Lark Drive API
// Implements ARCH-002: Download attachments from Lark Drive with auth
func (h *AttachmentHandler) DownloadAttachment(ctx context.Context, url, token string) (*entity.AttachmentFile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header if token provided
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.logger.Warn("Download request failed",
			zap.String("url", url),
			zap.Error(err))
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		h.logger.Warn("Download returned non-200 status",
			zap.Int("status", resp.StatusCode),
			zap.String("url", url))
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Read response body
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Warn("Failed to read response body", zap.Error(err))
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Extract metadata from headers
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	file := &entity.AttachmentFile{
		Content:  content,
		FileName: "attachment",
		MimeType: mimeType,
		Size:     int64(len(content)),
	}

	h.logger.Debug("Successfully downloaded attachment",
		zap.String("url", url),
		zap.Int64("size", file.Size),
		zap.String("mime_type", mimeType))

	return file, nil
}

// DownloadAttachmentWithRetry downloads with exponential backoff retry
// Implements ARCH-006: Retry failed downloads with exponential backoff
func (h *AttachmentHandler) DownloadAttachmentWithRetry(ctx context.Context, url, token string, maxAttempts int) (*entity.AttachmentFile, error) {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		file, err := h.DownloadAttachment(ctx, url, token)
		if err == nil {
			return file, nil
		}

		lastErr = err

		// Don't retry on permanent errors (4xx except 429)
		if strings.Contains(err.Error(), "status 404") || strings.Contains(err.Error(), "status 401") {
			h.logger.Info("Permanent error, not retrying",
				zap.Int("attempt", attempt),
				zap.Error(err))
			return nil, err
		}

		if attempt < maxAttempts {
			// Exponential backoff with jitter
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			h.logger.Info("Retrying download",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err))
			time.Sleep(backoff)
		}
	}

	h.logger.Error("Failed to download after retries",
		zap.Int("max_attempts", maxAttempts),
		zap.Error(lastErr))
	return nil, fmt.Errorf("download failed after %d attempts: %w", maxAttempts, lastErr)
}

// GenerateFileName generates a unique filename for an attachment.
// If withSubdir is true, it creates a path with the instance ID as a subdirectory.
// Format (withSubdir=false, item=nil): {larkInstanceID}_att{attachmentID}_{originalName}
// Format (withSubdir=false, item!=nil): invoice_{larkInstanceID}_{amount}_{currency}.{ext}
// Format (withSubdir=true with item):  {larkInstanceID}/invoice_{larkInstanceID}_{amount}_{currency}.{ext}
// Format (withSubdir=true without item):  {larkInstanceID}/{originalName}
func (h *AttachmentHandler) GenerateFileName(larkInstanceID string, attachmentID int64, originalName string, withSubdir bool, item *entity.ReimbursementItem) string {
	// Sanitize originalName to prevent path traversal issues from originalName itself
	safeOriginalName := filepath.Base(originalName)
	ext := filepath.Ext(safeOriginalName)

	// Generate invoice filename when item info is available
	if item != nil {
		invoiceFileName := fmt.Sprintf("invoice_%s_%.2f %s%s", larkInstanceID, item.Amount, item.Currency, ext)
		if withSubdir {
			return filepath.Join(larkInstanceID, invoiceFileName)
		}
		return invoiceFileName
	}

	// Fallback when item is not available
	if withSubdir {
		return filepath.Join(larkInstanceID, safeOriginalName)
	}

	// Legacy format for backward compatibility
	return fmt.Sprintf("%s_att%d_%s", larkInstanceID, attachmentID, safeOriginalName)
}

// ExtractFileMetadata extracts filename and MIME type from widget data
// Implements ARCH-001: Extract metadata for proper file handling
func (h *AttachmentHandler) ExtractFileMetadata(widgetData map[string]interface{}) map[string]string {
	metadata := make(map[string]string)

	// Extract filename from ext field
	if ext, ok := widgetData["ext"].(string); ok {
		// Remove extension to get name
		name := strings.TrimSuffix(ext, filepath.Ext(ext))
		metadata["file_name"] = name
		metadata["mime_type"] = getMimeType(filepath.Ext(ext))
	}

	return metadata
}

// getMimeType returns MIME type for common file extensions
func getMimeType(ext string) string {
	ext = strings.ToLower(ext)

	mimeTypes := map[string]string{
		".pdf":  "application/pdf",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".xls":  "application/vnd.ms-excel",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".txt":  "text/plain",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".7z":   "application/x-7z-compressed",
	}

	if mimeType, ok := mimeTypes[ext]; ok {
		return mimeType
	}

	return "application/octet-stream"
}
