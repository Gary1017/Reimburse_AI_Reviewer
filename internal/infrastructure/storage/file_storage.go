// internal/infrastructure/storage/file_storage.go
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"go.uber.org/zap"
)

// LocalFileStorage implements port.FileStorage for local filesystem
type LocalFileStorage struct {
	baseDir string
	logger  *zap.Logger
}

// NewLocalFileStorage creates a new LocalFileStorage
func NewLocalFileStorage(baseDir string, logger *zap.Logger) port.FileStorage {
	return &LocalFileStorage{
		baseDir: baseDir,
		logger:  logger,
	}
}

// Save writes content to the specified relative path
func (s *LocalFileStorage) Save(ctx context.Context, path string, content []byte) error {
	fullPath := s.GetFullPath(path)

	// Validate path security
	if err := s.validatePath(fullPath); err != nil {
		return err
	}

	// Create parent directories
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		s.logger.Error("Failed to create parent directories",
			zap.String("path", parentDir),
			zap.Error(err))
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		s.logger.Error("Failed to write file",
			zap.String("path", fullPath),
			zap.Error(err))
		return fmt.Errorf("failed to write file: %w", err)
	}

	s.logger.Debug("File saved successfully",
		zap.String("path", fullPath),
		zap.Int("size", len(content)))

	return nil
}

// Read reads content from the specified relative path
func (s *LocalFileStorage) Read(ctx context.Context, path string) ([]byte, error) {
	fullPath := s.GetFullPath(path)

	// Validate path security
	if err := s.validatePath(fullPath); err != nil {
		return nil, err
	}

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		s.logger.Error("Failed to read file",
			zap.String("path", fullPath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	s.logger.Debug("File read successfully",
		zap.String("path", fullPath),
		zap.Int("size", len(content)))

	return content, nil
}

// Exists checks if a file exists at the specified relative path
func (s *LocalFileStorage) Exists(ctx context.Context, path string) bool {
	fullPath := s.GetFullPath(path)
	_, err := os.Stat(fullPath)
	return err == nil
}

// Delete removes a file at the specified relative path
func (s *LocalFileStorage) Delete(ctx context.Context, path string) error {
	fullPath := s.GetFullPath(path)

	// Validate path security
	if err := s.validatePath(fullPath); err != nil {
		return err
	}

	// Check if file exists first
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		// File doesn't exist - idempotent, return success
		return nil
	}

	// Delete file
	if err := os.Remove(fullPath); err != nil {
		s.logger.Error("Failed to delete file",
			zap.String("path", fullPath),
			zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.logger.Debug("File deleted successfully",
		zap.String("path", fullPath))

	return nil
}

// GetFullPath converts a relative path to full path
func (s *LocalFileStorage) GetFullPath(relativePath string) string {
	return filepath.Join(s.baseDir, relativePath)
}

// validatePath checks that the path is safe and within baseDir
func (s *LocalFileStorage) validatePath(fullPath string) error {
	// Resolve to absolute path
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	absBase, err := filepath.Abs(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base path: %w", err)
	}

	// Check path is within base directory
	// Proper check: ensure path starts with base + separator or equals base
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) && absPath != absBase {
		return fmt.Errorf("path escapes base directory: %s", fullPath)
	}

	return nil
}
