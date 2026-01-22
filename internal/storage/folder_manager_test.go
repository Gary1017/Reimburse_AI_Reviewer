package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ARCH-014-A: FolderManager tests
// Tests for instance-specific folder management

func TestFolderManager_CreateInstanceFolder(t *testing.T) {
	// Setup: create a temp directory for testing
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fm := NewFolderManager(tempDir, logger)

	t.Run("creates folder with valid LarkInstanceID", func(t *testing.T) {
		larkInstanceID := "ABC123-XYZ-456"

		folderPath, err := fm.CreateInstanceFolder(larkInstanceID)

		require.NoError(t, err)
		assert.DirExists(t, folderPath)
		assert.Equal(t, filepath.Join(tempDir, "ABC123-XYZ-456"), folderPath)
	})

	t.Run("creates folder with UUID-like LarkInstanceID", func(t *testing.T) {
		larkInstanceID := "6A3847A3-14F5-4C7E-A5D1-26C7FB0BF6EF"

		folderPath, err := fm.CreateInstanceFolder(larkInstanceID)

		require.NoError(t, err)
		assert.DirExists(t, folderPath)
		assert.Contains(t, folderPath, "6A3847A3-14F5-4C7E-A5D1-26C7FB0BF6EF")
	})

	t.Run("returns existing folder path if folder already exists", func(t *testing.T) {
		larkInstanceID := "EXISTING-FOLDER"

		// Create folder first time
		folderPath1, err := fm.CreateInstanceFolder(larkInstanceID)
		require.NoError(t, err)

		// Create folder second time - should return same path without error
		folderPath2, err := fm.CreateInstanceFolder(larkInstanceID)
		require.NoError(t, err)
		assert.Equal(t, folderPath1, folderPath2)
	})

	t.Run("returns error for empty LarkInstanceID", func(t *testing.T) {
		_, err := fm.CreateInstanceFolder("")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})
}

func TestFolderManager_GetInstanceFolderPath(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fm := NewFolderManager(tempDir, logger)

	t.Run("returns correct path for valid LarkInstanceID", func(t *testing.T) {
		larkInstanceID := "TEST-123"

		path := fm.GetInstanceFolderPath(larkInstanceID)

		assert.Equal(t, filepath.Join(tempDir, "TEST-123"), path)
	})

	t.Run("returns path even if folder does not exist", func(t *testing.T) {
		larkInstanceID := "NON-EXISTENT"

		path := fm.GetInstanceFolderPath(larkInstanceID)

		assert.NotEmpty(t, path)
		// Folder should NOT exist
		_, err := os.Stat(path)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestFolderManager_FolderExists(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fm := NewFolderManager(tempDir, logger)

	t.Run("returns true for existing folder", func(t *testing.T) {
		larkInstanceID := "EXISTS-FOLDER"
		_, err := fm.CreateInstanceFolder(larkInstanceID)
		require.NoError(t, err)

		exists := fm.FolderExists(larkInstanceID)

		assert.True(t, exists)
	})

	t.Run("returns false for non-existing folder", func(t *testing.T) {
		exists := fm.FolderExists("DOES-NOT-EXIST")

		assert.False(t, exists)
	})
}

func TestFolderManager_DeleteInstanceFolder(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fm := NewFolderManager(tempDir, logger)

	t.Run("deletes existing folder and contents", func(t *testing.T) {
		larkInstanceID := "DELETE-ME"
		folderPath, err := fm.CreateInstanceFolder(larkInstanceID)
		require.NoError(t, err)

		// Create a file inside the folder
		testFile := filepath.Join(folderPath, "test.txt")
		err = os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err)

		// Delete the folder
		err = fm.DeleteInstanceFolder(larkInstanceID)

		require.NoError(t, err)
		assert.NoDirExists(t, folderPath)
	})

	t.Run("returns no error for non-existing folder", func(t *testing.T) {
		err := fm.DeleteInstanceFolder("NEVER-EXISTED")

		// Should not error - idempotent operation
		assert.NoError(t, err)
	})
}

func TestFolderManager_SanitizeFolderName(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fm := NewFolderManager(tempDir, logger)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "keeps valid characters",
			input:    "ABC123-XYZ",
			expected: "ABC123-XYZ",
		},
		{
			name:     "removes path separators",
			input:    "../../../etc/passwd",
			expected: "etcpasswd",
		},
		{
			name:     "removes special characters",
			input:    "test<>:\"|?*file",
			expected: "testfile",
		},
		{
			name:     "preserves underscores and hyphens",
			input:    "test_file-name",
			expected: "test_file-name",
		},
		{
			name:     "handles UUID format",
			input:    "6A3847A3-14F5-4C7E-A5D1-26C7FB0BF6EF",
			expected: "6A3847A3-14F5-4C7E-A5D1-26C7FB0BF6EF",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fm.SanitizeFolderName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFolderManager_PathTraversalPrevention(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()
	fm := NewFolderManager(tempDir, logger)

	t.Run("prevents path traversal with ../", func(t *testing.T) {
		maliciousID := "../../../etc/passwd"

		folderPath, err := fm.CreateInstanceFolder(maliciousID)

		require.NoError(t, err)
		// The path should be within the base directory
		assert.True(t, filepath.HasPrefix(folderPath, tempDir))
		// Should not contain path traversal
		assert.NotContains(t, folderPath, "..")
	})

	t.Run("prevents path traversal with absolute path", func(t *testing.T) {
		maliciousID := "/etc/passwd"

		folderPath, err := fm.CreateInstanceFolder(maliciousID)

		require.NoError(t, err)
		// The path should be within the base directory
		assert.True(t, filepath.HasPrefix(folderPath, tempDir))
	})
}

func TestNewFolderManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	t.Run("creates FolderManager with valid base directory", func(t *testing.T) {
		fm := NewFolderManager("/tmp/test", logger)

		assert.NotNil(t, fm)
	})
}
