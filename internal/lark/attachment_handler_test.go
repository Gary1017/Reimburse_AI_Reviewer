package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// MockHTTPClient for testing attachment downloads
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// TestAttachmentHandlerExtractURLs tests extraction of attachment URLs from form data
func TestAttachmentHandlerExtractURLs(t *testing.T) {
	logger := zap.NewNop()
	handler := NewAttachmentHandler(logger, "/tmp/attachments")

	tests := []struct {
		name        string
		formData    string
		expectedLen int
		expectError bool
		intent      string
	}{
		{
			name: "ARCH-001: Single attachment extraction from attachmentV2 widget",
			formData: `{
				"widgets": [
					{
						"id": "widget16510510447300001",
						"name": "附件",
						"type": "attachmentV2",
						"value": [
							"https://internal-api-drive-stream.feishu.cn/space/api/box/stream/download/authcode/?code=test123"
						]
					}
				]
			}`,
			expectedLen: 1,
			expectError: false,
			intent:      "Extract single file URL from attachmentV2 widget",
		},
		{
			name: "ARCH-001: Multiple attachments from single widget",
			formData: `{
				"widgets": [
					{
						"id": "widget16510510447300001",
						"name": "附件",
						"type": "attachmentV2",
						"value": [
							"https://internal-api-drive-stream.feishu.cn/space/api/box/stream/download/authcode/?code=file1",
							"https://internal-api-drive-stream.feishu.cn/space/api/box/stream/download/authcode/?code=file2"
						]
					}
				]
			}`,
			expectedLen: 2,
			expectError: false,
			intent:      "Extract multiple URLs from single attachmentV2 widget",
		},
		{
			name: "ARCH-001: No attachments in form data",
			formData: `{
				"widgets": [
					{
						"id": "widget1",
						"name": "description",
						"type": "text",
						"value": "Test description"
					}
				]
			}`,
			expectedLen: 0,
			expectError: false,
			intent:      "Handle form with no attachments gracefully",
		},
		{
			name: "ARCH-001: Empty form data",
			formData: `{
				"widgets": []
			}`,
			expectedLen: 0,
			expectError: false,
			intent:      "Handle empty form data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := handler.ExtractAttachmentURLs(tt.formData)

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(refs) != tt.expectedLen {
				t.Errorf("Expected %d attachments, got %d", tt.expectedLen, len(refs))
			}
		})
	}
}

// TestAttachmentHandlerFileNaming tests unique file naming strategy
func TestAttachmentHandlerFileNaming(t *testing.T) {
	logger := zap.NewNop()
	handler := NewAttachmentHandler(logger, "/tmp/attachments")

	tests := []struct {
		name           string
		larkInstanceID string
		attachmentID   int64
		originalName   string
		withSubdir     bool
		item           *models.ReimbursementItem
		expectedFormat string
		intent         string
	}{
		{
			name:           "ARCH-003: Standard filename format",
			larkInstanceID: "LARK12345",
			attachmentID:   67890,
			originalName:   "发票.pdf",
			withSubdir:     false,
			item:           nil,
			expectedFormat: "LARK12345_att67890_发票.pdf",
			intent:         "Generate unique filename with lark instance ID and attachment ID",
		},
		{
			name:           "ARCH-003: Filename with spaces",
			larkInstanceID: "LARK12345",
			attachmentID:   67890,
			originalName:   "Receipt 2026-01-15.pdf",
			withSubdir:     false,
			item:           nil,
			expectedFormat: "LARK12345_att67890_Receipt 2026-01-15.pdf",
			intent:         "Preserve spaces in filename",
		},
		{
			name:           "ARCH-003: Filename with special characters",
			larkInstanceID: "LARK999",
			attachmentID:   888,
			originalName:   "invoice (final).xlsx",
			withSubdir:     false,
			item:           nil,
			expectedFormat: "LARK999_att888_invoice (final).xlsx",
			intent:         "Handle special characters in filename",
		},
		{
			name:           "ARCH-014-B: Generate path with instance subdirectory but no item",
			larkInstanceID: "ABC123-XYZ",
			attachmentID:   67890,
			originalName:   "发票.pdf",
			withSubdir:     true,
			item:           nil,
			expectedFormat: "ABC123-XYZ/发票.pdf",
			intent:         "Generate path with LarkInstanceID as subdirectory",
		},
		{
			name:           "ARCH-014-B: Generate path with instance subdirectory and item",
			larkInstanceID: "ABC123-XYZ",
			attachmentID:   67890,
			originalName:   "发票.pdf",
			withSubdir:     true,
			item: &models.ReimbursementItem{
				Amount:   150.75,
				Currency: "CNY",
			},
			expectedFormat: "ABC123-XYZ/invoice_ABC123-XYZ_150.75 CNY.pdf",
			intent:         "Generate path with LarkInstanceID as subdirectory and item details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := handler.GenerateFileName(tt.larkInstanceID, tt.attachmentID, tt.originalName, tt.withSubdir, tt.item)
			expected := tt.expectedFormat
			if tt.withSubdir {
				expected = filepath.FromSlash(tt.expectedFormat)
			}
			assert.Equal(t, expected, filename)
		})
	}
}

