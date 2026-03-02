package worker

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
	"go.uber.org/zap"
)

const workerFixtureDir = "../../../testdata/lark_fixtures"

// loadExtractionFixture loads the GPT-4 extraction result fixture
func loadExtractionFixture(t *testing.T) *port.InvoiceExtractionResult {
	t.Helper()
	data, err := os.ReadFile(workerFixtureDir + "/20260217_172016_01_invoice_extraction.json")
	if err != nil {
		t.Fatalf("Failed to read extraction fixture: %v", err)
	}

	var result port.InvoiceExtractionResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse extraction fixture: %v", err)
	}
	return &result
}

// --- Mocks ---

type mockWorkerAttachmentRepo struct {
	statusUpdates   []struct{ ID int64; Status string; ErrorMsg string }
	fileTypeUpdates []struct{ ID int64; FileType string }
	attachments     []*entity.Attachment // for GetByInstanceID
	countResult     int
}

func (m *mockWorkerAttachmentRepo) Create(ctx context.Context, att *entity.Attachment) error {
	return nil
}

func (m *mockWorkerAttachmentRepo) GetByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	return nil, nil
}

func (m *mockWorkerAttachmentRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
	return m.attachments, nil
}

func (m *mockWorkerAttachmentRepo) GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockWorkerAttachmentRepo) MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error {
	return nil
}

func (m *mockWorkerAttachmentRepo) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	m.statusUpdates = append(m.statusUpdates, struct{ ID int64; Status string; ErrorMsg string }{id, status, errorMsg})
	return nil
}

func (m *mockWorkerAttachmentRepo) GetCompletedUnprocessed(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockWorkerAttachmentRepo) UpdateItemID(ctx context.Context, attachmentID int64, itemID int64) error {
	return nil
}

func (m *mockWorkerAttachmentRepo) UpdateFileType(ctx context.Context, attachmentID int64, fileType string) error {
	m.fileTypeUpdates = append(m.fileTypeUpdates, struct{ ID int64; FileType string }{attachmentID, fileType})
	return nil
}

func (m *mockWorkerAttachmentRepo) GetByInstanceIDAndStatus(ctx context.Context, instanceID int64, status string) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockWorkerAttachmentRepo) CountByInstanceIDAndStatuses(ctx context.Context, instanceID int64, statuses []string) (int, error) {
	return m.countResult, nil
}

type mockWorkerItemRepo struct{}

func (m *mockWorkerItemRepo) Create(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}
func (m *mockWorkerItemRepo) GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error) {
	return nil, nil
}
func (m *mockWorkerItemRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
	return nil, nil
}
func (m *mockWorkerItemRepo) Update(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}

type mockWorkerInvoiceV2Repo struct {
	createdInvoices []*entity.InvoiceV2
}

func (m *mockWorkerInvoiceV2Repo) Create(ctx context.Context, invoice *entity.InvoiceV2) error {
	invoice.ID = int64(len(m.createdInvoices) + 1)
	m.createdInvoices = append(m.createdInvoices, invoice)
	return nil
}

func (m *mockWorkerInvoiceV2Repo) GetByID(ctx context.Context, id int64) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockWorkerInvoiceV2Repo) GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockWorkerInvoiceV2Repo) GetByItemID(ctx context.Context, itemID int64) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockWorkerInvoiceV2Repo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockWorkerInvoiceV2Repo) GetByUniqueID(ctx context.Context, uniqueID string) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockWorkerInvoiceV2Repo) GetTotalsByInstanceID(ctx context.Context, instanceID int64) (*port.InvoiceTotals, error) {
	return nil, nil
}
func (m *mockWorkerInvoiceV2Repo) Update(ctx context.Context, invoice *entity.InvoiceV2) error {
	return nil
}

type mockWorkerFileStorage struct {
	content []byte
	err     error
}

func (m *mockWorkerFileStorage) Save(ctx context.Context, path string, data []byte) error {
	return nil
}
func (m *mockWorkerFileStorage) Read(ctx context.Context, path string) ([]byte, error) {
	return m.content, m.err
}
func (m *mockWorkerFileStorage) Exists(ctx context.Context, path string) bool {
	return true
}
func (m *mockWorkerFileStorage) Delete(ctx context.Context, path string) error {
	return nil
}
func (m *mockWorkerFileStorage) GetFullPath(relativePath string) string {
	return relativePath
}

