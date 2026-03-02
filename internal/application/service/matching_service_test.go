package service

import (
	"context"
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
)

// --- Mocks for MatchingService ---

type mockMatchingItemRepo struct {
	items []*entity.ReimbursementItem
}

func (m *mockMatchingItemRepo) Create(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}
func (m *mockMatchingItemRepo) GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error) {
	return nil, nil
}
func (m *mockMatchingItemRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
	return m.items, nil
}
func (m *mockMatchingItemRepo) Update(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}

type mockMatchingInvoiceV2Repo struct {
	invoices []*entity.InvoiceV2
}

func (m *mockMatchingInvoiceV2Repo) Create(ctx context.Context, invoice *entity.InvoiceV2) error {
	return nil
}
func (m *mockMatchingInvoiceV2Repo) GetByID(ctx context.Context, id int64) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockMatchingInvoiceV2Repo) GetByAttachmentID(ctx context.Context, attachmentID int64) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockMatchingInvoiceV2Repo) GetByItemID(ctx context.Context, itemID int64) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockMatchingInvoiceV2Repo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.InvoiceV2, error) {
	return m.invoices, nil
}
func (m *mockMatchingInvoiceV2Repo) GetByUniqueID(ctx context.Context, uniqueID string) (*entity.InvoiceV2, error) {
	return nil, nil
}
func (m *mockMatchingInvoiceV2Repo) GetTotalsByInstanceID(ctx context.Context, instanceID int64) (*port.InvoiceTotals, error) {
	return nil, nil
}
func (m *mockMatchingInvoiceV2Repo) Update(ctx context.Context, invoice *entity.InvoiceV2) error {
	return nil
}

type mockMatchingAttachmentRepo struct {
	updateItemIDCalls []struct{ AttachmentID int64; ItemID int64 }
}

func (m *mockMatchingAttachmentRepo) Create(ctx context.Context, att *entity.Attachment) error {
	return nil
}
func (m *mockMatchingAttachmentRepo) GetByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	return nil, nil
}
func (m *mockMatchingAttachmentRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
	return nil, nil
}
func (m *mockMatchingAttachmentRepo) GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return nil, nil
}
func (m *mockMatchingAttachmentRepo) MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error {
	return nil
}
func (m *mockMatchingAttachmentRepo) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	return nil
}
func (m *mockMatchingAttachmentRepo) GetCompletedUnprocessed(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return nil, nil
}
func (m *mockMatchingAttachmentRepo) UpdateItemID(ctx context.Context, attachmentID int64, itemID int64) error {
	m.updateItemIDCalls = append(m.updateItemIDCalls, struct{ AttachmentID int64; ItemID int64 }{attachmentID, itemID})
	return nil
}
func (m *mockMatchingAttachmentRepo) UpdateFileType(ctx context.Context, attachmentID int64, fileType string) error {
	return nil
}
func (m *mockMatchingAttachmentRepo) GetByInstanceIDAndStatus(ctx context.Context, instanceID int64, status string) ([]*entity.Attachment, error) {
	return nil, nil
}
func (m *mockMatchingAttachmentRepo) CountByInstanceIDAndStatuses(ctx context.Context, instanceID int64, statuses []string) (int, error) {
	return 0, nil
}

type mockMatchingAIMatcher struct {
	result *port.InvoiceMatchingResult
	err    error
}

func (m *mockMatchingAIMatcher) MatchInvoicesToItems(ctx context.Context, items []*entity.ReimbursementItem, invoices []*entity.InvoiceV2) (*port.InvoiceMatchingResult, error) {
	return m.result, m.err
}

type mockMatchingDispatcher struct {
	dispatchedEvents []*event.Event
}

func (m *mockMatchingDispatcher) Subscribe(eventType event.Type, handler dispatcher.Handler)                   {}
func (m *mockMatchingDispatcher) SubscribeNamed(eventType event.Type, name string, handler dispatcher.Handler) {}
func (m *mockMatchingDispatcher) Unsubscribe(eventType event.Type, name string)                                {}
func (m *mockMatchingDispatcher) Dispatch(ctx context.Context, evt *event.Event) error {
	m.dispatchedEvents = append(m.dispatchedEvents, evt)
	return nil
}
func (m *mockMatchingDispatcher) DispatchAsync(ctx context.Context, evt *event.Event) {
	m.dispatchedEvents = append(m.dispatchedEvents, evt)
}
func (m *mockMatchingDispatcher) ListHandlers(eventType event.Type) []dispatcher.HandlerInfo {
	return nil
}
func (m *mockMatchingDispatcher) Close() error {
	return nil
}

// --- Tests ---

