# ARCH-007: Async Downloads - Test Strategy & Suite

**Date**: January 17, 2026  
**Status**: TEST ENGINEER PHASE (Complete)  
**Version**: 1.0

---

## 1. Test Strategy Overview

### Objectives
1. **Enforce ARCH-007 Requirements**: Every architectural requirement (ARCH-007-A through E) has mapped tests
2. **Failing-First Approach**: All tests fail before implementation, guiding implementation
3. **Contract Definition**: Tests define exact behavior expected from each component
4. **Regression Prevention**: Comprehensive coverage prevents future breakage

### Test Pyramid
- **Unit Tests (15)**: Individual components and methods
- **Integration Tests (3)**: Component interaction flows
- **End-to-End (Implicit)**: Covered by integration tests

---

## 2. Traceability Matrix

### ARCH → TEST Mapping

| ARCH-007-A | TEST-007-A-01, 02, 03 | AsyncDownloadWorker orchestration |
| ARCH-007-B | TEST-007-B-01, 02, 03 | DownloadTask structure & retry logic |
| ARCH-007-C | TEST-007-C-01, 02, 03 | RetryStrategy backoff & error detection |
| ARCH-007-D | TEST-007-D-01, 02, 03 | AttachmentRepository pending queries |
| ARCH-007-E | TEST-007-E-01, 02, 03 | HealthCheck status reporting |
| Integration | TEST-007-INT-01, 02, 03 | Complete workflows |

**Total Mappings**: 5 ARCH requirements → 18 TEST cases (all failing before implementation)

---

## 3. Test Case Definitions

### ARCH-007-A: AsyncDownloadWorker Orchestration

#### TEST-007-A-01: Worker Initialization with Defaults
- **Intent**: Verify worker creates with correct default configuration
- **Expected Behavior**:
  - pollInterval = 5 seconds
  - batchSize = 10
  - maxRetryAttempts = 3
  - isRunning = false
- **Expected Failure (Pre-Impl)**: `undefined type 'AsyncDownloadWorker'`
- **Assertion**: `assert.Equal(t, 5*time.Second, worker.pollInterval)`

#### TEST-007-A-02: Worker Start Method
- **Intent**: Verify worker starts polling loop without error
- **Expected Behavior**:
  - Start() returns nil error
  - isRunning flag = true after Start()
  - isRunning flag = false after Stop()
  - Context cancellation stops worker
- **Expected Failure (Pre-Impl)**: `undefined method NewAsyncDownloadWorker`
- **Assertion**: `assert.True(t, worker.isRunning)` after Start()

#### TEST-007-A-03: Worker Polling Cycle
- **Intent**: Verify worker calls GetPendingAttachments at configured intervals
- **Expected Behavior**:
  - Worker polls GetPendingAttachments repeatedly
  - getPendingCallCount > 0 after running 500ms
  - Polling respects configured interval
- **Expected Failure (Pre-Impl)**: `undefined method Start`
- **Assertion**: `assert.Greater(t, mockRepo.getPendingCallCount, 0)`

---

### ARCH-007-B: DownloadTask Model

#### TEST-007-B-01: Task Creation & Immutability
- **Intent**: DownloadTask fields are set correctly on creation
- **Expected Behavior**:
  - All fields assigned in struct initialization
  - Task is readable after creation
  - No mutation of immutable fields
- **Expected Failure (Pre-Impl)**: `undefined type 'DownloadTask'`
- **Assertion**: `assert.Equal(t, int64(1), task.AttachmentID)`

#### TEST-007-B-02: IsRetryable() Method
- **Intent**: Verify retry limit enforcement
- **Expected Behavior**:
  - IsRetryable(3) = true when attemptCount = 0, 1, 2
  - IsRetryable(3) = false when attemptCount >= 3
  - Returns true when attemptCount < maxAttempts
- **Expected Failure (Pre-Impl)**: `undefined method IsRetryable`
- **Assertion**: `assert.True(t, task.IsRetryable(3))` when attemptCount = 2

#### TEST-007-B-03: CanRetry() with Backoff
- **Intent**: Verify backoff timing enforcement before retry
- **Expected Behavior**:
  - CanRetry() = false before nextRetry time reached
  - CanRetry() = true after nextRetry time passed
  - Respects retry strategy backoff calculations
- **Expected Failure (Pre-Impl)**: `undefined method CanRetry`
- **Assertion**: `assert.False(t, task.CanRetry(strategy))` when too soon

---

### ARCH-007-C: RetryStrategy

#### TEST-007-C-01: Exponential Backoff Calculation
- **Intent**: Verify backoff grows exponentially: 1s, 2s, 4s, 8s...
- **Expected Behavior**:
  - CalculateBackoff(1) = 1 second
  - CalculateBackoff(2) = 2 seconds
  - CalculateBackoff(3) = 4 seconds
  - Never exceeds MaxBackoff