type mockWorkerAIAuditor struct {
	extractResult *port.InvoiceExtractionResult
	extractErr    error
}

func (m *mockWorkerAIAuditor) AuditPolicy(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PolicyAuditResult, error) {
	return nil, nil
}
func (m *mockWorkerAIAuditor) AuditPrice(ctx context.Context, item *entity.ReimbursementItem, invoiceData *port.InvoiceExtractionResult) (*port.PriceAuditResult, error) {
	return nil, nil
}
func (m *mockWorkerAIAuditor) ExtractInvoice(ctx context.Context, imageData []byte, mimeType string) (*port.InvoiceExtractionResult, error) {
	return m.extractResult, m.extractErr
}

type mockWorkerDispatcher struct {
	dispatchedEvents []*event.Event
}

func (m *mockWorkerDispatcher) Subscribe(eventType event.Type, handler dispatcher.Handler) {}
func (m *mockWorkerDispatcher) SubscribeNamed(eventType event.Type, name string, handler dispatcher.Handler) {
}
func (m *mockWorkerDispatcher) Unsubscribe(eventType event.Type, name string) {}
func (m *mockWorkerDispatcher) Dispatch(ctx context.Context, evt *event.Event) error {
	m.dispatchedEvents = append(m.dispatchedEvents, evt)
	return nil
}
func (m *mockWorkerDispatcher) DispatchAsync(ctx context.Context, evt *event.Event) {
	m.dispatchedEvents = append(m.dispatchedEvents, evt)
}
func (m *mockWorkerDispatcher) ListHandlers(eventType event.Type) []dispatcher.HandlerInfo {
	return nil
}
func (m *mockWorkerDispatcher) Close() error {
	return nil
}

// --- Tests ---

func newTestInvoiceWorker(
	attachmentRepo *mockWorkerAttachmentRepo,
	invoiceV2Repo *mockWorkerInvoiceV2Repo,
	fileStorage *mockWorkerFileStorage,
	aiAuditor *mockWorkerAIAuditor,
	disp *mockWorkerDispatcher,
) *InvoiceWorker {
	logger, _ := zap.NewDevelopment()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewInvoiceWorker(
		DefaultInvoiceWorkerConfig(),
		attachmentRepo,
		&mockWorkerItemRepo{},
		invoiceV2Repo,
		fileStorage,
		aiAuditor,
		disp,
		logger,
	)
	w.ctx = ctx
	w.cancel = cancel
	return w
}

