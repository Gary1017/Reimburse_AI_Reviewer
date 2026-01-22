// internal/storage/file_storage.go
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// FileType represents the type of file being stored
type FileType int

const (
	FileTypeGeneric FileType = iota
	FileTypePDF
	FileTypeExcel
	FileTypeImage
)

// FileStorage defines the interface for file storage operations
type FileStorage interface {
	// SaveFile writes content to the specified full path
	// Creates parent directories if needed
	SaveFile(fullPath string, content []byte) error

	// SaveFileWithType allows type-specific handling
	SaveFileWithType(fullPath string, content []byte, fileType FileType) error

	// ValidatePath checks path security (no traversal, within base)
	ValidatePath(fullPath string) error
}

// LocalFileStorage implements FileStorage for local filesystem
type LocalFileStorage struct {
	baseDir string
	logger  *zap.Logger
}

// NewLocalFileStorage creates a new LocalFileStorage
func NewLocalFileStorage(baseDir string, logger *zap.Logger) *LocalFileStorage {
	return &LocalFileStorage{
		baseDir: baseDir,
		logger:  logger,
	}
}

// SaveFile writes content to the specified full path
func (s *LocalFileStorage) SaveFile(fullPath string, content []byte) error {
	return s.SaveFileWithType(fullPath, content, FileTypeGeneric)
}

// SaveFileWithType writes content with type-specific handling
func (s *LocalFileStorage) SaveFileWithType(fullPath string, content []byte, fileType FileType) error {
	// Validate path security
	if err := s.ValidatePath(fullPath); err != nil {
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
		zap.Int("size", len(content)),
		zap.Int("file_type", int(fileType)))

	return nil
}

// ValidatePath checks that the path is safe and within baseDir
func (s *LocalFileStorage) ValidatePath(fullPath string) error {
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
