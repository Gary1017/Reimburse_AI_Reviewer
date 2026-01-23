package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"go.uber.org/zap"
)

// LocalFolderManager implements port.FolderManager for local filesystem
type LocalFolderManager struct {
	baseDir string
	logger  *zap.Logger
}

// NewLocalFolderManager creates a new LocalFolderManager
func NewLocalFolderManager(baseDir string, logger *zap.Logger) port.FolderManager {
	return &LocalFolderManager{
		baseDir: baseDir,
		logger:  logger,
	}
}

// CreateFolder creates a folder with the given name
// Returns the full path to the created folder or error
func (m *LocalFolderManager) CreateFolder(ctx context.Context, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("cannot create folder: empty name")
	}

	// Sanitize the folder name to prevent path traversal
	safeName := m.SanitizeName(name)
	folderPath := filepath.Join(m.baseDir, safeName)

	// Create the directory (including any parent directories)
	if err := os.MkdirAll(folderPath, 0755); err != nil {
		m.logger.Error("Failed to create folder",
			zap.String("name", name),
			zap.String("folder_path", folderPath),
			zap.Error(err))
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	m.logger.Debug("Created folder",
		zap.String("name", name),
		zap.String("folder_path", folderPath))

	return folderPath, nil
}

// GetPath returns the path for a folder
// Does not create the folder if it doesn't exist
func (m *LocalFolderManager) GetPath(name string) string {
	safeName := m.SanitizeName(name)
	return filepath.Join(m.baseDir, safeName)
}

// Exists checks if folder already exists
func (m *LocalFolderManager) Exists(name string) bool {
	folderPath := m.GetPath(name)
	info, err := os.Stat(folderPath)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// Delete removes a folder and all contents
func (m *LocalFolderManager) Delete(ctx context.Context, name string) error {
	folderPath := m.GetPath(name)

	// Check if folder exists first
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		// Folder doesn't exist - idempotent, return success
		return nil
	}

	if err := os.RemoveAll(folderPath); err != nil {
		m.logger.Error("Failed to delete folder",
			zap.String("name", name),
			zap.String("folder_path", folderPath),
			zap.Error(err))
		return fmt.Errorf("failed to delete folder: %w", err)
	}

	m.logger.Debug("Deleted folder",
		zap.String("name", name),
		zap.String("folder_path", folderPath))

	return nil
}

// SanitizeName returns a filesystem-safe version of the name
// Removes path separators and special characters to prevent directory traversal
func (m *LocalFolderManager) SanitizeName(name string) string {
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
