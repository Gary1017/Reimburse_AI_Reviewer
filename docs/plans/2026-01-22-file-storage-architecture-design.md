# File Storage Architecture Redesign

**Date:** 2026-01-22
**Status:** Pending Implementation
**ARCH Reference:** ARCH-014-B (Instance-scoped file organization)

## Problem Statement

The current architecture has file storage logic scattered across components:
- `AttachmentHandler.SaveFileToStorage()` handles both path validation and file writing
- Folder creation happens in `FormPackager` rather than at download time
- No universal file storage utility for reuse across the project

## Design Goals

1. Create folder on first download attempt (not at form generation time)
2. Universal `FileStorage` utility callable from any component
3. Clear separation: callers build full paths, `FileStorage` handles writing
4. Polymorphism support for different file types (future extensibility)

## Architecture

### Component Responsibilities

```
┌─────────────────────────────────────────────────────────────┐
│                    internal/storage/                        │
│  ┌──────────────────┐    ┌─────────────────────────────┐   │
│  │  FolderManager   │    │       FileStorage           │   │
│  │  - CreateFolder  │    │  - SaveFile(path, content)  │   │
│  │  - GetFolderPath │    │  - ValidatePath(path)       │   │
│  │  - FolderExists  │    │  - SaveFileWithType(...)    │   │
│  └──────────────────┘    └─────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

```
Webhook arrives → Attachment records created (status: PENDING)
                           ↓
AsyncDownloadWorker picks up pending attachments
                           ↓
         FolderManager.CreateInstanceFolder(larkInstanceID)
                           ↓
         AttachmentHandler.Download() returns content
                           ↓
         Caller builds: filepath.Join(folderPath, filename)
                           ↓
         FileStorage.SaveFile(fullPath, content)
                           ↓
         After all downloads → FormPackager generates form
                           ↓
         Form: form_{larkInstanceID}.xlsx in same folder
```

### Folder Structure

```
attachments/
└── {larkInstanceID}/
    ├── invoice_{larkInstanceID}_{amount}_{currency}.pdf
    ├── invoice_{larkInstanceID}_{amount}_{currency}.pdf
    └── form_{larkInstanceID}.xlsx
```

## Interface Definitions

### FileStorage Interface

```go
// internal/storage/file_storage.go

type FileType int

const (
    FileTypeGeneric FileType = iota
    FileTypePDF
    FileTypeExcel
    FileTypeImage
)

type FileStorage interface {
    // SaveFile writes content to the specified full path
    // Creates parent directories if needed
    SaveFile(fullPath string, content []byte) error

    // SaveFileWithType allows type-specific handling
    SaveFileWithType(fullPath string, content []byte, fileType FileType) error

    // ValidatePath checks path security (no traversal, within base)
    ValidatePath(fullPath string) error
}
```

### Implementation

```go
type LocalFileStorage struct {
    baseDir string
    logger  *zap.Logger
}

func NewLocalFileStorage(baseDir string, logger *zap.Logger) *LocalFileStorage
```

## Changes Required

### 1. NEW: internal/storage/file_storage.go
- `FileType` enum
- `FileStorage` interface
- `LocalFileStorage` implementation
- Path validation logic (moved from AttachmentHandler)

### 2. MODIFY: internal/worker/async_download.go
- Add `FolderManagerInterface` dependency
- Add `FileStorageInterface` dependency
- Before download: `folderManager.CreateInstanceFolder(larkInstanceID)`
- After download: build full path, call `fileStorage.SaveFile()`
- Remove direct calls to `attachmentHandler.SaveFileToStorage()`

### 3. MODIFY: internal/lark/attachment_handler.go
- Remove `SaveFileToStorage()` method
- Remove `ValidatePath()` method
- Keep only: `DownloadAttachment()`, `DownloadAttachmentWithRetry()`, `GenerateFileName()`, `ExtractAttachmentURLs()`

### 4. MODIFY: internal/voucher/form_packager.go
- Change form filename: `{id}.xlsx` → `form_{id}.xlsx`
- Use `FileStorage` for saving (optional, excelize can save directly)

### 5. MODIFY: internal/voucher/interfaces.go
- Add `FileStorageInterface` definition

## Dependency Graph

```
AsyncDownloadWorker
  ├── FolderManagerInterface (create folder)
  ├── FileStorageInterface (save files)
  ├── AttachmentHandlerInterface (download only)
  └── AttachmentRepositoryInterface (DB operations)

FormPackager
  ├── FolderManagerInterface (get folder path)
  ├── FormFillerInterface (fill Excel template)
  └── FormDataAggregatorInterface (collect data)
```

## Testing Strategy

1. Unit tests for `LocalFileStorage`:
   - `TestSaveFile_Success`
   - `TestSaveFile_CreatesParentDirs`
   - `TestValidatePath_BlocksTraversal`
   - `TestSaveFileWithType_PDF`

2. Update `AsyncDownloadWorker` tests:
   - Mock `FolderManagerInterface`
   - Mock `FileStorageInterface`
   - Verify folder created before download

3. Update `AttachmentHandler` tests:
   - Remove tests for deleted methods

## Migration Notes

- `AttachmentHandler.SaveFileToStorage()` callers must switch to `FileStorage.SaveFile()`
- Existing files in `attachments/` remain valid (folder structure unchanged)
- Form filename change is backwards-compatible (new files only)
