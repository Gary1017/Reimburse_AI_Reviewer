package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

const testFixtureDir = "../../../testdata/lark_fixtures"

// loadInstanceDetailFixture loads the instance_detail fixture and builds a port.LarkInstanceDetail
func loadInstanceDetailFixture(t *testing.T) *port.LarkInstanceDetail {
	t.Helper()
	data, err := os.ReadFile(testFixtureDir + "/20260217_172016_01_instance_detail.json")
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to parse fixture: %v", err)
	}

	detail := &port.LarkInstanceDetail{
		InstanceCode: raw["instance_code"].(string),
		ApprovalCode: raw["approval_code"].(string),
		UserID:       raw["user_id"].(string),
		OpenID:       raw["open_id"].(string),
		Status:       raw["status"].(string),
		FormData:     raw["form"].(string),
	}

	if startTimeStr, ok := raw["start_time"].(string); ok {
		if st, err := strconv.ParseInt(startTimeStr, 10, 64); err == nil {
			detail.StartTime = st
		}
	}

	// Parse task_list
	if taskListRaw, ok := raw["task_list"].([]interface{}); ok {
		for _, tRaw := range taskListRaw {
			tMap := tRaw.(map[string]interface{})
			task := port.LarkTaskInfo{
				TaskID:   tMap["id"].(string),
				UserID:   tMap["user_id"].(string),
				OpenID:   tMap["open_id"].(string),
				Status:   tMap["status"].(string),
				NodeID:   tMap["node_id"].(string),
				NodeName: tMap["node_name"].(string),
				Type:     tMap["type"].(string),
			}
			if st, ok := tMap["start_time"].(string); ok {
				if v, err := strconv.ParseInt(st, 10, 64); err == nil {
					task.StartTime = v
				}
			}
			detail.TaskList = append(detail.TaskList, task)
		}
	}

	return detail
}

// --- Mocks for InstanceCreationService ---

type mockCreationInstanceRepo struct {
	instances             map[string]*entity.ApprovalInstance
	createdInstances      []*entity.ApprovalInstance
	updateStatusCalls     []struct{ ID int64; Status string }
	getByLarkInstanceIDFunc func(ctx context.Context, larkID string) (*entity.ApprovalInstance, error)
}

func newMockCreationInstanceRepo() *mockCreationInstanceRepo {
	return &mockCreationInstanceRepo{
		instances: make(map[string]*entity.ApprovalInstance),
	}
}

func (m *mockCreationInstanceRepo) Create(ctx context.Context, instance *entity.ApprovalInstance) error {
	instance.ID = int64(len(m.createdInstances) + 1)
	m.createdInstances = append(m.createdInstances, instance)
	m.instances[instance.LarkInstanceID] = instance
	return nil
}

func (m *mockCreationInstanceRepo) GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
	for _, inst := range m.instances {
		if inst.ID == id {
			return inst, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockCreationInstanceRepo) GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
	if m.getByLarkInstanceIDFunc != nil {
		return m.getByLarkInstanceIDFunc(ctx, larkID)
	}
	if inst, ok := m.instances[larkID]; ok {
		return inst, nil
	}
	return nil, errors.New("not found")
}

func (m *mockCreationInstanceRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	m.updateStatusCalls = append(m.updateStatusCalls, struct{ ID int64; Status string }{id, status})
	return nil
}

func (m *mockCreationInstanceRepo) SetApprovalTime(ctx context.Context, id int64, t time.Time) error {
	return nil
}

func (m *mockCreationInstanceRepo) List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error) {
	return nil, nil
}

type mockCreationItemRepo struct {
	createdItems []*entity.ReimbursementItem
}

func (m *mockCreationItemRepo) Create(ctx context.Context, item *entity.ReimbursementItem) error {
	item.ID = int64(len(m.createdItems) + 1)
	m.createdItems = append(m.createdItems, item)
	return nil
}

func (m *mockCreationItemRepo) GetByID(ctx context.Context, id int64) (*entity.ReimbursementItem, error) {
	return nil, nil
}

func (m *mockCreationItemRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ReimbursementItem, error) {
	return nil, nil
}