func TestInvoiceWorker_ProcessAttachment_PDFInvoice(t *testing.T) {
	extractionResult := loadExtractionFixture(t)

	attachmentRepo := &mockWorkerAttachmentRepo{}
	invoiceV2Repo := &mockWorkerInvoiceV2Repo{}
	fileStorage := &mockWorkerFileStorage{content: []byte("fake pdf content")}
	aiAuditor := &mockWorkerAIAuditor{extractResult: extractionResult}
	disp := &mockWorkerDispatcher{}

	w := newTestInvoiceWorker(attachmentRepo, invoiceV2Repo, fileStorage, aiAuditor, disp)

	att := &entity.Attachment{
		ID:             1,
		InstanceID:     100,
		FileName:       "浙江通行费电子发票1.pdf",
		FilePath:       "/tmp/test/invoice.pdf",
		DownloadStatus: entity.AttachmentStatusCompleted,
	}

	err := w.processAttachment(context.Background(), att)
	if err != nil {
		t.Fatalf("processAttachment() error = %v", err)
	}

	// Verify status transitions: first PROCESSING, then PROCESSED
	if len(attachmentRepo.statusUpdates) < 2 {
		t.Fatalf("Expected at least 2 status updates, got %d", len(attachmentRepo.statusUpdates))
	}
	if attachmentRepo.statusUpdates[0].Status != entity.AttachmentStatusProcessing {
		t.Errorf("First status update = %q, want %q", attachmentRepo.statusUpdates[0].Status, entity.AttachmentStatusProcessing)
	}
	lastUpdate := attachmentRepo.statusUpdates[len(attachmentRepo.statusUpdates)-1]
	if lastUpdate.Status != entity.AttachmentStatusProcessed {
		t.Errorf("Last status update = %q, want %q", lastUpdate.Status, entity.AttachmentStatusProcessed)
	}

	// Verify file type set to INVOICE
	foundInvoiceType := false
	for _, ft := range attachmentRepo.fileTypeUpdates {
		if ft.ID == 1 && ft.FileType == entity.FileTypeInvoice {
			foundInvoiceType = true
		}
	}
	if !foundInvoiceType {
		t.Errorf("Expected file type update to INVOICE for attachment 1, got %v", attachmentRepo.fileTypeUpdates)
	}

	// Verify InvoiceV2 created with correct data from fixture
	if len(invoiceV2Repo.createdInvoices) != 1 {
		t.Fatalf("Expected 1 InvoiceV2 created, got %d", len(invoiceV2Repo.createdInvoices))
	}
	inv := invoiceV2Repo.createdInvoices[0]
	if inv.InvoiceCode != "033002200311" {
		t.Errorf("InvoiceCode = %q, want '033002200311'", inv.InvoiceCode)
	}
	if inv.InvoiceNumber != "45677890" {
		t.Errorf("InvoiceNumber = %q, want '45677890'", inv.InvoiceNumber)
	}
	if inv.UniqueID != "033002200311-45677890" {
		t.Errorf("UniqueID = %q, want '033002200311-45677890'", inv.UniqueID)
	}
	if inv.InvoiceAmountCents != 5000 {
		t.Errorf("InvoiceAmountCents = %d, want 5000 (50.00 CNY)", inv.InvoiceAmountCents)
	}
	if inv.SellerName != "浙江省交通投资集团有限公司" {
		t.Errorf("SellerName = %q, want '浙江省交通投资集团有限公司'", inv.SellerName)
	}
	if inv.InstanceID != 100 {
		t.Errorf("InstanceID = %d, want 100", inv.InstanceID)
	}
	if inv.AttachmentID != 1 {
		t.Errorf("AttachmentID = %d, want 1", inv.AttachmentID)
	}
	if inv.InvoiceDate == nil {
		t.Error("InvoiceDate should not be nil")
	} else {
		expected := time.Date(2026, 1, 18, 0, 0, 0, 0, time.UTC)
		if !inv.InvoiceDate.Equal(expected) {
			t.Errorf("InvoiceDate = %v, want %v", inv.InvoiceDate, expected)
		}
	}
}

func TestInvoiceWorker_ProcessAttachment_NonInvoice(t *testing.T) {
	// AI returns extraction but with no invoice code/number (e.g., a receipt photo)
	noInvoiceResult := &port.InvoiceExtractionResult{
		Success:       true,
		InvoiceCode:   "",
		InvoiceNumber: "",
		TotalAmount:   100.0,
		SellerName:    "Some Store",
		Confidence:    0.8,
	}

	attachmentRepo := &mockWorkerAttachmentRepo{}
	invoiceV2Repo := &mockWorkerInvoiceV2Repo{}
	fileStorage := &mockWorkerFileStorage{content: []byte("fake jpg content")}
	aiAuditor := &mockWorkerAIAuditor{extractResult: noInvoiceResult}
	disp := &mockWorkerDispatcher{}

	w := newTestInvoiceWorker(attachmentRepo, invoiceV2Repo, fileStorage, aiAuditor, disp)

	att := &entity.Attachment{
		ID:             2,
		InstanceID:     100,
		FileName:       "receipt_photo.jpg",
		FilePath:       "/tmp/test/receipt.jpg",
		DownloadStatus: entity.AttachmentStatusCompleted,
	}

	err := w.processAttachment(context.Background(), att)
	if err != nil {
		t.Fatalf("processAttachment() error = %v", err)
	}

	// Verify file type set to OTHER
	foundOtherType := false
	for _, ft := range attachmentRepo.fileTypeUpdates {
		if ft.ID == 2 && ft.FileType == entity.FileTypeOther {
			foundOtherType = true
		}
	}
	if !foundOtherType {
		t.Errorf("Expected file type update to OTHER for attachment 2, got %v", attachmentRepo.fileTypeUpdates)
	}

	// Verify no InvoiceV2 created
	if len(invoiceV2Repo.createdInvoices) != 0 {
		t.Errorf("Expected 0 InvoiceV2 created for non-invoice, got %d", len(invoiceV2Repo.createdInvoices))
	}

	// Verify status ends at PROCESSED
	lastUpdate := attachmentRepo.statusUpdates[len(attachmentRepo.statusUpdates)-1]
	if lastUpdate.Status != entity.AttachmentStatusProcessed {
		t.Errorf("Last status = %q, want %q", lastUpdate.Status, entity.AttachmentStatusProcessed)
	}
}

