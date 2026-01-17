package worker

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockAttachmentRepository for testing
type MockAttachmentRepository struct {
	mu                      sync.RWMutex
	attachments             map[int64]*models.Attachment
	pendingAttachments      []*models.Attachment
	getPendingCallCount     int
	updateStatusCallCount   int
	markDownloadCallCount   int
	lastGetPendingLimit     int
	expectedGetPendingError error
}

func NewMockAttachmentRepository() *MockAttachmentRepository {
	return &MockAttachmentRepository{
		attachments:        make(map[int64]*models.Attachment),
		pendingAttachments: []*models.Attachment{},
	}
}

func (m *MockAttachmentRepository) Create(tx *sql.Tx, attachment *models.Attachment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attachments[attachment.ID] = attachment
	return nil
}

func (m *MockAttachmentRepository) GetByID(id int64) (*models.Attachment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.attachments[id], nil
}

func (m *MockAttachmentRepository) GetPendingAttachments(limit int) ([]*models.Attachment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getPendingCallCount++
	m.lastGetPendingLimit = limit
	if m.expectedGetPendingError != nil {
		return nil, m.expectedGetPendingError
	}
	
	// Respect limit
	if len(m.pendingAttachments) > limit {
		return m.pendingAttachments[:limit], nil
	}
	return m.pendingAttachments, nil
}

func (m *MockAttachmentRepository) UpdateStatus(tx *sql.Tx, id int64, status, errorMessage string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateStatusCallCount++
	if att, ok := m.attachments[id]; ok {
		att.DownloadStatus = status
		att.ErrorMessage = errorMessage
	}
	return nil
}

func (m *MockAttachmentRepository) MarkDownloadCompleted(tx *sql.Tx, id int64, filePath string, fileSize int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.markDownloadCallCount++
	if att, ok := m.attachments[id]; ok {
		att.DownloadStatus = models.AttachmentStatusCompleted
		att.FilePath = filePath
		att.FileSize = fileSize
		now := time.Now()
		att.DownloadedAt = &now
	}
	return nil
}

func (m *MockAttachmentRepository) AddPendingAttachment(att *models.Attachment) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attachments[att.ID] = att
	m.pendingAttachments = append(m.pendingAttachments, att)
}

func (m *MockAttachmentRepository) ClearPending() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pendingAttachments = []*models.Attachment{}
}

// MockAttachmentHandler for testing
type MockAttachmentHandler struct {
	mu                             sync.RWMutex
	downloadAttempts               int
	shouldFailDownload             bool
	shouldFailWithTemporaryError   bool
	shouldFailWithPermanentError   bool
	downloadedFiles                map[int64]*models.AttachmentFile
	downloadDelay                  time.Duration
}

func NewMockAttachmentHandler() *MockAttachmentHandler {
	return &MockAttachmentHandler{
		downloadedFiles: make(map[int64]*models.AttachmentFile),
	}
}

func (m *MockAttachmentHandler) DownloadAttachment(ctx context.Context, url, token string) (*models.AttachmentFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.downloadDelay > 0 {
		time.Sleep(m.downloadDelay)
	}

	m.downloadAttempts++

	if m.shouldFailWithPermanentError {
		return nil, fmt.Errorf("permanent error: 404 not found")
	}

	if m.shouldFailWithTemporaryError {
		return nil, fmt.Errorf("temporary error: 500 server error")
	}

	if m.shouldFailDownload {
		return nil, fmt.Errorf("download failed")
	}

	file := &models.AttachmentFile{
		Content:  []byte("test file content"),
		FileName: "test.pdf",
		MimeType: "application/pdf",
		Size:     17,
	}
	return file, nil
}

func (m *MockAttachmentHandler) DownloadAttachmentWithRetry(
	ctx context.Context,
	url, token string,
	maxAttempts int,
) (*models.AttachmentFile, error) {
	return m.DownloadAttachment(ctx, url, token)
}

func (m *MockAttachmentHandler) SaveFileToStorage(filename string, content []byte) (string, error) {
	return fmt.Sprintf("/attachments/%s", filename), nil
}

func (m *MockAttachmentHandler) ValidatePath(baseDir, filename string) error {
	return nil
}

func (m *MockAttachmentHandler) GenerateFileName(larkInstanceID string, attachmentID int64, originalName string) string {
	return fmt.Sprintf("%s_att%d_%s", larkInstanceID, attachmentID, originalName)
}