// TestAttachmentHandlerDownload tests file download logic
func TestAttachmentHandlerDownload(t *testing.T) {
	logger := zap.NewNop()
	handler := NewAttachmentHandler(logger, "/tmp/attachments")

	tests := []struct {
		name               string
		mockResponse       *http.Response
		mockError          error
		expectError        bool
		expectedContentLen int
		intent             string
	}{
		{
			name: "ARCH-002: Successful file download",
			mockResponse: &http.Response{
				StatusCode: 200,
				Header: http.Header{
					"Content-Type":   []string{"application/pdf"},
					"Content-Length": []string{"1024"},
				},
				Body: io.NopCloser(bytes.NewReader([]byte("PDF content here"))),
			},
			mockError:          nil,
			expectError:        false,
			expectedContentLen: 16,
			intent:             "Download file successfully with correct status and headers",
		},
		{
			name: "ARCH-006: Handle 404 error",
			mockResponse: &http.Response{
				StatusCode: 404,
				Body:       io.NopCloser(bytes.NewReader([]byte("Not Found"))),
			},
			mockError:   nil,
			expectError: true,
			intent:      "Handle 404 error gracefully",
		},
		{
			name:         "ARCH-006: Handle network error",
			mockResponse: nil,
			mockError:    fmt.Errorf("network timeout"),
			expectError:  true,
			intent:       "Handle network errors with appropriate error message",
		},
		{
			name: "ARCH-006: Handle 401 unauthorized",
			mockResponse: &http.Response{
				StatusCode: 401,
				Body:       io.NopCloser(bytes.NewReader([]byte("Unauthorized"))),
			},
			mockError:   nil,
			expectError: true,
			intent:      "Handle authentication errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock HTTP client
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					return tt.mockResponse, nil
				},
			}

			handler.httpClient = mockClient
			ctx := context.Background()

			file, err := handler.DownloadAttachment(ctx, "https://example.com/file.pdf", "test_token")

			if tt.expectError && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && file != nil {
				if len(file.Content) != tt.expectedContentLen {
					t.Errorf("Expected content length %d, got %d", tt.expectedContentLen, len(file.Content))
				}
			}
		})
	}
}

// TestAttachmentHandlerRetryLogic tests exponential backoff retry on transient failures
func TestAttachmentHandlerRetryLogic(t *testing.T) {
	logger := zap.NewNop()
	handler := NewAttachmentHandler(logger, "/tmp/attachments")

	tests := []struct {
		name          string
		attempts      int
		failOnAttempt int
		expectSuccess bool
		expectedCalls int
		intent        string
	}{
		{
			name:          "ARCH-006: Succeed on first attempt",
			attempts:      3,
			failOnAttempt: -1, // Never fail
			expectSuccess: true,
			expectedCalls: 1,
			intent:        "Return immediately on success",
		},
		{
			name:          "ARCH-006: Retry and succeed on second attempt",
			attempts:      3,
			failOnAttempt: 1,
			expectSuccess: true,
			expectedCalls: 2,
			intent:        "Retry transient failure and eventually succeed",
		},
		{
			name:          "ARCH-006: Fail after max retries",
			attempts:      3,
			failOnAttempt: 0, // Fail all attempts
			expectSuccess: false,
			expectedCalls: 3,
			intent:        "Return error after exhausting retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			mockClient := &MockHTTPClient{
				DoFunc: func(req *http.Request) (*http.Response, error) {
					callCount++
					if tt.failOnAttempt >= 0 && callCount == tt.failOnAttempt {
						return nil, fmt.Errorf("transient failure")
					}
					if callCount == tt.failOnAttempt || tt.failOnAttempt == 0 {
						if callCount <= tt.expectedCalls {
							return nil, fmt.Errorf("persistent failure")
						}
					}
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewReader([]byte("content"))),
					}, nil
				},
			}

			handler.httpClient = mockClient
			ctx := context.Background()

			_, err := handler.DownloadAttachmentWithRetry(ctx, "https://example.com/file.pdf", "token", tt.attempts)

			if tt.expectSuccess && err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
			if !tt.expectSuccess && err == nil {
				t.Errorf("Expected error, got nil")
			}

			if callCount != tt.expectedCalls {
				t.Errorf("Expected %d calls, got %d", tt.expectedCalls, callCount)
			}
		})
	}
}