func (m *mockCreationItemRepo) Update(ctx context.Context, item *entity.ReimbursementItem) error {
	return nil
}

type mockCreationAttachmentRepo struct {
	createdAttachments []*entity.Attachment
}

func (m *mockCreationAttachmentRepo) Create(ctx context.Context, att *entity.Attachment) error {
	att.ID = int64(len(m.createdAttachments) + 1)
	m.createdAttachments = append(m.createdAttachments, att)
	return nil
}

func (m *mockCreationAttachmentRepo) GetByID(ctx context.Context, id int64) (*entity.Attachment, error) {
	return nil, nil
}

func (m *mockCreationAttachmentRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockCreationAttachmentRepo) GetPending(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockCreationAttachmentRepo) MarkCompleted(ctx context.Context, id int64, filePath string, fileSize int64) error {
	return nil
}

func (m *mockCreationAttachmentRepo) UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error {
	return nil
}

func (m *mockCreationAttachmentRepo) GetCompletedUnprocessed(ctx context.Context, limit int) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockCreationAttachmentRepo) UpdateItemID(ctx context.Context, attachmentID int64, itemID int64) error {
	return nil
}

func (m *mockCreationAttachmentRepo) UpdateFileType(ctx context.Context, attachmentID int64, fileType string) error {
	return nil
}

func (m *mockCreationAttachmentRepo) GetByInstanceIDAndStatus(ctx context.Context, instanceID int64, status string) ([]*entity.Attachment, error) {
	return nil, nil
}

func (m *mockCreationAttachmentRepo) CountByInstanceIDAndStatuses(ctx context.Context, instanceID int64, statuses []string) (int, error) {
	return 0, nil
}

type mockCreationTaskRepo struct {
	createdTasks []*entity.ApprovalTask
}

func (m *mockCreationTaskRepo) Create(ctx context.Context, task *entity.ApprovalTask) error {
	task.ID = int64(len(m.createdTasks) + 1)
	m.createdTasks = append(m.createdTasks, task)
	return nil
}

func (m *mockCreationTaskRepo) GetByID(ctx context.Context, id int64) (*entity.ApprovalTask, error) {
	return nil, nil
}

func (m *mockCreationTaskRepo) GetByLarkTaskID(ctx context.Context, larkTaskID string) (*entity.ApprovalTask, error) {
	return nil, nil
}

func (m *mockCreationTaskRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalTask, error) {
	return nil, nil
}

func (m *mockCreationTaskRepo) GetCurrentTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error) {
	return nil, nil
}

func (m *mockCreationTaskRepo) GetAIReviewTask(ctx context.Context, instanceID int64) (*entity.ApprovalTask, error) {
	return nil, nil
}

func (m *mockCreationTaskRepo) Update(ctx context.Context, task *entity.ApprovalTask) error {
	return nil
}

func (m *mockCreationTaskRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	return nil
}

func (m *mockCreationTaskRepo) CompleteTask(ctx context.Context, id int64, decision string, confidence *float64, resultData string, violations string, completedBy string) error {
	return nil
}

func (m *mockCreationTaskRepo) SetCurrent(ctx context.Context, instanceID int64, taskID int64) error {
	return nil
}

func (m *mockCreationTaskRepo) MarkNotificationSent(ctx context.Context, id int64) error {
	return nil
}

type mockCreationLarkClient struct {
	detail *port.LarkInstanceDetail
	err    error
}

func (m *mockCreationLarkClient) GetInstanceDetail(ctx context.Context, instanceID string) (*port.LarkInstanceDetail, error) {
	return m.detail, m.err
}

func (m *mockCreationLarkClient) GetApprovers(ctx context.Context, instanceID string) ([]port.ApproverInfo, error) {
	return nil, nil
}

type mockCreationFormParser struct {
	items      []*entity.ReimbursementItem
	attachRefs []*entity.AttachmentReference
	err        error
}

func (m *mockCreationFormParser) ParseWithAttachments(jsonData string) ([]*entity.ReimbursementItem, []*entity.AttachmentReference, error) {
	return m.items, m.attachRefs, m.err
}

// --- Tests ---