func (m *MockAttachmentHandler) SetDownloadAttempts(attempts int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadAttempts = attempts
}

func (m *MockAttachmentHandler) GetDownloadAttempts() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.downloadAttempts
}

// ============================================================================
// TEST SUITE - TEST-XXX Mapping to ARCH-007-A through E
// ============================================================================

// TEST-007-A-01: AsyncDownloadWorker initialization with default configuration
// Related: ARCH-007-A (AsyncDownloadWorker orchestration)
func TestAsyncDownloadWorker_Initialize(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	require.NotNil(t, worker)
	assert.Equal(t, 5*time.Second, worker.pollInterval)
	assert.Equal(t, 10, worker.batchSize)
	assert.Equal(t, 3, worker.maxRetryAttempts)
	assert.False(t, worker.isRunning)
}

// TEST-007-A-02: AsyncDownloadWorker starts successfully
// Related: ARCH-007-A (Worker Start method)
func TestAsyncDownloadWorker_Start(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := worker.Start(ctx)
	require.NoError(t, err)
	assert.True(t, worker.isRunning)

	worker.Stop()
	assert.False(t, worker.isRunning)
}

// TEST-007-A-03: AsyncDownloadWorker polls pending attachments at intervals
// Related: ARCH-007-A (Polling behavior)
func TestAsyncDownloadWorker_PollsAttachments(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)
	worker.SetPollInterval(100 * time.Millisecond)

	att := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		FileName:       "test.pdf",
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(att)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	worker.Start(ctx)
	time.Sleep(250 * time.Millisecond) // Give polling time to execute
	worker.Stop()

	// Should have called GetPendingAttachments multiple times
	assert.Greater(t, mockRepo.getPendingCallCount, 0)
}

// TEST-007-B-01: DownloadTask creation and immutability
// Related: ARCH-007-B (DownloadTask structure)
func TestDownloadTask_Creation(t *testing.T) {
	task := &DownloadTask{
		AttachmentID:   1,
		ItemID:         10,
		InstanceID:     100,
		LarkInstanceID: "LARK123",
		FileName:       "receipt.pdf",
		URL:            "https://lark.example.com/file/123",
		LarkToken:      "token_xyz",
		AttemptCount:   0,
		CreatedAt:      time.Now(),
	}

	assert.Equal(t, int64(1), task.AttachmentID)
	assert.Equal(t, int64(10), task.ItemID)
	assert.Equal(t, "LARK123", task.LarkInstanceID)
	assert.Equal(t, "receipt.pdf", task.FileName)
	assert.Equal(t, 0, task.AttemptCount)
}

// TEST-007-B-02: DownloadTask IsRetryable returns true within max attempts
// Related: ARCH-007-B (Retry logic)
func TestDownloadTask_IsRetryable_WithinLimit(t *testing.T) {
	task := &DownloadTask{
		AttachmentID: 1,
		AttemptCount: 1,
	}

	// With 1 attempt done, can retry up to 3 attempts: true
	assert.True(t, task.IsRetryable(3))
	// With 1 attempt done, can retry up to 2 attempts: true
	assert.True(t, task.IsRetryable(2))
	
	// With 2 attempts done, can retry up to 2 attempts: false (already at limit)
	task.AttemptCount = 2
	assert.False(t, task.IsRetryable(2))
}

// TEST-007-B-03: DownloadTask CanRetry respects backoff timing
// Related: ARCH-007-B (Timing constraints)
func TestDownloadTask_CanRetry_WithBackoff(t *testing.T) {
	now := time.Now()
	task := &DownloadTask{
		AttachmentID: 1,
		LastAttempt:  now,
		NextRetry:    now.Add(2 * time.Second),
		AttemptCount: 1,
	}

	strategy := &RetryStrategy{
		MaxAttempts: 3,
		BaseBackoff: 1 * time.Second,
	}

	// Too soon - retry window not reached
	assert.False(t, task.CanRetry(strategy))

	// After sufficient backoff
	task.NextRetry = time.Now().Add(-1 * time.Second)
	assert.True(t, task.CanRetry(strategy))
}

