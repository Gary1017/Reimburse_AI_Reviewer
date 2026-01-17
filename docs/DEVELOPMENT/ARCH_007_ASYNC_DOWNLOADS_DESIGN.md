# ARCH-007: Async Downloads - Architectural Design

**Date**: January 17, 2026  
**Status**: ARCHITECTURE PHASE (Design Complete)  
**Version**: 1.0

---

## 1. Problem Understanding

### Current State (Phase 3)
- Attachment downloads are triggered **synchronously** during `HandleInstanceCreated`
- Each attachment download blocks the webhook response (30s timeout per file)
- Lark API calls have variable latency (100ms–5s per file)
- Large files or slow network cause cascading failures

### Impact
- **Webhook timeout risk**: 5+ attachments × 3-5s each = 15-25s blocking
- **Blocking non-critical I/O**: Attachments not needed until voucher generation
- **Cascading failures**: One slow download blocks entire approval flow
- **Scalability**: Cannot handle high-volume approvals (1000+/day)

### Business Requirement
Move attachment downloads to a **background worker** so webhook returns immediately with `PENDING` status, and downloads proceed asynchronously with retry logic.

---

## 2. Architecture Overview

### Component Diagram
```
Webhook Handler
     ↓
Instance Created (fast return)
     ↓
Create Attachment (PENDING status)
     ↓
[Queue/Repository]
     ↓
Download Worker (background goroutine)
     ↓
Fetch & Retry Logic
     ↓
Update Status (DOWNLOADED/FAILED)
```

### Design Pattern
- **Async Background Worker**: Long-running service that polls for pending attachments
- **Exponential Backoff**: Retry with 1s, 2s, 4s, 8s delays (max 3 attempts)
- **State Machine**: PENDING → DOWNLOADED | FAILED
- **Graceful Shutdown**: Clean worker termination and connection pooling
- **Observability**: Structured logging for each download operation

---

## 3. Architectural Requirements (ARCH-007)

| ID | Description | Component | Constraints / Invariants |
|---|---|---|---|
| **ARCH-007-A** | AsyncDownloadWorker orchestrates background downloads | `AsyncDownloadWorker` | <ul><li>Polls every N seconds for PENDING attachments</li><li>Processes max 10 attachments per batch</li><li>Non-blocking to main workflow</li><li>Graceful shutdown support</li></ul> |
| **ARCH-007-B** | DownloadTask encapsulates single attachment download | `DownloadTask` | <ul><li>Immutable after creation</li><li>Tracks attempt count and timestamps</li><li>Links attachment ID to download operation</li></ul> |
| **ARCH-007-C** | RetryStrategy implements exponential backoff | `RetryStrategy` | <ul><li>Max attempts: 3</li><li>Backoff: 1s, 2s, 4s (2^n)</li><li>Skip permanent errors (404, 401)</li><li>Jitter to prevent thundering herd</li></ul> |
| **ARCH-007-D** | GetPendingAttachments filters by status | `AttachmentRepository` | <ul><li>Returns only PENDING attachments</li><li>Ordered by creation time (FIFO)</li><li>Limit configurable (default 10)</li></ul> |
| **ARCH-007-E** | HealthCheck monitors worker liveness | `HealthCheck` | <ul><li>Track last processed timestamp</li><li>Expose worker status via HTTP endpoint</li><li>Alert if stuck >5 min without processing</li></ul> |

---

## 4. Interface & Class Definitions

### 4.1 AsyncDownloadWorker

```go
// AsyncDownloadWorker manages background attachment downloads
type AsyncDownloadWorker struct {
    // Configuration
    pollInterval      time.Duration       // Default: 5 seconds
    batchSize         int                 // Default: 10
    maxRetryAttempts  int                 // Default: 3
    
    // Dependencies
    attachmentRepo    *AttachmentRepository
    attachmentHandler *AttachmentHandler
    larkClient        *Client
    logger            *zap.Logger
    
    // Runtime state
    ctx               context.Context
    cancel            context.CancelFunc
    lastProcessed     time.Time
    isRunning         bool
}

// NewAsyncDownloadWorker creates a new worker
func NewAsyncDownloadWorker(
    attachmentRepo *AttachmentRepository,
    attachmentHandler *AttachmentHandler,
    larkClient *Client,
    logger *zap.Logger,
) *AsyncDownloadWorker

// Start begins the worker polling loop
func (w *AsyncDownloadWorker) Start(ctx context.Context) error

// Stop gracefully terminates the worker
func (w *AsyncDownloadWorker) Stop()

// GetStatus returns current worker status
func (w *AsyncDownloadWorker) GetStatus() WorkerStatus

// processPendingAttachments polls and processes pending downloads
func (w *AsyncDownloadWorker) processPendingAttachments() error

// downloadSingleAttachment downloads one attachment with retry
func (w *AsyncDownloadWorker) downloadSingleAttachment(
    task *DownloadTask,
) error
```

### 4.2 DownloadTask

```go
// DownloadTask represents a single attachment download operation
type DownloadTask struct {
    AttachmentID  int64
    ItemID        int64
    InstanceID    int64
    FileName      string
    URL           string
    LarkToken     string
    
    AttemptCount  int
    LastAttempt   time.Time
    NextRetry     time.Time
    CreatedAt     time.Time
}

// IsRetryable returns true if more retries are available
func (t *DownloadTask) IsRetryable(maxAttempts int) bool

// CanRetry returns true if enough time has passed since last attempt
func (t *DownloadTask) CanRetry(strategy *RetryStrategy) bool
```