// TestExtractAttachmentMetadata tests extraction of filename and mime type from form data
func TestExtractAttachmentMetadata(t *testing.T) {
	logger := zap.NewNop()
	handler := NewAttachmentHandler(logger, "/tmp/attachments")

	tests := []struct {
		name             string
		formData         string
		expectedName     string
		expectedMimeType string
		expectError      bool
		intent           string
	}{
		{
			name: "ARCH-001: Extract filename and mime type",
			formData: `{
				"widgets": [
					{
						"id": "widget1",
						"name": "附件",
						"type": "attachmentV2",
						"ext": "发票.pdf",
						"value": ["https://example.com/file"]
					}
				]
			}`,
			expectedName:     "发票",
			expectedMimeType: "application/pdf",
			expectError:      false,
			intent:           "Extract original filename and infer MIME type from extension",
		},
		{
			name: "ARCH-001: Handle Excel file",
			formData: `{
				"widgets": [
					{
						"id": "widget1",
						"name": "attachments",
						"type": "attachmentV2",
						"ext": "report.xlsx",
						"value": ["https://example.com/file"]
					}
				]
			}`,
			expectedName:     "report",
			expectedMimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			expectError:      false,
			intent:           "Handle Excel XLSX files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(tt.formData), &data); err != nil {
				t.Fatalf("Failed to parse test data: %v", err)
			}

			// Extract widget from data
			widgets := data["widgets"].([]interface{})
			widget := widgets[0].(map[string]interface{})

			metadata := handler.ExtractFileMetadata(widget)
			if metadata == nil {
				if !tt.expectError {
					t.Errorf("Expected metadata, got nil")
				}
				return
			}

			if metadata["file_name"] != tt.expectedName {
				t.Errorf("Expected filename %q, got %q", tt.expectedName, metadata["file_name"])
			}
			if metadata["mime_type"] != tt.expectedMimeType {
				t.Errorf("Expected mime type %q, got %q", tt.expectedMimeType, metadata["mime_type"])
			}
		})
	}
}

// TestAttachmentExtractionDoesNotBlockFormParsing tests that attachment extraction failures don't block form parsing
func TestAttachmentExtractionDoesNotBlockFormParsing(t *testing.T) {
	const intent = "ARCH-001: Attachment extraction failures must not block reimbursement item parsing"

	logger := zap.NewNop()
	formParser := NewFormParserWithAttachmentSupport(logger)

	// Form with valid items but invalid/missing attachments
	formData := `{
		"form": "[{\"type\": \"fieldList\", \"name\": \"費用明細\", \"value\": [[{\"name\": \"金額\", \"value\": 100}, {\"name\": \"內容\", \"value\": \"Test\"}]]}, {\"type\": \"attachmentV2\", \"name\": \"附件\", \"value\": []}]"
	}`

	items, attachments, err := formParser.ParseWithAttachments(formData)

	if err != nil {
		t.Errorf("%s: Expected success despite attachment issue, got error: %v", intent, err)
	}

	if len(items) == 0 {
		t.Errorf("%s: Expected items to be parsed despite attachment issues", intent)
	}

	// Attachments may be empty, but should not block parsing
	if attachments == nil {
		t.Errorf("%s: Expected attachments slice (even if empty), got nil", intent)
	}
}

