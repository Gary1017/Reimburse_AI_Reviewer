// internal/storage/file_storage_test.go
package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLocalFileStorage_SaveFile(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fs := NewLocalFileStorage(tempDir, logger)

	t.Run("saves file successfully", func(t *testing.T) {
		fullPath := filepath.Join(tempDir, "test-instance", "invoice.pdf")
		content := []byte("PDF content here")

		err := fs.SaveFile(fullPath, content)

		require.NoError(t, err)
		assert.FileExists(t, fullPath)

		// Verify content
		savedContent, err := os.ReadFile(fullPath)
		require.NoError(t, err)
		assert.Equal(t, content, savedContent)
	})

	t.Run("creates parent directories", func(t *testing.T) {
		fullPath := filepath.Join(tempDir, "deep", "nested", "dir", "file.pdf")
		content := []byte("content")

		err := fs.SaveFile(fullPath, content)

		require.NoError(t, err)
		assert.FileExists(t, fullPath)
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		fullPath := filepath.Join(tempDir, "overwrite", "file.txt")

		// First write
		err := fs.SaveFile(fullPath, []byte("original"))
		require.NoError(t, err)

		// Second write
		err = fs.SaveFile(fullPath, []byte("updated"))
		require.NoError(t, err)

		content, _ := os.ReadFile(fullPath)
		assert.Equal(t, []byte("updated"), content)
	})
}

func TestLocalFileStorage_ValidatePath(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fs := NewLocalFileStorage(tempDir, logger)

	t.Run("accepts valid path within base", func(t *testing.T) {
		validPath := filepath.Join(tempDir, "instance", "file.pdf")
		err := fs.ValidatePath(validPath)
		assert.NoError(t, err)
	})

	t.Run("rejects path outside base directory", func(t *testing.T) {
		outsidePath := "/etc/passwd"
		err := fs.ValidatePath(outsidePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "escapes base directory")
	})

	t.Run("rejects path traversal attempt", func(t *testing.T) {
		traversalPath := filepath.Join(tempDir, "..", "..", "etc", "passwd")
		err := fs.ValidatePath(traversalPath)
		assert.Error(t, err)
	})

	t.Run("rejects path with similar prefix", func(t *testing.T) {
		// If base is /tmp/test123, this should fail
		similarPrefixPath := tempDir + "_malicious/file.txt"
		err := fs.ValidatePath(similarPrefixPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "escapes base directory")
	})
}

func TestLocalFileStorage_SaveFile_EmptyContent(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fs := NewLocalFileStorage(tempDir, logger)

	t.Run("saves empty file", func(t *testing.T) {
		fullPath := filepath.Join(tempDir, "empty.txt")
		err := fs.SaveFile(fullPath, []byte{})
		require.NoError(t, err)

		info, _ := os.Stat(fullPath)
		assert.Equal(t, int64(0), info.Size())
	})
}