func TestInstanceCreationService_HappyPath(t *testing.T) {
	detail := loadInstanceDetailFixture(t)

	instanceRepo := newMockCreationInstanceRepo()
	itemRepo := &mockCreationItemRepo{}
	attachmentRepo := &mockCreationAttachmentRepo{}
	taskRepo := &mockCreationTaskRepo{}
	larkClient := &mockCreationLarkClient{detail: detail}

	// FormParser returns parsed items and attachments from fixture data
	formParser := &mockCreationFormParser{
		items: []*entity.ReimbursementItem{
			{
				ItemType:    entity.ItemTypeAccommodation,
				Description: "酒店",
				Amount:      900.0,
				Currency:    "CNY",
			},
		},
		attachRefs: []*entity.AttachmentReference{
			{
				OriginalName: "浙江通行费电子发票1.pdf",
				URL:          "https://internal-api-drive-stream.feishu.cn/space/api/box/stream/download/authcode/?code=ZDIyNTAzYjdlY2IwOWE1YzhkMzNjZjY3YmY1ODVmNDdfMTdkNjMyYTJiODMxNTg4MjFiNDBmNzE0Y2Q5NmYzMmZfSUQ6NzU5Nzc1NDIzMDQ4OTA1ODUyM18xNzcxMzIwMDE3OjE3NzE0MDY0MTdfVjM",
			},
		},
	}

	txManager := &mockTxManager{}
	logger := &mockLogger{}

	svc := NewInstanceCreationService(
		instanceRepo, itemRepo, attachmentRepo, taskRepo,
		larkClient, formParser, txManager,
		"ou_ai_approver_open_id", "ai_approver_user_id",
		logger,
	)

	err := svc.HandleInstanceCreated(context.Background(), "BBD14B93-13A7-4ECE-8F78-901C9E94C820", nil)
	if err != nil {
		t.Fatalf("HandleInstanceCreated() error = %v", err)
	}

	// Verify instance created
	if len(instanceRepo.createdInstances) != 1 {
		t.Fatalf("Expected 1 instance created, got %d", len(instanceRepo.createdInstances))
	}
	inst := instanceRepo.createdInstances[0]
	if inst.LarkInstanceID != "BBD14B93-13A7-4ECE-8F78-901C9E94C820" {
		t.Errorf("Instance LarkInstanceID = %q, want BBD14B93-13A7-4ECE-8F78-901C9E94C820", inst.LarkInstanceID)
	}
	if inst.Status != entity.StatusCreated {
		t.Errorf("Instance Status = %q, want %q", inst.Status, entity.StatusCreated)
	}
	if inst.ApplicantUserID != "bj1" {
		t.Errorf("Instance ApplicantUserID = %q, want 'bj1'", inst.ApplicantUserID)
	}
	if inst.FormData != detail.FormData {
		t.Errorf("Instance FormData not set from fixture detail")
	}

	// Verify reimbursement item created
	if len(itemRepo.createdItems) != 1 {
		t.Fatalf("Expected 1 item created, got %d", len(itemRepo.createdItems))
	}
	item := itemRepo.createdItems[0]
	if item.ItemType != entity.ItemTypeAccommodation {
		t.Errorf("Item type = %q, want %q", item.ItemType, entity.ItemTypeAccommodation)
	}
	if item.Description != "酒店" {
		t.Errorf("Item description = %q, want '酒店'", item.Description)
	}
	if item.Amount != 900.0 {
		t.Errorf("Item amount = %v, want 900.0", item.Amount)
	}
	if item.AmountCents != 90000 {
		t.Errorf("Item AmountCents = %d, want 90000", item.AmountCents)
	}
	if item.InstanceID != inst.ID {
		t.Errorf("Item InstanceID = %d, want %d", item.InstanceID, inst.ID)
	}

	// Verify attachment created
	if len(attachmentRepo.createdAttachments) != 1 {
		t.Fatalf("Expected 1 attachment created, got %d", len(attachmentRepo.createdAttachments))
	}
	att := attachmentRepo.createdAttachments[0]
	if att.FileName != "浙江通行费电子发票1.pdf" {
		t.Errorf("Attachment FileName = %q, want '浙江通行费电子发票1.pdf'", att.FileName)
	}
	if att.DownloadStatus != entity.AttachmentStatusPending {
		t.Errorf("Attachment DownloadStatus = %q, want %q", att.DownloadStatus, entity.AttachmentStatusPending)
	}
	if att.InstanceID != inst.ID {
		t.Errorf("Attachment InstanceID = %d, want %d", att.InstanceID, inst.ID)
	}

	// Verify tasks created: 1 AI review + 1 human review (from fixture task_list)
	if len(taskRepo.createdTasks) != 2 {
		t.Fatalf("Expected 2 tasks created (1 AI + 1 human), got %d", len(taskRepo.createdTasks))
	}

	aiTask := taskRepo.createdTasks[0]
	if aiTask.TaskType != entity.TaskTypeAIReview {
		t.Errorf("AI task type = %q, want %q", aiTask.TaskType, entity.TaskTypeAIReview)
	}
	if aiTask.SequenceNumber != 0 {
		t.Errorf("AI task sequence = %d, want 0", aiTask.SequenceNumber)
	}
	if !aiTask.IsCurrent {
		t.Error("AI task should be current")
	}
	if !aiTask.IsAIDecision {
		t.Error("AI task should be AI decision")
	}

	humanTask := taskRepo.createdTasks[1]
	if humanTask.TaskType != entity.TaskTypeHumanReview {
		t.Errorf("Human task type = %q, want %q", humanTask.TaskType, entity.TaskTypeHumanReview)
	}
	if humanTask.LarkTaskID != "7607761536752307417" {
		t.Errorf("Human task LarkTaskID = %q, want '7607761536752307417'", humanTask.LarkTaskID)
	}
	if humanTask.NodeName != "AI审阅审批内容" {
		t.Errorf("Human task NodeName = %q, want 'AI审阅审批内容'", humanTask.NodeName)
	}
	if humanTask.SequenceNumber != 1 {
		t.Errorf("Human task sequence = %d, want 1", humanTask.SequenceNumber)
	}
}

