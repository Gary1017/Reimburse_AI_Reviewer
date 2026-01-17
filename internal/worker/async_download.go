package worker

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

// ============================================================================
// ARCH-007-C: RetryStrategy - Implements exponential backoff retry logic
// ============================================================================

// RetryStrategy defines exponential backoff retry logic
type RetryStrategy struct {
	MaxAttempts int           // Default: 3
	BaseBackoff time.Duration // Default: 1 second
	MaxBackoff  time.Duration // Default: 8 seconds
	Jitter      bool          // Enable jitter (default: true)
}

// NewRetryStrategy creates a new RetryStrategy with defaults
func NewRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxAttempts: 3,
		BaseBackoff: 1 * time.Second,
		MaxBackoff:  8 * time.Second,
		Jitter:      true,
	}
}

// CalculateBackoff returns duration until next retry attempt
// Implements exponential backoff: 1s, 2s, 4s, 8s...
func (s *RetryStrategy) CalculateBackoff(attemptNumber int) time.Duration {
	if attemptNumber <= 0 {
		return s.BaseBackoff
	}

	// Calculate exponential backoff: 2^(n-1) * BaseBackoff
	exponent := float64(attemptNumber - 1)
	multiplier := math.Pow(2, exponent)
	backoff := time.Duration(multiplier) * s.BaseBackoff

	// Cap at MaxBackoff
	if backoff > s.MaxBackoff {
		backoff = s.MaxBackoff
	}

	// Add jitter if enabled
	if s.Jitter {
		// Add random jitter: ±10% of backoff
		jitterRange := backoff / 10
		if jitterRange > 0 {
			jitter := time.Duration(rand.Intn(int(jitterRange*2))) - jitterRange
			backoff = backoff + jitter
			if backoff < s.BaseBackoff {
				backoff = s.BaseBackoff
			}
		}
	}

	return backoff
}

// IsTemporaryError determines if error is retryable
func (s *RetryStrategy) IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Timeout errors are temporary
	if strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "Timeout") {
		return true
	}

	// Network errors are temporary
	if strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "reset by peer") {
		return true
	}

	return false
}

// IsRetryableStatusCode determines if HTTP status warrants retry
func (s *RetryStrategy) IsRetryableStatusCode(statusCode int) bool {
	// Permanent errors: 4xx except 429
	if statusCode >= 400 && statusCode < 500 {
		return statusCode == 429 // Only 429 (rate limit) is retryable in 4xx
	}

	// Transient errors: 5xx (server errors)
	if statusCode >= 500 && statusCode < 600 {
		return true
	}

	return false
}

// ============================================================================
// ARCH-007-B: DownloadTask - Encapsulates single attachment download
// ============================================================================

// DownloadTask represents a single attachment download operation
type DownloadTask struct {
	AttachmentID   int64
	ItemID         int64
	InstanceID     int64
	LarkInstanceID string
	FileName       string
	URL            string
	LarkToken      string

	AttemptCount int
	LastAttempt  time.Time
	NextRetry    time.Time
	CreatedAt    time.Time
}

// IsRetryable returns true if more retries are available
func (t *DownloadTask) IsRetryable(maxAttempts int) bool {
	return t.AttemptCount < maxAttempts
}

// CanRetry returns true if enough time has passed since last attempt
func (t *DownloadTask) CanRetry(strategy *RetryStrategy) bool {
	if !t.IsRetryable(strategy.MaxAttempts) {
		return false
	}

	return time.Now().After(t.NextRetry)
}

// ============================================================================
// ARCH-007-E: WorkerStatus - Reports worker health & state
// ============================================================================

// WorkerStatus reports current worker health
type WorkerStatus struct {
	IsRunning       bool
	LastProcessed   time.Time
	PendingCount    int
	ProcessedCount  int
	FailedCount     int
	UpSinceDuration time.Duration
	IsHealthy       bool
	LastError       error
}

// ============================================================================
// ARCH-007-A: AsyncDownloadWorker - Orchestrates background downloads
// ============================================================================

// AttachmentRepositoryInterface defines the repository contract for testing
type AttachmentRepositoryInterface interface {
	GetPendingAttachments(limit int) ([]*models.Attachment, error)
	MarkDownloadCompleted(tx *sql.Tx, id int64, filePath string, fileSize int64) error
	UpdateStatus(tx *sql.Tx, id int64, status, errorMessage string) error
}

// AttachmentHandlerInterface defines the handler contract for testing
type AttachmentHandlerInterface interface {
	DownloadAttachmentWithRetry(ctx context.Context, url, token string, maxAttempts int) (*models.AttachmentFile, error)
	SaveFileToStorage(filename string, content []byte) (string, error)
	ValidatePath(baseDir, filename string) error
	GenerateFileName(larkInstanceID string, attachmentID int64, originalName string) string
}

