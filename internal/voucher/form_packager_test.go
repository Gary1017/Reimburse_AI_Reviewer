package voucher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ARCH-013-D: FormPackager tests
// Tests for orchestrating form generation and file organization

// MockFormFiller implements FormFillerInterface for testing
type MockFormFiller struct {
	fillError    error
	outputPath   string
	validateErr  error
}

func (m *MockFormFiller) FillTemplate(ctx context.Context, data *FormData, outputPath string) (string, error) {
	if m.fillError != nil {
		return "", m.fillError
	}
	// Create the output file for verification
	if err := os.WriteFile(outputPath, []byte("mock excel content"), 0644); err != nil {
		return "", err
	}
	return outputPath, nil
}

func (m *MockFormFiller) ValidateTemplate() error {
	return m.validateErr
}

// MockFolderManager implements FolderManagerInterface for testing
type MockFolderManager struct {
	baseDir       string
	createdPaths  []string
	createErr     error
	deleteErr     error
	existsMap     map[string]bool
}

func (m *MockFolderManager) CreateInstanceFolder(larkInstanceID string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	path := filepath.Join(m.baseDir, larkInstanceID)
	m.createdPaths = append(m.createdPaths, path)
	// Create the folder for real
	os.MkdirAll(path, 0755)
	return path, nil
}

func (m *MockFolderManager) GetInstanceFolderPath(larkInstanceID string) string {
	return filepath.Join(m.baseDir, larkInstanceID)
}

func (m *MockFolderManager) FolderExists(larkInstanceID string) bool {
	if m.existsMap == nil {
		return false
	}
	return m.existsMap[larkInstanceID]
}

func (m *MockFolderManager) DeleteInstanceFolder(larkInstanceID string) error {
	return m.deleteErr
}

func (m *MockFolderManager) SanitizeFolderName(name string) string {
	return name
}

// MockFormDataAggregator implements FormDataAggregatorInterface for testing
type MockFormDataAggregator struct {
	formData *FormData
	err      error
}

func (m *MockFormDataAggregator) AggregateData(ctx context.Context, instanceID int64) (*FormData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.formData, nil
}

// MockAttachmentRepoForPackager implements AttachmentRepositoryInterface for testing
type MockAttachmentRepoForPackager struct {
	attachmentsByInstance map[int64][]*models.Attachment
	attachmentsByItem     map[int64][]*models.Attachment
	err                   error
}

func (m *MockAttachmentRepoForPackager) GetByInstanceID(instanceID int64) ([]*models.Attachment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.attachmentsByInstance[instanceID], nil
}

func (m *MockAttachmentRepoForPackager) GetByItemID(itemID int64) ([]*models.Attachment, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.attachmentsByItem[itemID], nil
}