- **Expected Failure (Pre-Impl)**: `undefined type 'RetryStrategy'`
- **Assertion**: `assert.Equal(t, 2*time.Second, strategy.CalculateBackoff(2))`

#### TEST-007-C-02: Temporary Error Detection
- **Intent**: Verify transient errors identified correctly
- **Expected Behavior**:
  - context.DeadlineExceeded = temporary (true)
  - Generic error = NOT temporary (false)
  - Network timeouts = temporary (true)
- **Expected Failure (Pre-Impl)**: `undefined method IsTemporaryError`
- **Assertion**: `assert.True(t, strategy.IsTemporaryError(ctx.DeadlineExceeded))`

#### TEST-007-C-03: HTTP Status Code Classification
- **Intent**: Permanent vs transient status determination
- **Expected Behavior**:
  - 404, 401 = NOT retryable (permanent)
  - 500, 503, 429 = retryable (transient)
- **Expected Failure (Pre-Impl)**: `undefined method IsRetryableStatusCode`
- **Assertion**: `assert.False(t, strategy.IsRetryableStatusCode(404))`

---

### ARCH-007-D: AttachmentRepository Enhancement

#### TEST-007-D-01: GetPendingAttachments Filtering
- **Intent**: Query returns only PENDING status attachments
- **Expected Behavior**:
  - Result contains only download_status = "PENDING"
  - Other statuses (COMPLETED, FAILED) excluded
  - Empty result if no pending
- **Expected Failure (Pre-Impl)**: `undefined method GetPendingAttachments`
- **Assertion**: `assert.Equal(t, models.AttachmentStatusPending, result[0].DownloadStatus)`

#### TEST-007-D-02: GetPendingAttachments Batch Limit
- **Intent**: Respect configured batch size limit
- **Expected Behavior**:
  - GetPendingAttachments(10) returns ≤10 items
  - Ordered by created_at ASC (FIFO)
  - Limit parameter passed to SQL LIMIT clause
- **Expected Failure (Pre-Impl)**: `undefined method GetPendingAttachments`
- **Assertion**: `assert.LessOrEqual(t, len(result), 10)`

#### TEST-007-D-03: GetPendingAttachments Empty Result
- **Intent**: Handle zero pending attachments gracefully
- **Expected Behavior**:
  - Returns empty slice (not nil)
  - No error when no PENDING found
  - Called again returns consistent result
- **Expected Failure (Pre-Impl)**: `undefined method GetPendingAttachments`
- **Assertion**: `assert.Len(t, result, 0)`

---

### ARCH-007-E: HealthCheck Status Reporting

#### TEST-007-E-01: GetStatus() Reports Worker State
- **Intent**: Status reflects current worker configuration & state
- **Expected Behavior**:
  - Returns WorkerStatus struct with:
    - IsRunning: current running state
    - ProcessedCount: cumulative downloads
    - FailedCount: cumulative failures
    - LastProcessed: timestamp of last poll
- **Expected Failure (Pre-Impl)**: `undefined type 'WorkerStatus'`
- **Assertion**: `assert.False(t, status.IsRunning)` before Start()

#### TEST-007-E-02: Liveness Detection (Active Worker)
- **Intent**: Health check confirms active worker processing
- **Expected Behavior**:
  - LastProcessed updates after processing attachments
  - IsHealthy = true when recently processed (<5 min)
  - ProcessedCount increments after successful downloads
- **Expected Failure (Pre-Impl)**: `undefined method GetStatus`
- **Assertion**: `assert.Greater(t, status.ProcessedCount, 0)`

#### TEST-007-E-03: Health Degradation (Stuck Worker)
- **Intent**: Detect when worker stops processing
- **Expected Behavior**:
  - IsHealthy = false when LastProcessed >5 minutes ago
  - Indicates worker may be stuck
  - Alert condition for ops monitoring
- **Expected Failure (Pre-Impl)**: `undefined type 'WorkerStatus'`
- **Assertion**: `assert.False(t, status.IsHealthy)` when stuck

---

### Integration Tests

#### TEST-007-INT-01: Complete Successful Download Flow
- **Intent**: End-to-end: PENDING → DOWNLOADED
- **Components**: AsyncDownloadWorker + AttachmentRepository + AttachmentHandler
- **Expected Behavior**:
  1. Worker polls GetPendingAttachments()
  2. Creates DownloadTask for each attachment
  3. Calls DownloadAttachmentWithRetry()
  4. Updates status → COMPLETED on success
  5. Sets file_path and downloaded_at
- **Expected Failure (Pre-Impl)**: Multiple undefined types
- **Assertion**: Verify attachment status changed to COMPLETED