// AsyncDownloadWorker manages background attachment downloads
type AsyncDownloadWorker struct {
	// Configuration
	pollInterval     time.Duration
	batchSize        int
	maxRetryAttempts int
	downloadTimeout  time.Duration

	// Dependencies (using interfaces for testability)
	attachmentRepo    AttachmentRepositoryInterface
	attachmentHandler AttachmentHandlerInterface
	larkClient        interface{}
	logger            *zap.Logger

	// Runtime state
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	lastProcessed  time.Time
	isRunning      bool
	processedCount int
	failedCount    int
	startTime      time.Time
	lastError      error
	retryStrategy  *RetryStrategy
}

// NewAsyncDownloadWorker creates a new worker
func NewAsyncDownloadWorker(
	attachmentRepo AttachmentRepositoryInterface,
	attachmentHandler AttachmentHandlerInterface,
	larkClient interface{},
	logger *zap.Logger,
) *AsyncDownloadWorker {
	return &AsyncDownloadWorker{
		pollInterval:      5 * time.Second,
		batchSize:         10,
		maxRetryAttempts:  3,
		downloadTimeout:   30 * time.Second,
		attachmentRepo:    attachmentRepo,
		attachmentHandler: attachmentHandler,
		larkClient:        larkClient,
		logger:            logger,
		retryStrategy:     NewRetryStrategy(),
		lastProcessed:     time.Now(),
		startTime:         time.Now(),
	}
}

// Start begins the worker polling loop
func (w *AsyncDownloadWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return fmt.Errorf("worker already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.isRunning = true
	w.startTime = time.Now()
	w.mu.Unlock()

	w.logger.Info("AsyncDownloadWorker started",
		zap.Duration("poll_interval", w.pollInterval),
		zap.Int("batch_size", w.batchSize),
		zap.Int("max_retries", w.maxRetryAttempts))

	// Run polling loop in background goroutine
	go w.pollLoop()

	return nil
}

// Stop gracefully terminates the worker
func (w *AsyncDownloadWorker) Stop() {
	w.mu.Lock()
	if !w.isRunning {
		w.mu.Unlock()
		return
	}

	w.isRunning = false
	w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}

	w.logger.Info("AsyncDownloadWorker stopped",
		zap.Int("processed_count", w.processedCount),
		zap.Int("failed_count", w.failedCount))
}

// GetStatus returns current worker status
func (w *AsyncDownloadWorker) GetStatus() WorkerStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()

	upDuration := time.Since(w.startTime)
	isHealthy := time.Since(w.lastProcessed) < 5*time.Minute && w.isRunning

	return WorkerStatus{
		IsRunning:       w.isRunning,
		LastProcessed:   w.lastProcessed,
		ProcessedCount:  w.processedCount,
		FailedCount:     w.failedCount,
		UpSinceDuration: upDuration,
		IsHealthy:       isHealthy,
		LastError:       w.lastError,
	}
}

// pollLoop runs the main polling loop in background
func (w *AsyncDownloadWorker) pollLoop() {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Debug("Poll loop context cancelled")
			return

		case <-ticker.C:
			if err := w.processPendingAttachments(); err != nil {
				w.mu.Lock()
				w.lastError = err
				w.mu.Unlock()
				w.logger.Error("Failed to process pending attachments", zap.Error(err))
			}

			w.mu.Lock()
			w.lastProcessed = time.Now()
			w.mu.Unlock()
		}
	}
}

// processPendingAttachments polls and processes pending downloads
func (w *AsyncDownloadWorker) processPendingAttachments() error {
	// Get pending attachments
	attachments, err := w.attachmentRepo.GetPendingAttachments(w.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending attachments: %w", err)
	}

	if len(attachments) == 0 {
		return nil
	}

	w.logger.Debug("Processing pending attachments", zap.Int("count", len(attachments)))

	for _, att := range attachments {
		// Create download task
		task := &DownloadTask{
			AttachmentID:   att.ID,
			ItemID:         att.ItemID,
			InstanceID:     att.InstanceID,
			LarkInstanceID: att.LarkInstanceID,
			FileName:       att.FileName,
			URL:            att.URL, // Use URL from database
			AttemptCount:   0,
			CreatedAt:      time.Now(),
		}

		// Attempt download
		if err := w.downloadSingleAttachment(task); err != nil {
			w.logger.Warn("Failed to download attachment",
				zap.Int64("attachment_id", att.ID),
				zap.String("file_name", att.FileName),
				zap.Error(err))

			w.mu.Lock()
			w.failedCount++
			w.mu.Unlock()
		} else {
			w.mu.Lock()
			w.processedCount++
			w.mu.Unlock()
		}
	}

	return nil
}

