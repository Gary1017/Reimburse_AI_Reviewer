# File Storage Architecture Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a universal FileStorage utility and refactor AsyncDownloadWorker to create folders on first download.

**Architecture:** Extract file storage into `internal/storage/file_storage.go`, remove storage logic from AttachmentHandler, wire FolderManager into AsyncDownloadWorker. Callers build full paths and pass to FileStorage.

**Tech Stack:** Go, testify/assert, zap logger, excelize (existing)

---

## Task 1: Create FileStorage Interface and Implementation

**Files:**
- Create: `internal/storage/file_storage.go`
- Create: `internal/storage/file_storage_test.go`

**Step 1: Write the failing test for SaveFile**

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/storage/... -run TestLocalFileStorage_SaveFile`
Expected: FAIL with "undefined: NewLocalFileStorage"

**Step 3: Write minimal implementation**

```go
// internal/storage/file_storage.go
package storage

import (
	"fmt"
	"os"
	"path/filepath"

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
	if !filepath.HasPrefix(absPath, absBase) {
		return fmt.Errorf("path escapes base directory: %s", fullPath)
	}

	// Check for path traversal attempts
	if filepath.Clean(fullPath) != fullPath {
		// Path contains .. or other suspicious elements
		cleaned := filepath.Clean(fullPath)
		if cleaned != fullPath && filepath.HasPrefix(fullPath, "..") {
			return fmt.Errorf("path traversal detected: %s", fullPath)
		}
	}

	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/storage/... -run TestLocalFileStorage_SaveFile`
Expected: PASS

**Step 5: Write ValidatePath tests**

```go
// Add to internal/storage/file_storage_test.go

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
}
```

**Step 6: Run all storage tests**

Run: `go test -v ./internal/storage/...`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/storage/file_storage.go internal/storage/file_storage_test.go
git commit -m "feat(storage): add universal FileStorage utility (ARCH-014-B)"
```

---

## Task 2: Add FolderManagerInterface to AsyncDownloadWorker

**Files:**
- Modify: `internal/worker/async_download.go`
- Modify: `internal/worker/async_download_test.go`

**Step 1: Write the failing test for folder creation**

```go
// Add to internal/worker/async_download_test.go

// MockFolderManager for testing
type MockFolderManager struct {
	mu                       sync.RWMutex
	createFolderCallCount    int
	getFolderPathCallCount   int
	createdFolders           map[string]string
	expectedCreateError      error
}

func NewMockFolderManager() *MockFolderManager {
	return &MockFolderManager{
		createdFolders: make(map[string]string),
	}
}

func (m *MockFolderManager) CreateInstanceFolder(larkInstanceID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createFolderCallCount++
	if m.expectedCreateError != nil {
		return "", m.expectedCreateError
	}
	folderPath := fmt.Sprintf("/attachments/%s", larkInstanceID)
	m.createdFolders[larkInstanceID] = folderPath
	return folderPath, nil
}

func (m *MockFolderManager) GetInstanceFolderPath(larkInstanceID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getFolderPathCallCount++
	return fmt.Sprintf("/attachments/%s", larkInstanceID)
}

func (m *MockFolderManager) FolderExists(larkInstanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.createdFolders[larkInstanceID]
	return exists
}

func (m *MockFolderManager) DeleteInstanceFolder(larkInstanceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.createdFolders, larkInstanceID)
	return nil
}

func (m *MockFolderManager) SanitizeFolderName(name string) string {
	return name
}

// TEST-014-B-01: AsyncDownloadWorker creates folder before download
func TestAsyncDownloadWorker_CreatesFolderBeforeDownload(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()
	mockFolderMgr := NewMockFolderManager()
	mockFileStorage := NewMockFileStorage()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)
	worker.SetFolderManager(mockFolderMgr)
	worker.SetFileStorage(mockFileStorage)

	att := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		LarkInstanceID: "LARK-12345",
		FileName:       "invoice.pdf",
		URL:            "https://example.com/file",
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(att)

	err := worker.ProcessNow()
	require.NoError(t, err)

	// Verify folder was created
	assert.Equal(t, 1, mockFolderMgr.createFolderCallCount)
	assert.True(t, mockFolderMgr.FolderExists("LARK-12345"))
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worker/... -run TestAsyncDownloadWorker_CreatesFolderBeforeDownload`
Expected: FAIL with "undefined: SetFolderManager" or similar

**Step 3: Add FolderManagerInterface and FileStorageInterface to worker**