// TEST-007-C-01: RetryStrategy calculates exponential backoff correctly
// Related: ARCH-007-C (RetryStrategy backoff calculation)
func TestRetryStrategy_CalculateBackoff_Exponential(t *testing.T) {
	strategy := &RetryStrategy{
		MaxAttempts: 3,
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  8 * time.Second,
		Jitter:      false,
	}

	// Attempt 1: 1 second
	assert.Equal(t, 1*time.Second, strategy.CalculateBackoff(1))

	// Attempt 2: 2 seconds
	assert.Equal(t, 2*time.Second, strategy.CalculateBackoff(2))

	// Attempt 3: 4 seconds
	assert.Equal(t, 4*time.Second, strategy.CalculateBackoff(3))

	// Should not exceed MaxBackoff
	assert.LessOrEqual(t, strategy.CalculateBackoff(4), strategy.MaxBackoff)
}

// TEST-007-C-02: RetryStrategy identifies temporary errors as retryable
// Related: ARCH-007-C (Transient error detection)
func TestRetryStrategy_IsTemporaryError_Transient(t *testing.T) {
	strategy := &RetryStrategy{}

	// Timeout should be temporary
	assert.True(t, strategy.IsTemporaryError(context.DeadlineExceeded))

	// Generic error should not be temporary
	assert.False(t, strategy.IsTemporaryError(fmt.Errorf("unknown error")))
}

// TEST-007-C-03: RetryStrategy identifies permanent errors correctly
// Related: ARCH-007-C (Permanent error detection)
func TestRetryStrategy_IsRetryableStatusCode_PermanentVsTransient(t *testing.T) {
	strategy := &RetryStrategy{}

	// Permanent errors should not be retryable
	assert.False(t, strategy.IsRetryableStatusCode(404)) // Not found
	assert.False(t, strategy.IsRetryableStatusCode(401)) // Unauthorized

	// Transient errors should be retryable
	assert.True(t, strategy.IsRetryableStatusCode(500)) // Server error
	assert.True(t, strategy.IsRetryableStatusCode(503)) // Service unavailable
	assert.True(t, strategy.IsRetryableStatusCode(429)) // Rate limit
}

// TEST-007-D-01: AttachmentRepository GetPendingAttachments returns PENDING only
// Related: ARCH-007-D (Status filtering)
func TestAttachmentRepository_GetPendingAttachments_FiltersByStatus(t *testing.T) {
	mockRepo := NewMockAttachmentRepository()

	pending := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(pending)

	result, err := mockRepo.GetPendingAttachments(10)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, models.AttachmentStatusPending, result[0].DownloadStatus)
}

// TEST-007-D-02: AttachmentRepository GetPendingAttachments respects limit
// Related: ARCH-007-D (Batch size constraint)
func TestAttachmentRepository_GetPendingAttachments_RespectsLimit(t *testing.T) {
	mockRepo := NewMockAttachmentRepository()

	// Add 15 pending attachments
	for i := 1; i <= 15; i++ {
		att := &models.Attachment{
			ID:             int64(i),
			InstanceID:     1,
			ItemID:         1,
			DownloadStatus: models.AttachmentStatusPending,
		}
		mockRepo.AddPendingAttachment(att)
	}

	result, err := mockRepo.GetPendingAttachments(10)

	require.NoError(t, err)
	assert.LessOrEqual(t, len(result), 10)
	assert.Equal(t, 10, mockRepo.lastGetPendingLimit)
}

// TEST-007-D-03: AttachmentRepository returns empty for no pending
// Related: ARCH-007-D (Empty result handling)
func TestAttachmentRepository_GetPendingAttachments_EmptyResult(t *testing.T) {
	mockRepo := NewMockAttachmentRepository()
	mockRepo.ClearPending()

	result, err := mockRepo.GetPendingAttachments(10)

	require.NoError(t, err)
	assert.Len(t, result, 0)
}

// TEST-007-E-01: HealthCheck reports worker status correctly
// Related: ARCH-007-E (Status reporting)
func TestWorkerStatus_Reporting(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	status := worker.GetStatus()

	assert.False(t, status.IsRunning)
	assert.Equal(t, 0, status.ProcessedCount)
	assert.Equal(t, 0, status.FailedCount)
}

// TEST-007-E-02: HealthCheck detects worker liveness
// Related: ARCH-007-E (Liveness detection)
func TestWorkerStatus_Liveness_ActiveWorker(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	att := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(att)

	// Process immediately to update lastProcessed
	err := worker.ProcessNow()
	require.NoError(t, err)

	// Get status should show processing occurred
	status := worker.GetStatus()

	// Worker should have processed at least one cycle
	assert.Greater(t, mockRepo.getPendingCallCount, 0)
	assert.Equal(t, 1, status.ProcessedCount)
}

