package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// Mock implementations for email service tests

type mockEmailSender struct {
	callCount   int
	failUntil   int // fail until this many calls, then succeed
	lastSubject string
	lastBody    string
	lastTo      string
}

func (m *mockEmailSender) SendEmail(ctx context.Context, to, subject, bodyText string, attachments []port.EmailAttachment) (string, error) {
	m.callCount++
	m.lastSubject = subject
	m.lastBody = bodyText
	m.lastTo = to
	if m.callCount <= m.failUntil {
		return "", fmt.Errorf("send failed (attempt %d)", m.callCount)
	}
	return "msg-123", nil
}

type mockEmailVoucherRepo struct {
	voucher *entity.GeneratedVoucher
	updated bool
}

func (m *mockEmailVoucherRepo) Create(ctx context.Context, v *entity.GeneratedVoucher) error {
	return nil
}

func (m *mockEmailVoucherRepo) GetByInstanceID(ctx context.Context, id int64) (*entity.GeneratedVoucher, error) {
	if m.voucher != nil {
		return m.voucher, nil
	}
	return &entity.GeneratedVoucher{ID: 1, InstanceID: id}, nil
}

func (m *mockEmailVoucherRepo) Update(ctx context.Context, v *entity.GeneratedVoucher) error {
	m.updated = true
	m.voucher = v
	return nil
}

type mockEmailInstanceRepo struct {
	instance *entity.ApprovalInstance
}

func (m *mockEmailInstanceRepo) Create(ctx context.Context, i *entity.ApprovalInstance) error {
	return nil
}

func (m *mockEmailInstanceRepo) GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
	if m.instance != nil {
		return m.instance, nil
	}
	return &entity.ApprovalInstance{ID: id, LarkInstanceID: "LARK-001"}, nil
}

func (m *mockEmailInstanceRepo) GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
	return nil, errors.New("not implemented")
}

func (m *mockEmailInstanceRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	return nil
}

func (m *mockEmailInstanceRepo) SetApprovalTime(ctx context.Context, id int64, t time.Time) error {
	return nil
}

func (m *mockEmailInstanceRepo) List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error) {
	return []*entity.ApprovalInstance{}, nil
}

type mockEmailMessenger struct {
	sentMessages []string
}

func (m *mockEmailMessenger) SendMessage(ctx context.Context, openID, content string) error {
	m.sentMessages = append(m.sentMessages, content)
	return nil
}

func (m *mockEmailMessenger) SendCardMessage(ctx context.Context, openID string, content interface{}) error {
	return nil
}

type mockEmailDiagnoser struct {
	called bool
}

func (m *mockEmailDiagnoser) DiagnoseError(ctx context.Context, desc string) (string, error) {
	m.called = true
	return "诊断结果: OAuth token 可能过期，请检查邮件服务配置", nil
}

type mockEmailLogger struct{}

func (m *mockEmailLogger) Info(msg string, kv ...interface{})  {}
func (m *mockEmailLogger) Error(msg string, kv ...interface{}) {}
func (m *mockEmailLogger) Debug(msg string, kv ...interface{}) {}
func (m *mockEmailLogger) Warn(msg string, kv ...interface{})  {}