```go
// Add to internal/worker/async_download.go

// FolderManagerInterface defines folder management operations
type FolderManagerInterface interface {
	CreateInstanceFolder(larkInstanceID string) (string, error)
	GetInstanceFolderPath(larkInstanceID string) string
	FolderExists(larkInstanceID string) bool
	DeleteInstanceFolder(larkInstanceID string) error
	SanitizeFolderName(name string) string
}

// FileStorageInterface defines file storage operations
type FileStorageInterface interface {
	SaveFile(fullPath string, content []byte) error
	ValidatePath(fullPath string) error
}

// Add fields to AsyncDownloadWorker struct:
// folderManager  FolderManagerInterface
// fileStorage    FileStorageInterface

// Add setter methods:
func (w *AsyncDownloadWorker) SetFolderManager(fm FolderManagerInterface) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.folderManager = fm
}

func (w *AsyncDownloadWorker) SetFileStorage(fs FileStorageInterface) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.fileStorage = fs
}
```

**Step 4: Update downloadSingleAttachment to use FolderManager and FileStorage**

```go
// In downloadSingleAttachment, before downloading:

// Step 1: Ensure instance folder exists (create on first download)
if w.folderManager != nil {
	folderPath, err := w.folderManager.CreateInstanceFolder(task.LarkInstanceID)
	if err != nil {
		w.logger.Error("Failed to create instance folder",
			zap.String("lark_instance_id", task.LarkInstanceID),
			zap.Error(err))
		return w.attachmentRepo.UpdateStatus(nil, task.AttachmentID,
			models.AttachmentStatusFailed, fmt.Sprintf("Folder creation failed: %v", err))
	}
	w.logger.Debug("Instance folder ready", zap.String("folder_path", folderPath))
}

// ... after downloading, replace SaveFileToStorage call:

// Build full path: folderPath + filename
var filePath string
if w.folderManager != nil && w.fileStorage != nil {
	folderPath := w.folderManager.GetInstanceFolderPath(task.LarkInstanceID)
	fileName := w.attachmentHandler.GenerateFileName(task.LarkInstanceID, task.AttachmentID, task.FileName, false, item)
	fullPath := filepath.Join(folderPath, fileName)

	if err := w.fileStorage.SaveFile(fullPath, file.Content); err != nil {
		// handle error
	}
	filePath = fullPath
} else {
	// Fallback to old behavior for backward compatibility
	filePath, err = w.attachmentHandler.SaveFileToStorage(safeFileName, file.Content, true)
}
```

**Step 5: Run test to verify it passes**

Run: `go test -v ./internal/worker/... -run TestAsyncDownloadWorker_CreatesFolderBeforeDownload`
Expected: PASS

**Step 6: Add MockFileStorage to test file**

```go
// Add to internal/worker/async_download_test.go

type MockFileStorage struct {
	mu            sync.RWMutex
	savedFiles    map[string][]byte
	saveCallCount int
	expectedError error
}

func NewMockFileStorage() *MockFileStorage {
	return &MockFileStorage{
		savedFiles: make(map[string][]byte),
	}
}

func (m *MockFileStorage) SaveFile(fullPath string, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCallCount++
	if m.expectedError != nil {
		return m.expectedError
	}
	m.savedFiles[fullPath] = content
	return nil
}

func (m *MockFileStorage) ValidatePath(fullPath string) error {
	return nil
}
```

**Step 7: Run all worker tests**

Run: `go test -v ./internal/worker/...`
Expected: All PASS

**Step 8: Commit**

```bash
git add internal/worker/async_download.go internal/worker/async_download_test.go
git commit -m "feat(worker): add FolderManager and FileStorage to AsyncDownloadWorker (ARCH-014-B)"
```

---

## Task 3: Update FormPackager Filename

**Files:**
- Modify: `internal/voucher/form_packager.go`
- Modify: `internal/voucher/form_packager_test.go`

**Step 1: Write the failing test for new filename format**

```go
// Add to internal/voucher/form_packager_test.go

func TestFormPackager_FormFileNameFormat(t *testing.T) {
	// ... setup mocks ...

	t.Run("generates form filename with form_ prefix", func(t *testing.T) {
		result, err := packager.GenerateFormPackage(ctx, 1)

		require.NoError(t, err)
		assert.Contains(t, result.FormFilePath, "form_LARK-12345.xlsx")
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v ./internal/voucher/... -run TestFormPackager_FormFileNameFormat`
Expected: FAIL - filename does not contain "form_" prefix

**Step 3: Update form filename in form_packager.go**

```go
// In GenerateFormPackageWithOptions, line ~81, change:
// formFileName := fmt.Sprintf("%s.xlsx", formData.LarkInstanceID)
// To:
formFileName := fmt.Sprintf("form_%s.xlsx", formData.LarkInstanceID)
```

**Step 4: Run test to verify it passes**

Run: `go test -v ./internal/voucher/... -run TestFormPackager_FormFileNameFormat`
Expected: PASS

**Step 5: Run all voucher tests**

