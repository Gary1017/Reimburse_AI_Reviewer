package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// DownloadWorkerConfig holds configuration for download worker
type DownloadWorkerConfig struct {
	PollInterval    time.Duration
	BatchSize       int
	DownloadTimeout time.Duration
}

// DefaultDownloadWorkerConfig returns default configuration
func DefaultDownloadWorkerConfig() DownloadWorkerConfig {
	return DownloadWorkerConfig{
		PollInterval:    5 * time.Second,
		BatchSize:       10,
		DownloadTimeout: 30 * time.Second,
	}
}

// DownloadWorker manages background attachment downloads
// ARCH-124: Infrastructure layer worker using port interfaces
type DownloadWorker struct {
	config DownloadWorkerConfig

	// Port dependencies
	attachmentRepo port.AttachmentRepository
	itemRepo       port.ItemRepository
	downloader     port.LarkAttachmentDownloader
	fileStorage    port.FileStorage
	folderManager  port.FolderManager
	logger         *zap.Logger

	// Runtime state
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	isRunning      bool
	lastProcessed  time.Time
	processedCount int
	failedCount    int
	startTime      time.Time
	lastError      error
}

// NewDownloadWorker creates a new download worker
func NewDownloadWorker(
	config DownloadWorkerConfig,
	attachmentRepo port.AttachmentRepository,
	itemRepo port.ItemRepository,
	downloader port.LarkAttachmentDownloader,
	fileStorage port.FileStorage,
	folderManager port.FolderManager,
	logger *zap.Logger,
) *DownloadWorker {
	return &DownloadWorker{
		config:         config,
		attachmentRepo: attachmentRepo,
		itemRepo:       itemRepo,
		downloader:     downloader,
		fileStorage:    fileStorage,
		folderManager:  folderManager,
		logger:         logger,
		lastProcessed:  time.Now(),
		startTime:      time.Now(),
	}
}

// Start begins the worker polling loop
func (w *DownloadWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.isRunning {
		w.mu.Unlock()
		return fmt.Errorf("download worker already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.isRunning = true
	w.startTime = time.Now()
	w.mu.Unlock()

	w.logger.Info("DownloadWorker started",
		zap.Duration("poll_interval", w.config.PollInterval),
		zap.Int("batch_size", w.config.BatchSize))

	// Run polling loop in background goroutine
	go w.pollLoop()

	return nil
}

// Stop gracefully terminates the worker
func (w *DownloadWorker) Stop() error {
	w.mu.Lock()
	if !w.isRunning {
		w.mu.Unlock()
		return nil
	}

	w.isRunning = false
	w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
	}

	w.logger.Info("DownloadWorker stopped",
		zap.Int("processed_count", w.processedCount),
		zap.Int("failed_count", w.failedCount))

	return nil
}

// Name returns the worker name for identification
func (w *DownloadWorker) Name() string {
	return "DownloadWorker"
}

// pollLoop runs the main polling loop in background
func (w *DownloadWorker) pollLoop() {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Debug("Poll loop context cancelled")
			return

		case <-ticker.C:
			if err := w.processPendingDownloads(); err != nil {
				w.mu.Lock()
				w.lastError = err
				w.mu.Unlock()
				w.logger.Error("Failed to process pending downloads", zap.Error(err))
			}

			w.mu.Lock()
			w.lastProcessed = time.Now()
			w.mu.Unlock()
		}
	}
}

// processPendingDownloads processes pending attachments
func (w *DownloadWorker) processPendingDownloads() error {
	ctx := w.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Get pending attachments
	attachments, err := w.attachmentRepo.GetPending(ctx, w.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending attachments: %w", err)
	}

	if len(attachments) == 0 {
		return nil
	}

	w.logger.Debug("Processing pending downloads", zap.Int("count", len(attachments)))

	for _, att := range attachments {
		if err := w.downloadAttachment(ctx, att); err != nil {
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

// downloadAttachment downloads a single attachment
func (w *DownloadWorker) downloadAttachment(ctx context.Context, att *entity.Attachment) error {
	// Create download context with timeout
	downloadCtx, cancel := context.WithTimeout(ctx, w.config.DownloadTimeout)
	defer cancel()

	// Step 1: Ensure instance folder exists
	folderPath, err := w.folderManager.CreateFolder(downloadCtx, att.LarkInstanceID)
	if err != nil {
		errMsg := fmt.Sprintf("folder creation failed: %v", err)
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, "FAILED", errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	w.logger.Debug("Instance folder ready",
		zap.String("folder_path", folderPath),
		zap.String("lark_instance_id", att.LarkInstanceID))

	// Step 2: Download file from Lark
	if att.URL == "" {
		errMsg := "attachment URL not available"
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, "FAILED", errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	content, size, err := w.downloader.DownloadWithRetry(downloadCtx, att.URL, 3)
	if err != nil {
		errMsg := fmt.Sprintf("download failed: %v", err)
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, "FAILED", errMsg)
		return err
	}

	w.logger.Debug("File downloaded from Lark",
		zap.Int64("attachment_id", att.ID),
		zap.Int64("size", size))

	// Step 3: Generate file path
	fileName := w.generateFileName(att)
	filePath := filepath.Join(folderPath, fileName)

	// Step 4: Save file to storage
	if err := w.fileStorage.Save(ctx, filePath, content); err != nil {
		errMsg := fmt.Sprintf("storage failed: %v", err)
		_ = w.attachmentRepo.UpdateStatus(ctx, att.ID, "FAILED", errMsg)
		return err
	}

	// Step 5: Update database with success
	if err := w.attachmentRepo.MarkCompleted(ctx, att.ID, filePath, size); err != nil {
		return err
	}

	w.logger.Info("Download completed successfully",
		zap.Int64("attachment_id", att.ID),
		zap.String("file_name", att.FileName),
		zap.String("file_path", filePath),
		zap.Int64("file_size", size))

	return nil
}

// generateFileName creates a filename for the attachment
func (w *DownloadWorker) generateFileName(att *entity.Attachment) string {
	// Use attachment ID and original filename to create unique name
	ext := filepath.Ext(att.FileName)
	baseName := att.FileName[:len(att.FileName)-len(ext)]
	return fmt.Sprintf("%d_%s%s", att.ID, baseName, ext)
}