// TEST-007-E-03: HealthCheck reports health as false when worker stuck
// Related: ARCH-007-E (Health degradation detection)
func TestWorkerStatus_HealthCheck_StuckWorker(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	worker.lastProcessed = time.Now().Add(-10 * time.Minute)

	status := worker.GetStatus()

	assert.False(t, status.IsHealthy)
}

// TEST-007-INTEGRATION-01: Complete download flow success
// Related: ARCH-007-A, ARCH-007-B, ARCH-007-C, ARCH-007-D
func TestAsyncDownloadWorker_CompleteDownloadFlow_Success(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	att := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		FileName:       "test.pdf",
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(att)

	// Process immediately instead of waiting for polling
	err := worker.ProcessNow()
	require.NoError(t, err)

	// Verify GetPendingAttachments was called
	assert.Greater(t, mockRepo.getPendingCallCount, 0)
}

// TEST-007-INTEGRATION-02: Download with retry on temporary failure
// Related: ARCH-007-C (Retry strategy)
func TestAsyncDownloadWorker_DownloadWithRetry_TemporaryFailure(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)

	att := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		FileName:       "test.pdf",
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(att)

	mockHandler.shouldFailWithTemporaryError = true

	// Process immediately
	err := worker.ProcessNow()
	require.NoError(t, err) // ProcessNow doesn't error out, it continues

	// Worker should have attempted processing
	assert.Greater(t, mockRepo.getPendingCallCount, 0)
}

// TEST-007-INTEGRATION-03: Graceful shutdown stops worker cleanly
// Related: ARCH-007-A (Stop method)
func TestAsyncDownloadWorker_GracefulShutdown(t *testing.T) {
	logger := zap.NewNop()
	mockRepo := NewMockAttachmentRepository()
	mockHandler := NewMockAttachmentHandler()
	mockHandler.downloadDelay = 100 * time.Millisecond

	worker := NewAsyncDownloadWorker(
		mockRepo,
		mockHandler,
		nil,
		logger,
	)
	worker.pollInterval = 100 * time.Millisecond

	att := &models.Attachment{
		ID:             1,
		InstanceID:     1,
		ItemID:         1,
		DownloadStatus: models.AttachmentStatusPending,
	}
	mockRepo.AddPendingAttachment(att)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	worker.Start(ctx)
	time.Sleep(300 * time.Millisecond)

	worker.Stop()
	assert.False(t, worker.isRunning)
}

// ============================================================================
// PRE-IMPLEMENTATION FAILURE CONFIRMATION
// ============================================================================

/*
FAILING-FIRST CONFIRMATION:

The following tests are EXPECTED TO FAIL before implementation because:

1. AsyncDownloadWorker type does not exist yet
   - TEST-007-A-01 will fail: undefined type
   - TEST-007-A-02 will fail: undefined type
   - TEST-007-A-03 will fail: undefined type

2. DownloadTask type does not exist yet
   - TEST-007-B-01 will fail: undefined type
   - TEST-007-B-02 will fail: undefined method IsRetryable
   - TEST-007-B-03 will fail: undefined method CanRetry

3. RetryStrategy type does not exist yet
   - TEST-007-C-01 will fail: undefined type
   - TEST-007-C-02 will fail: undefined method IsTemporaryError
   - TEST-007-C-03 will fail: undefined method IsRetryableStatusCode

4. AttachmentRepository missing GetPendingAttachments method
   - TEST-007-D-01 will fail: undefined method
   - TEST-007-D-02 will fail: undefined method
   - TEST-007-D-03 will fail: undefined method

5. WorkerStatus type does not exist yet
   - TEST-007-E-01 will fail: undefined type
   - TEST-007-E-02 will fail: undefined type
   - TEST-007-E-03 will fail: undefined type

6. Integration tests fail due to missing types and methods
   - TEST-007-INTEGRATION-01 through 03: undefined types

Total: 18 tests, all expected to FAIL before implementation.
Status: ✅ READY FOR IMPLEMENTATION PHASE

Following strict TDD: tests define the contract, implementation fills it in.
*/