func TestMatchingService_Mismatch_TollInvoiceVsHotelItem(t *testing.T) {
	// Item: 酒店/住宿费 900 CNY
	// Invoice: 浙江通行费电子发票 50 CNY
	// Amounts don't match → AI returns no matches
	itemRepo := &mockMatchingItemRepo{
		items: []*entity.ReimbursementItem{
			{
				ID:          1,
				InstanceID:  100,
				ItemType:    entity.ItemTypeAccommodation,
				Description: "酒店",
				Amount:      900.0,
				AmountCents: 90000,
				Currency:    "CNY",
			},
		},
	}

	invoiceV2Repo := &mockMatchingInvoiceV2Repo{
		invoices: []*entity.InvoiceV2{
			{
				ID:                 1,
				InstanceID:         100,
				AttachmentID:       10,
				InvoiceCode:        "033002200311",
				InvoiceNumber:      "45677890",
				InvoiceAmountCents: 5000, // 50 CNY
				SellerName:         "浙江省交通投资集团有限公司",
			},
		},
	}

	attachmentRepo := &mockMatchingAttachmentRepo{}

	// AI matcher returns no matches due to amount mismatch
	matcher := &mockMatchingAIMatcher{
		result: &port.InvoiceMatchingResult{
			Matches:    []port.InvoiceItemMatch{}, // empty - no match
			Confidence: 0.9,
			Reasoning:  "Amount mismatch: invoice 50 CNY vs item 900 CNY. Item types also differ (toll vs accommodation).",
		},
	}

	disp := &mockMatchingDispatcher{}
	logger := &mockLogger{}

	svc := NewMatchingService(
		itemRepo, invoiceV2Repo, attachmentRepo, matcher,
		&mockTxManager{}, disp, logger,
	)

	err := svc.MatchInstance(context.Background(), 100)
	if err != nil {
		t.Fatalf("MatchInstance() error = %v", err)
	}

	// Verify NO UpdateItemID calls (item_id stays null)
	if len(attachmentRepo.updateItemIDCalls) != 0 {
		t.Errorf("Expected 0 UpdateItemID calls (mismatch), got %d: %v",
			len(attachmentRepo.updateItemIDCalls), attachmentRepo.updateItemIDCalls)
	}

	// Verify DataReady event still emitted
	if len(disp.dispatchedEvents) != 1 {
		t.Fatalf("Expected 1 DataReady event dispatched, got %d", len(disp.dispatchedEvents))
	}
	evt := disp.dispatchedEvents[0]
	if evt.Type != event.TypeDataReady {
		t.Errorf("Event type = %v, want %v", evt.Type, event.TypeDataReady)
	}
	matchesCount := evt.GetPayloadInt("matches_count")
	if matchesCount != 0 {
		t.Errorf("matches_count = %d, want 0", matchesCount)
	}
}

func TestMatchingService_NoInvoices(t *testing.T) {
	itemRepo := &mockMatchingItemRepo{
		items: []*entity.ReimbursementItem{
			{ID: 1, InstanceID: 100, Description: "酒店", Amount: 900.0},
		},
	}
	invoiceV2Repo := &mockMatchingInvoiceV2Repo{
		invoices: []*entity.InvoiceV2{}, // no invoices
	}
	attachmentRepo := &mockMatchingAttachmentRepo{}
	// matcher should NOT be called when there are no invoices
	matcher := &mockMatchingAIMatcher{
		result: nil,
	}
	disp := &mockMatchingDispatcher{}
	logger := &mockLogger{}

	svc := NewMatchingService(
		itemRepo, invoiceV2Repo, attachmentRepo, matcher,
		&mockTxManager{}, disp, logger,
	)

	err := svc.MatchInstance(context.Background(), 100)
	if err != nil {
		t.Fatalf("MatchInstance() error = %v", err)
	}

	// No UpdateItemID calls
	if len(attachmentRepo.updateItemIDCalls) != 0 {
		t.Errorf("Expected 0 UpdateItemID calls, got %d", len(attachmentRepo.updateItemIDCalls))
	}

	// DataReady event still emitted
	if len(disp.dispatchedEvents) != 1 {
		t.Fatalf("Expected 1 DataReady event, got %d", len(disp.dispatchedEvents))
	}
	if disp.dispatchedEvents[0].Type != event.TypeDataReady {
		t.Errorf("Event type = %v, want %v", disp.dispatchedEvents[0].Type, event.TypeDataReady)
	}
}

func TestMatchingService_SuccessfulMatch(t *testing.T) {
	itemRepo := &mockMatchingItemRepo{
		items: []*entity.ReimbursementItem{
			{ID: 1, InstanceID: 100, Description: "机票", Amount: 500.0, AmountCents: 50000},
		},
	}
	invoiceV2Repo := &mockMatchingInvoiceV2Repo{
		invoices: []*entity.InvoiceV2{
			{
				ID:                 1,
				InstanceID:         100,
				AttachmentID:       10,
				InvoiceAmountCents: 50000, // 500 CNY - matches
				SellerName:         "某航空公司",
			},
		},
	}
	attachmentRepo := &mockMatchingAttachmentRepo{}

	matcher := &mockMatchingAIMatcher{
		result: &port.InvoiceMatchingResult{
			Matches: []port.InvoiceItemMatch{
				{
					AttachmentID: 10,
					ItemID:       1,
					InvoiceID:    1,
					Confidence:   0.95,
					Reasoning:    "Amount matches (500 CNY) and aviation-related invoice matches flight item.",
				},
			},
			Confidence: 0.95,
			Reasoning:  "All invoices matched.",
		},
	}

	disp := &mockMatchingDispatcher{}
	logger := &mockLogger{}

	svc := NewMatchingService(
		itemRepo, invoiceV2Repo, attachmentRepo, matcher,
		&mockTxManager{}, disp, logger,
	)

	err := svc.MatchInstance(context.Background(), 100)
	if err != nil {
		t.Fatalf("MatchInstance() error = %v", err)
	}

	// Verify UpdateItemID called with correct IDs
	if len(attachmentRepo.updateItemIDCalls) != 1 {
		t.Fatalf("Expected 1 UpdateItemID call, got %d", len(attachmentRepo.updateItemIDCalls))
	}
	call := attachmentRepo.updateItemIDCalls[0]
	if call.AttachmentID != 10 {
		t.Errorf("UpdateItemID AttachmentID = %d, want 10", call.AttachmentID)
	}
	if call.ItemID != 1 {
		t.Errorf("UpdateItemID ItemID = %d, want 1", call.ItemID)
	}

	// DataReady event emitted with matches_count=1
	if len(disp.dispatchedEvents) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(disp.dispatchedEvents))
	}
	matchesCount := disp.dispatchedEvents[0].GetPayloadInt("matches_count")
	if matchesCount != 1 {
		t.Errorf("matches_count = %d, want 1", matchesCount)
	}
}