### 4.3 RetryStrategy

```go
// RetryStrategy defines exponential backoff retry logic
type RetryStrategy struct {
    MaxAttempts int           // Default: 3
    BaseBackoff time.Duration // Default: 1 second
    MaxBackoff  time.Duration // Default: 8 seconds
    Jitter      bool          // Enable jitter (default: true)
}

// CalculateBackoff returns duration until next retry attempt
func (s *RetryStrategy) CalculateBackoff(attemptNumber int) time.Duration

// IsTemporaryError determines if error is retryable
func (s *RetryStrategy) IsTemporaryError(err error) bool

// IsRetryableStatusCode determines if HTTP status warrants retry
func (s *RetryStrategy) IsRetryableStatusCode(statusCode int) bool
```

### 4.4 WorkerStatus

```go
// WorkerStatus reports current worker health
type WorkerStatus struct {
    IsRunning          bool
    LastProcessed      time.Time
    PendingCount       int
    ProcessedCount     int
    FailedCount        int
    UpSinceDuration    time.Duration
    IsHealthy          bool
    LastError          error
}
```

### 4.5 Enhanced AttachmentRepository

**New Methods**:
```go
// GetPendingAttachments retrieves attachments with PENDING status
func (r *AttachmentRepository) GetPendingAttachments(limit int) (
    []*models.Attachment,
    error,
)

// GetFailedAttachments retrieves attachments that failed downloads
func (r *AttachmentRepository) GetFailedAttachments(limit int) (
    []*models.Attachment,
    error,
)
```

---

## 5. Data Flow

### Download Lifecycle

```
1. WEBHOOK: Instance Created
   ↓
2. Extract attachment URLs from form
   ↓
3. Create Attachment records with status = PENDING
   ↓
4. Return immediately (webhook completes in <1s)
   ↓
5. [BACKGROUND] Worker polls GetPendingAttachments()
   ↓
6. For each attachment:
   - Create DownloadTask
   - Call DownloadAttachmentWithRetry()
   - Update status → DOWNLOADED (success)
   - OR status → FAILED + error_message (3 attempts exhausted)
   ↓
7. Voucher generation queries DOWNLOADED attachments
```

### Status Transitions

```
PENDING
  ↓
  ├─ [Success] → DOWNLOADED (downloaded_at = NOW)
  │
  └─ [Retry] → PENDING (nextRetry = NOW + backoff)
                ↓
                [Max attempts exceeded] → FAILED (error_message)
```

---

## 6. Configuration & Runtime

### Environment Variables
```yaml
ATTACHMENT_DOWNLOAD_POLL_INTERVAL=5s      # Polling frequency
ATTACHMENT_DOWNLOAD_BATCH_SIZE=10         # Max per batch
ATTACHMENT_DOWNLOAD_MAX_RETRIES=3         # Max retry attempts
ATTACHMENT_DOWNLOAD_TIMEOUT=30s           # Per-file timeout
```

### Graceful Shutdown Flow
```
1. HTTP server signals shutdown
2. Worker receives context cancellation
3. Complete current batch processing
4. Close connections
5. Exit without killing in-flight downloads
```

---

## 7. Invariants & Constraints

### Thread Safety
- **Worker isolation**: Single worker processes sequentially
- **Database transactions**: All updates use transactions
- **No race conditions**: PENDING status prevents duplicate processing

### Error Handling
- **Permanent errors** (404, 401): Skip retry, mark FAILED
- **Transient errors** (5xx, timeouts): Retry with backoff
- **Network failures**: Retry with exponential backoff
- **Missing attachments**: Gracefully skip and log

### Scalability
- **Batch processing**: Max 10 attachments per poll cycle
- **Connection pooling**: HTTP client reused across downloads
- **Memory efficiency**: Stream large files (not buffering all)
- **Audit trail**: Every transition logged for compliance

---

## 8. Risks & Assumptions

### Risks
1. **Lark API Rate Limiting**: No explicit rate limiting handling yet
   - *Mitigation*: Batch size of 10, polling interval of 5s
   
2. **Storage Capacity**: Downloaded files accumulate on disk
   - *Mitigation*: Archive/cleanup logic (Phase 4/5)
   
3. **Stuck Worker**: Worker may hang if context not properly canceled
   - *Mitigation*: Health check endpoint, timeout monitoring

### Assumptions
1. Lark Drive API supports Bearer token authentication
2. Attachment URLs remain valid for ≥5 minutes
3. Disk storage available for attachment files
4. Database supports concurrent transactions
5. Context cancellation properly propagates through goroutines

---

## 9. Integration Points

### Existing Components
- **AttachmentRepository**: Uses GetPendingAttachments() [ENHANCED]
- **AttachmentHandler**: Uses DownloadAttachmentWithRetry() [EXISTING]
- **Lark Client**: Obtains Bearer token for download auth
- **Workflow Engine**: No changes to HandleInstanceCreated flow

### Future Integration (Phase 5)
- **Voucher Generator**: Will consume DOWNLOADED attachments
- **Email Service**: Alert on download failures
- **Monitoring**: Prometheus metrics for download performance

---

## 10. Sign-Off

| Role | Deliverables | Status |
|------|--------------|--------|
| **ARCHITECT** | <ul><li>5 architectural requirements (ARCH-007-A through E)</li><li>4 class/interface definitions</li><li>Data flow diagrams</li><li>Configuration & constraints</li></ul> | ✅ COMPLETE |

---

**Next Phase**: Test Engineer designs comprehensive test suite mapping to ARCH-007-A through E.
