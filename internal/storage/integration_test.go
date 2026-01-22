package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestIntegration_FolderAndFileStorage tests the complete workflow of
// creating instance folders and saving files (invoices and forms)
// ARCH-014-B: Instance-scoped file organization integration test
func TestIntegration_FolderAndFileStorage(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()

	folderMgr := storage.NewFolderManager(tempDir, logger)
	fileStorage := storage.NewLocalFileStorage(tempDir, logger)

	// 1. Create instance folder
	larkInstanceID := "TEST-INTEGRATION-001"
	folderPath, err := folderMgr.CreateInstanceFolder(larkInstanceID)
	require.NoError(t, err)
	assert.DirExists(t, folderPath)
	assert.Equal(t, filepath.Join(tempDir, larkInstanceID), folderPath)

	// 2. Save invoice file to folder
	invoicePath := filepath.Join(folderPath, "invoice_TEST-INTEGRATION-001_100.00 CNY.pdf")
	invoiceContent := []byte("%PDF-1.4 fake pdf content for testing")
	err = fileStorage.SaveFile(invoicePath, invoiceContent)
	require.NoError(t, err)
	assert.FileExists(t, invoicePath)

	// Verify invoice content
	savedInvoice, err := os.ReadFile(invoicePath)
	require.NoError(t, err)
	assert.Equal(t, invoiceContent, savedInvoice)

	// 3. Save second invoice file to folder
	invoice2Path := filepath.Join(folderPath, "invoice_TEST-INTEGRATION-001_250.50 CNY.pdf")
	invoice2Content := []byte("%PDF-1.4 second invoice content")
	err = fileStorage.SaveFile(invoice2Path, invoice2Content)
	require.NoError(t, err)
	assert.FileExists(t, invoice2Path)

	// 4. Save form file to folder (using new form_ prefix)
	formPath := filepath.Join(folderPath, "form_TEST-INTEGRATION-001.xlsx")
	formContent := []byte("PK\x03\x04 fake xlsx content for testing")
	err = fileStorage.SaveFile(formPath, formContent)
	require.NoError(t, err)
	assert.FileExists(t, formPath)

	// 5. Verify folder structure - should have 3 files
	entries, err := os.ReadDir(folderPath)
	require.NoError(t, err)
	assert.Len(t, entries, 3)

	// 6. Verify all expected files exist
	expectedFiles := []string{
		"invoice_TEST-INTEGRATION-001_100.00 CNY.pdf",
		"invoice_TEST-INTEGRATION-001_250.50 CNY.pdf",
		"form_TEST-INTEGRATION-001.xlsx",
	}
	for _, expectedFile := range expectedFiles {
		fullPath := filepath.Join(folderPath, expectedFile)
		assert.FileExists(t, fullPath, "Expected file %s to exist", expectedFile)
	}

	// 7. Verify folder exists check
	assert.True(t, folderMgr.FolderExists(larkInstanceID))
	assert.False(t, folderMgr.FolderExists("NON-EXISTENT-ID"))

	// 8. Test idempotency - creating folder again should succeed
	folderPath2, err := folderMgr.CreateInstanceFolder(larkInstanceID)
	require.NoError(t, err)
	assert.Equal(t, folderPath, folderPath2)

	// 9. Files should still exist after re-creating folder
	assert.FileExists(t, invoicePath)
	assert.FileExists(t, formPath)
}

// TestIntegration_MultipleInstances tests handling of multiple approval instances
func TestIntegration_MultipleInstances(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()

	folderMgr := storage.NewFolderManager(tempDir, logger)
	fileStorage := storage.NewLocalFileStorage(tempDir, logger)

	instances := []string{"INSTANCE-001", "INSTANCE-002", "INSTANCE-003"}

	// Create folders for multiple instances
	for _, instanceID := range instances {
		folderPath, err := folderMgr.CreateInstanceFolder(instanceID)
		require.NoError(t, err)

		// Save a file in each instance folder
		filePath := filepath.Join(folderPath, "form_"+instanceID+".xlsx")
		err = fileStorage.SaveFile(filePath, []byte("content for "+instanceID))
		require.NoError(t, err)
	}

	// Verify all instance folders exist with their files
	for _, instanceID := range instances {
		assert.True(t, folderMgr.FolderExists(instanceID))

		filePath := filepath.Join(tempDir, instanceID, "form_"+instanceID+".xlsx")
		assert.FileExists(t, filePath)
	}

	// Verify base directory has 3 subdirectories
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

// TestIntegration_SecurityValidation tests that security checks work end-to-end
func TestIntegration_SecurityValidation(t *testing.T) {
	tempDir := t.TempDir()
	logger, _ := zap.NewDevelopment()

	fileStorage := storage.NewLocalFileStorage(tempDir, logger)

	t.Run("rejects path outside base directory", func(t *testing.T) {
		outsidePath := "/etc/passwd"
		err := fileStorage.SaveFile(outsidePath, []byte("malicious"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "escapes base directory")
	})

	t.Run("rejects path with similar prefix attack", func(t *testing.T) {
		// Create a path that starts with the same prefix but escapes
		attackPath := tempDir + "_malicious/evil.txt"
		err := fileStorage.SaveFile(attackPath, []byte("malicious"))
		assert.Error(t, err)
	})

	t.Run("accepts valid path within base", func(t *testing.T) {
		validPath := filepath.Join(tempDir, "valid", "file.txt")
		err := fileStorage.SaveFile(validPath, []byte("valid content"))
		assert.NoError(t, err)
		assert.FileExists(t, validPath)
	})
}
