package voucher

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// AttachmentHandler handles downloading attachments from Lark
type AttachmentHandler struct {
	approvalAPI *lark.ApprovalAPI
	outputDir   string
	logger      *zap.Logger
}

// NewAttachmentHandler creates a new attachment handler
func NewAttachmentHandler(approvalAPI *lark.ApprovalAPI, outputDir string, logger *zap.Logger) *AttachmentHandler {
	return &AttachmentHandler{
		approvalAPI: approvalAPI,
		outputDir:   outputDir,
		logger:      logger,
	}
}

// DownloadAttachments downloads all attachments for an instance
func (ah *AttachmentHandler) DownloadAttachments(ctx context.Context, instance *models.ApprovalInstance) ([]string, error) {
	ah.logger.Info("Downloading attachments", zap.Int64("instance_id", instance.ID))

	// Parse form data to extract file tokens
	var formData map[string]interface{}
	if err := json.Unmarshal([]byte(instance.FormData), &formData); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
	}

	// Extract file keys from form data
	fileKeys := ah.extractFileKeys(formData)
	if len(fileKeys) == 0 {
		ah.logger.Info("No attachments found", zap.Int64("instance_id", instance.ID))
		return []string{}, nil
	}

	// Create instance-specific subdirectory
	instanceDir := filepath.Join(ah.outputDir, fmt.Sprintf("instance_%d", instance.ID))
	if err := os.MkdirAll(instanceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create instance directory: %w", err)
	}

	// Download each file
	downloadedPaths := []string{}
	for i, fileKey := range fileKeys {
		filePath, err := ah.downloadFile(ctx, fileKey, instanceDir, i)
		if err != nil {
			ah.logger.Error("Failed to download file",
				zap.String("file_key", fileKey),
				zap.Error(err))
			// Continue with other files
			continue
		}
		downloadedPaths = append(downloadedPaths, filePath)
	}

	ah.logger.Info("Attachments downloaded",
		zap.Int("total", len(fileKeys)),
		zap.Int("downloaded", len(downloadedPaths)))

	return downloadedPaths, nil
}

// downloadFile downloads a single file
func (ah *AttachmentHandler) downloadFile(ctx context.Context, fileKey, outputDir string, index int) (string, error) {
	// Download file data from Lark
	fileData, err := ah.approvalAPI.DownloadFile(ctx, fileKey)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}

	// Generate output file name
	fileName := fmt.Sprintf("attachment_%d_%s", index, fileKey)
	filePath := filepath.Join(outputDir, fileName)

	// Write file to disk
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	ah.logger.Debug("File downloaded",
		zap.String("file_key", fileKey),
		zap.String("path", filePath))

	return filePath, nil
}

// extractFileKeys extracts file keys from form data
func (ah *AttachmentHandler) extractFileKeys(formData map[string]interface{}) []string {
	fileKeys := []string{}

	// Recursively search for file_key fields
	ah.searchFileKeys(formData, &fileKeys)

	return fileKeys
}

// searchFileKeys recursively searches for file keys in nested data
func (ah *AttachmentHandler) searchFileKeys(data interface{}, fileKeys *[]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			if key == "file_key" || key == "file_token" {
				if fileKey, ok := value.(string); ok && fileKey != "" {
					*fileKeys = append(*fileKeys, fileKey)
				}
			} else {
				ah.searchFileKeys(value, fileKeys)
			}
		}
	case []interface{}:
		for _, item := range v {
			ah.searchFileKeys(item, fileKeys)
		}
	}
}