Run: `go test -v ./internal/voucher/...`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/voucher/form_packager.go internal/voucher/form_packager_test.go
git commit -m "feat(voucher): change form filename to form_{larkInstanceID}.xlsx (ARCH-014-B)"
```

---

## Task 4: Remove Storage Methods from AttachmentHandler

**Files:**
- Modify: `internal/lark/attachment_handler.go`
- Modify: `internal/lark/attachment_handler_test.go`
- Modify: `internal/worker/async_download.go` (update interface)

**Step 1: Update AttachmentHandlerInterface in worker to remove storage methods**

```go
// In internal/worker/async_download.go, update interface:
type AttachmentHandlerInterface interface {
	DownloadAttachmentWithRetry(ctx context.Context, url, token string, maxAttempts int) (*models.AttachmentFile, error)
	GenerateFileName(larkInstanceID string, attachmentID int64, originalName string, withSubdir bool, item *models.ReimbursementItem) string
	// Removed: SaveFileToStorage
	// Removed: ValidatePath
}
```

**Step 2: Run tests to identify any failures**

Run: `go test -v ./internal/worker/...`
Expected: May fail if mocks still reference old methods

**Step 3: Update MockAttachmentHandler to remove storage methods**

```go
// In internal/worker/async_download_test.go, remove from MockAttachmentHandler:
// - SaveFileToStorage method
// - ValidatePath method
```

**Step 4: Remove methods from attachment_handler.go**

Remove these methods from `internal/lark/attachment_handler.go`:
- `SaveFileToStorage(filename string, content []byte, withSubdir bool) (string, error)`
- `ValidatePath(baseDir, filename string, allowSubdir bool) error`

**Step 5: Remove tests for deleted methods from attachment_handler_test.go**

Remove test functions:
- `TestAttachmentHandlerPathValidation`
- `TestAttachmentHandlerSaveFileToStorage`

**Step 6: Run all tests**

Run: `go test -v ./...`
Expected: All PASS

**Step 7: Commit**

```bash
git add internal/lark/attachment_handler.go internal/lark/attachment_handler_test.go internal/worker/async_download.go internal/worker/async_download_test.go
git commit -m "refactor(lark): remove storage methods from AttachmentHandler (ARCH-014-B)

Storage responsibility moved to internal/storage/file_storage.go"
```

---

## Task 5: Wire Dependencies in Application

**Files:**
- Modify: `cmd/server/main.go` (or wherever dependencies are wired)

**Step 1: Review current wiring**

Run: `grep -n "AsyncDownloadWorker\|FolderManager\|AttachmentHandler" cmd/server/main.go`

**Step 2: Add FileStorage and FolderManager to worker initialization**

```go
// In cmd/server/main.go or application setup:

// Create storage components
folderManager := storage.NewFolderManager(cfg.AttachmentsDir, logger)
fileStorage := storage.NewLocalFileStorage(cfg.AttachmentsDir, logger)

// Create worker with new dependencies
asyncWorker := worker.NewAsyncDownloadWorker(
	attachmentRepo,
	attachmentHandler,
	larkClient,
	logger,
)
asyncWorker.SetFolderManager(folderManager)
asyncWorker.SetFileStorage(fileStorage)
```

**Step 3: Run the application and verify startup**

Run: `go build ./cmd/server && ./bin/server` (or `make run`)
Expected: Server starts without errors

**Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): wire FileStorage and FolderManager into AsyncDownloadWorker"
```

---

## Task 6: Integration Test

**Files:**
- Create: `internal/storage/integration_test.go` (optional, if time permits)

**Step 1: Write integration test for full download flow**

```go
// internal/storage/integration_test.go
package storage_test

import (
	"testing"
	"path/filepath"
	"os"

	"github.com/garyjia/ai-reimbursement/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

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

	// 2. Save invoice file to folder
	invoicePath := filepath.Join(folderPath, "invoice_TEST-INTEGRATION-001_100.00 CNY.pdf")
	err = fileStorage.SaveFile(invoicePath, []byte("PDF content"))
	require.NoError(t, err)
	assert.FileExists(t, invoicePath)

	// 3. Save form file to folder
	formPath := filepath.Join(folderPath, "form_TEST-INTEGRATION-001.xlsx")
	err = fileStorage.SaveFile(formPath, []byte("Excel content"))
	require.NoError(t, err)
	assert.FileExists(t, formPath)

	// 4. Verify folder structure
	entries, err := os.ReadDir(folderPath)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
}
```

**Step 2: Run integration test**

Run: `go test -v ./internal/storage/... -run TestIntegration`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/storage/integration_test.go
git commit -m "test(storage): add integration test for folder and file storage"
```

---

## Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Create FileStorage utility | `internal/storage/file_storage.go`, `*_test.go` |
| 2 | Add FolderManager to AsyncDownloadWorker | `internal/worker/async_download.go`, `*_test.go` |
| 3 | Update form filename format | `internal/voucher/form_packager.go` |
| 4 | Remove storage from AttachmentHandler | `internal/lark/attachment_handler.go` |
| 5 | Wire dependencies in main | `cmd/server/main.go` |
| 6 | Integration test | `internal/storage/integration_test.go` |

**Total commits:** 6
**Estimated test count:** ~15 new tests