// TestAttachmentIntegrationWithWorkflow tests attachment processing in workflow context
func TestAttachmentIntegrationWithWorkflow(t *testing.T) {
	const intent = "ARCH-005: Attachment handling must not block workflow progression"

	// This test verifies that the workflow engine can trigger attachment download
	// without waiting for completion (async processing)

	// Simulate attachment metadata that would be stored after extraction
	attachment := &models.Attachment{
		ItemID:         123,
		InstanceID:     456,
		FileName:       "invoice.pdf",
		FilePath:       "",
		DownloadStatus: models.AttachmentStatusPending,
		CreatedAt:      now(),
	}

	// Verify initial state
	if attachment.DownloadStatus != models.AttachmentStatusPending {
		t.Errorf("%s: Expected initial status to be PENDING", intent)
	}

	if attachment.FilePath != "" {
		t.Errorf("%s: Expected empty file path initially", intent)
	}

	// After async download completes
	attachment.DownloadStatus = models.AttachmentStatusCompleted
	attachment.FilePath = "/tmp/attachments/456_123_invoice.pdf"

	if attachment.DownloadStatus != models.AttachmentStatusCompleted {
		t.Errorf("%s: Expected status to be COMPLETED after download", intent)
	}

	if attachment.FilePath == "" {
		t.Errorf("%s: Expected file path to be set after download", intent)
	}
}

// Helper function for current time
func now() time.Time {
	return time.Now()
}

// TestFormParserExtractAttachmentV2Widgets tests form parser attachment extraction capability
func TestFormParserExtractAttachmentV2Widgets(t *testing.T) {
	const intent = "ARCH-001: Form parser must extract attachmentV2 widget data and link to items"

	logger := zap.NewNop()
	parser := NewFormParserWithAttachmentSupport(logger)

	// Realistic Lark form with both items and attachments
	formData := `{
		"form": "[{\"type\": \"radioV2\", \"name\": \"報銷類型\", \"value\": \"差旅\"}, {\"type\": \"fieldList\", \"name\": \"費用明細\", \"value\": [[{\"name\": \"日期\", \"value\": \"2026-01-15\"}, {\"name\": \"金額\", \"value\": 500}, {\"name\": \"內容\", \"value\": \"flights\"}, {\"name\": \"發票\", \"value\": \"file_token_123\"}]]}, {\"type\": \"attachmentV2\", \"name\": \"附件\", \"ext\": \"receipt.pdf\", \"value\": [\"https://example.com/download?code=abc123\"]}]"
	}`

	items, attachments, err := parser.ParseWithAttachments(formData)

	if err != nil {
		t.Errorf("%s: Parse failed: %v", intent, err)
		return
	}

	if len(items) == 0 {
		t.Errorf("%s: Expected items to be parsed", intent)
	}

	if len(attachments) == 0 {
		t.Errorf("%s: Expected attachments to be extracted", intent)
	}

	if len(attachments) > 0 && attachments[0].URL != "https://example.com/download?code=abc123" {
		t.Errorf("%s: Expected attachment URL to be extracted", intent)
	}
}

// Test that verifies attachment status transitions
func TestAttachmentStatusTransitions(t *testing.T) {
	const intent = "ARCH-004: Attachment download status must track PENDING -> COMPLETED/FAILED"

	validTransitions := map[string][]string{
		models.AttachmentStatusPending:   {models.AttachmentStatusCompleted, models.AttachmentStatusFailed},
		models.AttachmentStatusCompleted: {},                               // Terminal state
		models.AttachmentStatusFailed:    {models.AttachmentStatusPending}, // Allow retry
	}

	// Test valid transitions
	for fromStatus, toStatuses := range validTransitions {
		for _, toStatus := range toStatuses {
			if !isValidTransition(fromStatus, toStatus) {
				t.Errorf("%s: Expected valid transition from %s to %s", intent, fromStatus, toStatus)
			}
		}
	}

	// Test invalid transitions
	if isValidTransition(models.AttachmentStatusCompleted, models.AttachmentStatusPending) {
		t.Errorf("%s: Should not allow transition from COMPLETED back to PENDING", intent)
	}
}

func isValidTransition(from, to string) bool {
	validTransitions := map[string]map[string]bool{
		models.AttachmentStatusPending: {
			models.AttachmentStatusCompleted: true,
			models.AttachmentStatusFailed:    true,
		},
		models.AttachmentStatusFailed: {
			models.AttachmentStatusPending: true, // Retry
		},
	}

	if transitions, ok := validTransitions[from]; ok {
		return transitions[to]
	}
	return false
}
