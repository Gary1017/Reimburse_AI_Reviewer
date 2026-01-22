package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// FolderManager manages instance-specific folders
// ARCH-014-A: Folder structure implementation for organizing attachments
type FolderManager struct {
	baseDir string
	logger  *zap.Logger
}

// NewFolderManager creates a new FolderManager
func NewFolderManager(baseDir string, logger *zap.Logger) *FolderManager {
	return &FolderManager{
		baseDir: baseDir,
		logger:  logger,
	}
}

// CreateInstanceFolder creates attachments/{larkInstanceID}/ folder
// Returns the full path to the created folder or error
func (m *FolderManager) CreateInstanceFolder(larkInstanceID string) (string, error) {
	if larkInstanceID == "" {
		return "", fmt.Errorf("cannot create folder: empty lark instance ID")
	}

	// Sanitize the folder name to prevent path traversal
	safeName := m.SanitizeFolderName(larkInstanceID)
	folderPath := filepath.Join(m.baseDir, safeName)

	// Create the directory (including any parent directories)
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		m.logger.Error("Failed to create instance folder",
			zap.String("lark_instance_id", larkInstanceID),
			zap.String("folder_path", folderPath),
			zap.Error(err))
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	m.logger.Debug("Created instance folder",
		zap.String("lark_instance_id", larkInstanceID),
		zap.String("folder_path", folderPath))

	return folderPath, nil
}

// GetInstanceFolderPath returns the path for an instance folder
// Does not create the folder if it doesn't exist
func (m *FolderManager) GetInstanceFolderPath(larkInstanceID string) string {
	safeName := m.SanitizeFolderName(larkInstanceID)
	return filepath.Join(m.baseDir, safeName)
}

// FolderExists checks if instance folder already exists
func (m *FolderManager) FolderExists(larkInstanceID string) bool {
	folderPath := m.GetInstanceFolderPath(larkInstanceID)
	info, err := os.Stat(folderPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// DeleteInstanceFolder removes an instance folder and all contents
func (m *FolderManager) DeleteInstanceFolder(larkInstanceID string) error {
	folderPath := m.GetInstanceFolderPath(larkInstanceID)

	// Check if folder exists first
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		// Folder doesn't exist - idempotent, return success
		return nil
	}

	if err := os.RemoveAll(folderPath); err != nil {
		m.logger.Error("Failed to delete instance folder",
			zap.String("lark_instance_id", larkInstanceID),
			zap.String("folder_path", folderPath),
			zap.Error(err))
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	m.logger.Debug("Deleted instance folder",
		zap.String("lark_instance_id", larkInstanceID),
		zap.String("folder_path", folderPath))

	return nil
}

// SanitizeFolderName returns a filesystem-safe version of the name
// Removes path separators and special characters to prevent directory traversal
func (m *FolderManager) SanitizeFolderName(name string) string {
	// Remove path separators and parent directory references
	name = strings.ReplaceAll(name, "..", "")
	name = strings.ReplaceAll(name, "/", "")
	name = strings.ReplaceAll(name, "\\", "")

	// Remove other unsafe characters for filesystems
	// Keep only alphanumeric, hyphens, and underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	name = re.ReplaceAllString(name, "")

	return name
}
