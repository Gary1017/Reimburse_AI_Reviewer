package voucher

import "errors"

// Domain errors for form generation
// ARCH-013-D: Error types for form package generation

var (
	// Template errors
	ErrTemplateNotFound  = errors.New("template file not found")
	ErrInvalidTemplate   = errors.New("invalid template structure")
	ErrTemplateCorrupted = errors.New("template file is corrupted")

	// Data errors
	ErrInstanceNotFound  = errors.New("approval instance not found")
	ErrNoItemsFound      = errors.New("no reimbursement items found")
	ErrItemLimitExceeded = errors.New("item count exceeds template capacity")

	// Filesystem errors
	ErrFolderCreationFailed = errors.New("failed to create folder")
	ErrFolderAlreadyExists  = errors.New("folder already exists")
	ErrFileSaveFailed       = errors.New("failed to save file")
	ErrInvalidFolderName    = errors.New("folder name contains invalid characters")

	// Attachment errors
	ErrAttachmentsIncomplete = errors.New("some attachments not yet downloaded")
	ErrAttachmentNotFound    = errors.New("attachment file not found on disk")
)