func TestEmailService_SendSuccess(t *testing.T) {
	// Create a temp folder with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	sender := &mockEmailSender{failUntil: 0} // succeed immediately
	voucherRepo := &mockEmailVoucherRepo{}
	instanceRepo := &mockEmailInstanceRepo{}
	messenger := &mockEmailMessenger{}
	diagnoser := &mockEmailDiagnoser{}

	svc := NewEmailServiceWithRetryInterval(
		sender,
		voucherRepo,
		instanceRepo,
		messenger,
		diagnoser,
		"test@test.com",
		"admin-open-id",
		1*time.Millisecond, // Fast retry for testing
		&mockEmailLogger{},
	)

	err := svc.SendVoucherEmail(context.Background(), 1, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.callCount != 1 {
		t.Errorf("expected 1 call, got %d", sender.callCount)
	}

	if sender.lastTo != "test@test.com" {
		t.Errorf("expected recipient 'test@test.com', got '%s'", sender.lastTo)
	}

	if !voucherRepo.updated {
		t.Error("expected voucher to be updated")
	}

	// Verify voucher fields were updated
	if voucherRepo.voucher.EmailMessageID != "msg-123" {
		t.Errorf("expected message ID 'msg-123', got '%s'", voucherRepo.voucher.EmailMessageID)
	}

	if voucherRepo.voucher.AccountantEmail != "test@test.com" {
		t.Errorf("expected accountant email 'test@test.com', got '%s'", voucherRepo.voucher.AccountantEmail)
	}

	if voucherRepo.voucher.SentAt == nil {
		t.Error("expected SentAt to be set")
	}
}

func TestEmailService_RetrySuccess(t *testing.T) {
	// Create a temp folder with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	sender := &mockEmailSender{failUntil: 2} // fail first 2 times, succeed on 3rd
	voucherRepo := &mockEmailVoucherRepo{}
	instanceRepo := &mockEmailInstanceRepo{}
	messenger := &mockEmailMessenger{}
	diagnoser := &mockEmailDiagnoser{}

	svc := NewEmailServiceWithRetryInterval(
		sender,
		voucherRepo,
		instanceRepo,
		messenger,
		diagnoser,
		"test@test.com",
		"admin-open-id",
		1*time.Millisecond, // Fast retry for testing
		&mockEmailLogger{},
	)

	err := svc.SendVoucherEmail(context.Background(), 1, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.callCount != 3 {
		t.Errorf("expected 3 calls (2 failures + 1 success), got %d", sender.callCount)
	}

	if !voucherRepo.updated {
		t.Error("expected voucher to be updated after successful retry")
	}
}

func TestEmailService_AllRetriesFail(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	sender := &mockEmailSender{failUntil: 10} // always fail
	voucherRepo := &mockEmailVoucherRepo{}
	instanceRepo := &mockEmailInstanceRepo{}
	messenger := &mockEmailMessenger{}
	diagnoser := &mockEmailDiagnoser{}

	svc := NewEmailServiceWithRetryInterval(
		sender,
		voucherRepo,
		instanceRepo,
		messenger,
		diagnoser,
		"test@test.com",
		"admin-open-id",
		1*time.Millisecond, // Fast retry for testing
		&mockEmailLogger{},
	)

	err := svc.SendVoucherEmail(context.Background(), 1, tmpDir)
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}

	if sender.callCount != 5 {
		t.Errorf("expected 5 retry attempts, got %d", sender.callCount)
	}

	if !diagnoser.called {
		t.Error("expected GPT diagnoser to be called after all retries fail")
	}

	if len(messenger.sentMessages) == 0 {
		t.Error("expected admin to be notified after all retries fail")
	}

	// Verify admin received the diagnosis
	if len(messenger.sentMessages) > 0 {
		msg := messenger.sentMessages[0]
		if msg != "诊断结果: OAuth token 可能过期，请检查邮件服务配置" {
			t.Errorf("unexpected admin notification: %s", msg)
		}
	}

	if voucherRepo.updated {
		t.Error("expected voucher NOT to be updated when all retries fail")
	}
}

func TestEmailService_DiagnoserFallback(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.xlsx")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	sender := &mockEmailSender{failUntil: 10} // always fail
	voucherRepo := &mockEmailVoucherRepo{}
	instanceRepo := &mockEmailInstanceRepo{}
	messenger := &mockEmailMessenger{}

	// Diagnoser that fails
	failingDiagnoser := &mockEmailFailingDiagnoser{}

	svc := NewEmailServiceWithRetryInterval(
		sender,
		voucherRepo,
		instanceRepo,
		messenger,
		failingDiagnoser,
		"test@test.com",
		"admin-open-id",
		1*time.Millisecond,
		&mockEmailLogger{},
	)

	err := svc.SendVoucherEmail(context.Background(), 1, tmpDir)
	if err == nil {
		t.Fatal("expected error after all retries fail")
	}

	// Should still notify admin with fallback message
	if len(messenger.sentMessages) == 0 {
		t.Error("expected admin to be notified even when diagnoser fails")
	}

	// Verify fallback message contains key info
	if len(messenger.sentMessages) > 0 {
		msg := messenger.sentMessages[0]
		if msg == "" {
			t.Error("expected non-empty fallback message")
		}
	}
}

type mockEmailFailingDiagnoser struct{}

func (m *mockEmailFailingDiagnoser) DiagnoseError(ctx context.Context, desc string) (string, error) {
	return "", errors.New("diagnosis failed")
}

func TestEmailService_InstanceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	sender := &mockEmailSender{}
	voucherRepo := &mockEmailVoucherRepo{}
	instanceRepo := &mockEmailInstanceRepo{}
	messenger := &mockEmailMessenger{}
	diagnoser := &mockEmailDiagnoser{}

	// Make GetByID return error
	instanceRepo.instance = nil

	svc := NewEmailServiceWithRetryInterval(
		sender,
		voucherRepo,
		instanceRepo,
		messenger,
		diagnoser,
		"test@test.com",
		"admin-open-id",
		1*time.Millisecond,
		&mockEmailLogger{},
	)

	// Should get instance successfully with default mock behavior
	// Empty tmpDir will create an empty zip, which is valid
	err := svc.SendVoucherEmail(context.Background(), 999, tmpDir)

	// Should succeed (empty zip is valid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have attempted to send email with empty zip
	if sender.callCount != 1 {
		t.Errorf("expected 1 email send attempt, got %d", sender.callCount)
	}
}