func TestInstanceCreationService_Idempotency(t *testing.T) {
	instanceRepo := newMockCreationInstanceRepo()
	instanceRepo.getByLarkInstanceIDFunc = func(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
		return &entity.ApprovalInstance{
			ID:             1,
			LarkInstanceID: larkID,
			FormData:       `[{"id":"widget1","name":"test"}]`, // Non-empty FormData
		}, nil
	}

	larkClient := &mockCreationLarkClient{
		detail: nil,
		err:    errors.New("should not be called"),
	}

	svc := NewInstanceCreationService(
		instanceRepo, &mockCreationItemRepo{}, &mockCreationAttachmentRepo{},
		&mockCreationTaskRepo{}, larkClient, nil, &mockTxManager{},
		"", "", &mockLogger{},
	)

	err := svc.HandleInstanceCreated(context.Background(), "existing-instance", nil)
	if err != nil {
		t.Fatalf("HandleInstanceCreated() should return nil for existing instance, got %v", err)
	}

	if len(instanceRepo.createdInstances) != 0 {
		t.Errorf("Expected 0 instances created (idempotency), got %d", len(instanceRepo.createdInstances))
	}
}

func TestInstanceCreationService_LarkAPIFailure(t *testing.T) {
	instanceRepo := newMockCreationInstanceRepo()
	larkClient := &mockCreationLarkClient{
		err: errors.New("Lark API timeout"),
	}

	svc := NewInstanceCreationService(
		instanceRepo, &mockCreationItemRepo{}, &mockCreationAttachmentRepo{},
		&mockCreationTaskRepo{}, larkClient, nil, &mockTxManager{},
		"", "", &mockLogger{},
	)

	err := svc.HandleInstanceCreated(context.Background(), "new-instance", nil)
	if err == nil {
		t.Fatal("HandleInstanceCreated() should return error when Lark API fails")
	}

	if len(instanceRepo.createdInstances) != 0 {
		t.Errorf("Expected 0 instances created on API failure, got %d", len(instanceRepo.createdInstances))
	}
}