func TestFormPackager_GenerateFormPackage(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	t.Run("generates complete form package", func(t *testing.T) {
		tempDir := t.TempDir()

		formData := &FormData{
			ApplicantName:  "张三",
			Department:     "技术部",
			LarkInstanceID: "TEST-PACKAGE-001",
			SubmissionDate: time.Now(),
			TotalAmount:    500.00,
			Items: []FormItemData{
				{
					SequenceNumber:    1,
					AccountingSubject: "差旅费",
					Description:       "出差",
					Amount:            500.00,
				},
			},
		}

		folderManager := &MockFolderManager{baseDir: tempDir}
		filler := &MockFormFiller{}
		aggregator := &MockFormDataAggregator{formData: formData}
		attachmentRepo := &MockAttachmentRepoForPackager{
			attachmentsByInstance: map[int64][]*models.Attachment{
				1: {
					{ID: 101, FilePath: "/some/path/receipt.pdf", DownloadStatus: models.AttachmentStatusCompleted},
				},
			},
		}
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {ID: 1, LarkInstanceID: "TEST-PACKAGE-001"},
			},
		}

		packager := NewFormPackager(filler, aggregator, folderManager, attachmentRepo, instanceRepo, logger)

		result, err := packager.GenerateFormPackage(ctx, 1)

		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotEmpty(t, result.FolderPath)
		assert.NotEmpty(t, result.FormFilePath)
		assert.Contains(t, result.FolderPath, "TEST-PACKAGE-001")
		assert.Contains(t, result.FormFilePath, "form_TEST-PACKAGE-001.xlsx")
	})

	t.Run("returns error when instance not found", func(t *testing.T) {
		tempDir := t.TempDir()

		folderManager := &MockFolderManager{baseDir: tempDir}
		filler := &MockFormFiller{}
		aggregator := &MockFormDataAggregator{err: ErrInstanceNotFound}
		attachmentRepo := &MockAttachmentRepoForPackager{}
		instanceRepo := &MockInstanceRepository{instances: map[int64]*models.ApprovalInstance{}}

		packager := NewFormPackager(filler, aggregator, folderManager, attachmentRepo, instanceRepo, logger)

		result, err := packager.GenerateFormPackage(ctx, 999)

		assert.Error(t, err)
		assert.False(t, result.Success)
	})

	t.Run("reports incomplete attachments", func(t *testing.T) {
		tempDir := t.TempDir()

		formData := &FormData{
			ApplicantName:  "李四",
			Department:     "财务部",
			LarkInstanceID: "INCOMPLETE-ATTACHMENTS",
			SubmissionDate: time.Now(),
			TotalAmount:    300.00,
			Items:          []FormItemData{},
		}

		folderManager := &MockFolderManager{baseDir: tempDir}
		filler := &MockFormFiller{}
		aggregator := &MockFormDataAggregator{formData: formData}
		attachmentRepo := &MockAttachmentRepoForPackager{
			attachmentsByInstance: map[int64][]*models.Attachment{
				1: {
					{ID: 101, FilePath: "/path/done.pdf", DownloadStatus: models.AttachmentStatusCompleted},
					{ID: 102, FilePath: "", DownloadStatus: models.AttachmentStatusPending},
					{ID: 103, FilePath: "", DownloadStatus: models.AttachmentStatusFailed},
				},
			},
		}
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {ID: 1, LarkInstanceID: "INCOMPLETE-ATTACHMENTS"},
			},
		}

		packager := NewFormPackager(filler, aggregator, folderManager, attachmentRepo, instanceRepo, logger)

		result, err := packager.GenerateFormPackage(ctx, 1)

		require.NoError(t, err)
		assert.True(t, result.Success) // Form is generated even with incomplete attachments
		assert.Equal(t, 2, result.IncompleteCount)
	})

	t.Run("handles folder creation failure", func(t *testing.T) {
		tempDir := t.TempDir()

		formData := &FormData{
			LarkInstanceID: "FOLDER-FAIL",
			SubmissionDate: time.Now(),
		}

		folderManager := &MockFolderManager{
			baseDir:   tempDir,
			createErr: ErrFolderCreationFailed,
		}
		filler := &MockFormFiller{}
		aggregator := &MockFormDataAggregator{formData: formData}
		attachmentRepo := &MockAttachmentRepoForPackager{}
		instanceRepo := &MockInstanceRepository{
			instances: map[int64]*models.ApprovalInstance{
				1: {ID: 1, LarkInstanceID: "FOLDER-FAIL"},
			},
		}

		packager := NewFormPackager(filler, aggregator, folderManager, attachmentRepo, instanceRepo, logger)

		result, err := packager.GenerateFormPackage(ctx, 1)

		assert.Error(t, err)
		assert.False(t, result.Success)
	})
}

func TestNewFormPackager(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	packager := NewFormPackager(
		&MockFormFiller{},
		&MockFormDataAggregator{},
		&MockFolderManager{},
		&MockAttachmentRepoForPackager{},
		&MockInstanceRepository{},
		logger,
	)

	assert.NotNil(t, packager)
}