func TestInvoiceWorker_ProcessAttachment_UnsupportedFileType(t *testing.T) {
	attachmentRepo := &mockWorkerAttachmentRepo{}
	invoiceV2Repo := &mockWorkerInvoiceV2Repo{}
	fileStorage := &mockWorkerFileStorage{}
	aiAuditor := &mockWorkerAIAuditor{}
	disp := &mockWorkerDispatcher{}

	w := newTestInvoiceWorker(attachmentRepo, invoiceV2Repo, fileStorage, aiAuditor, disp)

	att := &entity.Attachment{
		ID:             3,
		InstanceID:     100,
		FileName:       "contract.docx",
		FilePath:       "/tmp/test/contract.docx",
		DownloadStatus: entity.AttachmentStatusCompleted,
	}

	err := w.processAttachment(context.Background(), att)
	if err != nil {
		t.Fatalf("processAttachment() error = %v", err)
	}

	// Verify file type set to OTHER
	foundOtherType := false
	for _, ft := range attachmentRepo.fileTypeUpdates {
		if ft.ID == 3 && ft.FileType == entity.FileTypeOther {
			foundOtherType = true
		}
	}
	if !foundOtherType {
		t.Errorf("Expected file type update to OTHER for .docx, got %v", attachmentRepo.fileTypeUpdates)
	}

	// Verify status ends at PROCESSED
	lastUpdate := attachmentRepo.statusUpdates[len(attachmentRepo.statusUpdates)-1]
	if lastUpdate.Status != entity.AttachmentStatusProcessed {
		t.Errorf("Last status = %q, want %q", lastUpdate.Status, entity.AttachmentStatusProcessed)
	}

	// No InvoiceV2 created
	if len(invoiceV2Repo.createdInvoices) != 0 {
		t.Errorf("Expected 0 InvoiceV2 for .docx, got %d", len(invoiceV2Repo.createdInvoices))
	}
}

func TestInvoiceWorker_CheckInstanceCompletion_EmitsEvent(t *testing.T) {
	// All 2 attachments processed → should emit TypeInvoiceExtracted
	attachmentRepo := &mockWorkerAttachmentRepo{
		attachments: []*entity.Attachment{
			{ID: 1, InstanceID: 100, DownloadStatus: entity.AttachmentStatusProcessed},
			{ID: 2, InstanceID: 100, DownloadStatus: entity.AttachmentStatusProcessed},
		},
		countResult: 2, // 2 processed
	}
	disp := &mockWorkerDispatcher{}

	logger, _ := zap.NewDevelopment()
	w := &InvoiceWorker{
		attachmentRepo: attachmentRepo,
		dispatcher:     disp,
		logger:         logger,
	}

	w.checkInstanceCompletion(context.Background(), 100)

	if len(disp.dispatchedEvents) != 1 {
		t.Fatalf("Expected 1 event dispatched, got %d", len(disp.dispatchedEvents))
	}

	evt := disp.dispatchedEvents[0]
	if evt.Type != event.TypeInvoiceExtracted {
		t.Errorf("Event type = %v, want %v", evt.Type, event.TypeInvoiceExtracted)
	}
	if evt.InstanceID != 100 {
		t.Errorf("Event InstanceID = %d, want 100", evt.InstanceID)
	}
}

func TestInvoiceWorker_CheckInstanceCompletion_NotAllProcessed(t *testing.T) {
	// 1 of 2 attachments processed → should NOT emit event
	attachmentRepo := &mockWorkerAttachmentRepo{
		attachments: []*entity.Attachment{
			{ID: 1, InstanceID: 100, DownloadStatus: entity.AttachmentStatusProcessed},
			{ID: 2, InstanceID: 100, DownloadStatus: entity.AttachmentStatusCompleted}, // still downloading
		},
		countResult: 1, // only 1 processed
	}
	disp := &mockWorkerDispatcher{}

	logger, _ := zap.NewDevelopment()
	w := &InvoiceWorker{
		attachmentRepo: attachmentRepo,
		dispatcher:     disp,
		logger:         logger,
	}

	w.checkInstanceCompletion(context.Background(), 100)

	if len(disp.dispatchedEvents) != 0 {
		t.Errorf("Expected 0 events dispatched (not all processed), got %d", len(disp.dispatchedEvents))
	}
}