// downloadSingleAttachment downloads one attachment with retry
// Implements ARCH-007-A: Download from Lark Drive API with error handling
func (w *AsyncDownloadWorker) downloadSingleAttachment(task *DownloadTask) error {
	// Create download context with timeout
	// Use a background context if w.ctx is not initialized (e.g., in tests)
	parentCtx := w.ctx
	if parentCtx == nil {
		parentCtx = context.Background()
	}

	ctx, cancel := context.WithTimeout(parentCtx, w.downloadTimeout)
	defer cancel()

	// Step 1: Generate safe filename first
	// Format: {lark_instance_id}_att{attachment_id}_{original_filename}
	// Using LarkInstanceID ensures it's easy to link back to the approval
	// Using AttachmentID ensures uniqueness if one approval has multiple attachments
	safeFileName := w.attachmentHandler.GenerateFileName(task.LarkInstanceID, task.AttachmentID, task.FileName)

	// Step 2: Validate file path for security (prevent directory traversal)
	if err := w.attachmentHandler.ValidatePath("attachments", safeFileName); err != nil {
		w.logger.Error("Invalid file path",
			zap.Int64("attachment_id", task.AttachmentID),
			zap.String("filename", safeFileName),
			zap.Error(err))
		return w.attachmentRepo.UpdateStatus(nil, task.AttachmentID,
			models.AttachmentStatusFailed, fmt.Sprintf("Path validation failed: %v", err))
	}

	// Step 3: Download file from Lark Drive API
	// API: GET /open-apis/drive/v1/files/{file_token}/download
	// Reference: https://go.feishu.cn/s/63soQp6OA0s
	// Note: URL is now stored in database during webhook processing (ARCH-007-D)

	// Ensure URL is available
	if task.URL == "" {
		errMsg := "Cannot download: attachment URL not available in database"
		w.logger.Error(errMsg,
			zap.Int64("attachment_id", task.AttachmentID),
			zap.String("file_name", task.FileName))
		return w.attachmentRepo.UpdateStatus(nil, task.AttachmentID,
			models.AttachmentStatusFailed, errMsg)
	}

	// Download file from Lark Drive API with retry logic
	file, err := w.attachmentHandler.DownloadAttachmentWithRetry(
		ctx,
		task.URL,
		task.LarkToken,
		w.maxRetryAttempts,
	)

	if err != nil {
		w.logger.Error("Failed to download attachment from Lark API",
			zap.Int64("attachment_id", task.AttachmentID),
			zap.String("file_name", task.FileName),
			zap.String("url", task.URL),
			zap.Error(err))

		// Check if error is temporary (retryable) or permanent
		if w.retryStrategy.IsTemporaryError(err) {
			// Temporary error - mark as failed but could be retried later
			return w.attachmentRepo.UpdateStatus(nil, task.AttachmentID,
				models.AttachmentStatusFailed, fmt.Sprintf("Download failed (temporary): %v", err))
		}

		// Permanent error - mark as failed and don't retry
		return w.attachmentRepo.UpdateStatus(nil, task.AttachmentID,
			models.AttachmentStatusFailed, fmt.Sprintf("Download failed (permanent): %v", err))
	}

	// Step 4: Save file to disk
	filePath, err := w.attachmentHandler.SaveFileToStorage(safeFileName, file.Content)
	if err != nil {
		w.logger.Error("Failed to save file to storage",
			zap.Int64("attachment_id", task.AttachmentID),
			zap.String("filename", safeFileName),
			zap.Error(err))
		return w.attachmentRepo.UpdateStatus(nil, task.AttachmentID,
			models.AttachmentStatusFailed, fmt.Sprintf("Storage error: %v", err))
	}

	// Step 5: Update database with success
	// Mark as COMPLETED and store file path and size
	w.logger.Info("Download completed successfully",
		zap.Int64("attachment_id", task.AttachmentID),
		zap.String("file_name", task.FileName),
		zap.String("file_path", filePath),
		zap.Int64("file_size", file.Size))

	return w.attachmentRepo.MarkDownloadCompleted(nil, task.AttachmentID, filePath, file.Size)
}

// ============================================================================
// Helper method to set custom retry strategy (for testing)
// ============================================================================

// SetRetryStrategy sets a custom retry strategy
func (w *AsyncDownloadWorker) SetRetryStrategy(strategy *RetryStrategy) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.retryStrategy = strategy
}

// SetPollInterval sets the polling interval (for testing)
func (w *AsyncDownloadWorker) SetPollInterval(interval time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pollInterval = interval
}

// SetBatchSize sets the batch size (for testing)
func (w *AsyncDownloadWorker) SetBatchSize(size int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.batchSize = size
}

// ProcessNow processes pending attachments immediately (for testing)
func (w *AsyncDownloadWorker) ProcessNow() error {
	return w.processPendingAttachments()
}