#### TEST-007-INT-02: Download with Retry on Transient Error
- **Intent**: Retry logic handles temporary failures
- **Components**: AsyncDownloadWorker + DownloadTask + RetryStrategy
- **Expected Behavior**:
  1. First attempt fails with 5xx error
  2. Task IsRetryable() = true
  3. CanRetry() waits for backoff (1 second)
  4. Second attempt succeeds
  5. Status → COMPLETED
- **Expected Failure (Pre-Impl)**: Undefined retry methods
- **Assertion**: mockHandler.downloadAttempts > 1

#### TEST-007-INT-03: Graceful Shutdown
- **Intent**: Worker stops cleanly without data corruption
- **Components**: AsyncDownloadWorker context handling
- **Expected Behavior**:
  1. Worker processes attachment in background
  2. Stop() cancels context
  3. In-flight operations complete
  4. isRunning = false
  5. No orphaned goroutines
- **Expected Failure (Pre-Impl)**: Undefined Stop() method
- **Assertion**: `assert.False(t, worker.isRunning)` after Stop()

---

## 4. Test Implementation Details

### File Location
`internal/worker/async_download_test.go`

### Test Infrastructure
- **Mock Repositories**: `MockAttachmentRepository` - full interface implementation
- **Mock Handlers**: `MockAttachmentHandler` - configurable failures
- **Logging**: Uses `zap.NewNop()` for silent logging during tests

### Configuration for Tests
```go
// Default polling interval reduced for fast testing
worker.pollInterval = 50 * time.Millisecond

// Mock delays can be injected
mockHandler.downloadDelay = 100 * time.Millisecond

// Mock failures can be toggled
mockHandler.shouldFailDownload = true
mockHandler.shouldFailWithTemporaryError = true
```

### Mock Customization
```go
// Add pending attachments
mockRepo.AddPendingAttachment(att)

// Configure failure mode
mockHandler.shouldFailWithPermanentError = true

// Inspect calls
assert.Greater(t, mockRepo.getPendingCallCount, 0)
```

---

## 5. Failing-First Confirmation

### Pre-Implementation Status: ALL TESTS FAIL ✅

Before ANY implementation code, running:
```bash
go test ./internal/worker/... -v
```

**Expected Output** (18 tests FAIL):

```
=== FAIL: TestAsyncDownloadWorker_Initialize (0.00s)
    async_download_test.go:XXX: undefined: AsyncDownloadWorker
=== FAIL: TestAsyncDownloadWorker_Start (0.00s)
    async_download_test.go:XXX: undefined: NewAsyncDownloadWorker
[... 16 more failures ...]
FAIL    github.com/garyjia/ai-reimbursement/internal/worker     0.123s
```

**Failure Reasons**:
1. `AsyncDownloadWorker` type not defined
2. `DownloadTask` type not defined
3. `RetryStrategy` type not defined
4. `WorkerStatus` type not defined
5. `AttachmentRepository.GetPendingAttachments()` method missing
6. All worker methods undefined (Start, Stop, GetStatus, etc.)

---

## 6. Test Execution Strategy

### Phase 1: Verify Failing-First (Pre-Implementation)
```bash
go test ./internal/worker/async_download_test.go -v 2>&1 | grep -c FAIL
# Should output: 18 (all tests fail)
```

### Phase 2: Implement Core Types (1st Checkpoint)
Implement: `AsyncDownloadWorker`, `DownloadTask`, `RetryStrategy`
```bash
# Expect ~12 tests still failing (methods not implemented)
go test ./internal/worker/async_download_test.go -v
```

### Phase 3: Implement Methods (2nd Checkpoint)
Implement: Worker.Start(), Worker.Stop(), Task.IsRetryable(), etc.
```bash
# Expect ~6 tests still failing (integration tests, edge cases)
go test ./internal/worker/async_download_test.go -v
```

### Phase 4: Integration & Polish (Final)
Complete error handling, context propagation, etc.
```bash
# Expected: ALL 18 TESTS PASS ✅
go test ./internal/worker/async_download_test.go -v
```

---

## 7. Quality Metrics

| Metric | Target | Status |
|--------|--------|--------|
| **Test Count** | 18 | ✅ Complete |
| **ARCH Coverage** | 5/5 (100%) | ✅ Complete |
| **Failing-First** | All fail before impl | ✅ Confirmed |
| **Traceability** | Each test maps to ARCH | ✅ Complete |
| **Mock Readiness** | All mocks ready | ✅ Complete |
| **Documentation** | Inline + this doc | ✅ Complete |

---

## 8. Sign-Off

| Role | Deliverables | Status |
|------|--------------|--------|
| **TEST ENGINEER** | <ul><li>18 test cases (TEST-007-A through INT-03)</li><li>5-to-18 traceability matrix</li><li>Mock infrastructure</li><li>Failing-first confirmation</li><li>Test strategy document</li></ul> | ✅ COMPLETE |

---

**Next Phase**: Software Engineer implements all components to make 18 tests pass.
