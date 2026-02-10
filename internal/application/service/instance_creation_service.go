package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// FormParserPort defines the interface for parsing Lark form data
type FormParserPort interface {
	ParseWithAttachments(jsonData string) ([]*entity.ReimbursementItem, []*entity.AttachmentReference, error)
}

// InstanceCreationService handles creation of instance records from Lark events
type InstanceCreationService interface {
	HandleInstanceCreated(ctx context.Context, larkInstanceID string, eventPayload map[string]interface{}) error
}

type instanceCreationServiceImpl struct {
	instanceRepo     port.InstanceRepository
	itemRepo         port.ItemRepository
	attachmentRepo   port.AttachmentRepository
	taskRepo         port.ApprovalTaskRepository
	larkClient       port.LarkClient
	formParser       FormParserPort
	txManager        port.TransactionManager
	aiApproverOpenID string
	aiApproverUserID string
	logger           Logger
}

// NewInstanceCreationService creates a new InstanceCreationService
func NewInstanceCreationService(
	instanceRepo port.InstanceRepository,
	itemRepo port.ItemRepository,
	attachmentRepo port.AttachmentRepository,
	taskRepo port.ApprovalTaskRepository,
	larkClient port.LarkClient,
	formParser FormParserPort,
	txManager port.TransactionManager,
	aiApproverOpenID string,
	aiApproverUserID string,
	logger Logger,
) InstanceCreationService {
	return &instanceCreationServiceImpl{
		instanceRepo:     instanceRepo,
		itemRepo:         itemRepo,
		attachmentRepo:   attachmentRepo,
		taskRepo:         taskRepo,
		larkClient:       larkClient,
		formParser:       formParser,
		txManager:        txManager,
		aiApproverOpenID: aiApproverOpenID,
		aiApproverUserID: aiApproverUserID,
		logger:           logger,
	}
}

// HandleInstanceCreated handles the creation of an instance from a Lark event
func (s *instanceCreationServiceImpl) HandleInstanceCreated(ctx context.Context, larkInstanceID string, eventPayload map[string]interface{}) error {
	// Step 1: Idempotency check
	existing, err := s.instanceRepo.GetByLarkInstanceID(ctx, larkInstanceID)
	if err == nil && existing != nil && existing.FormData != "" {
		s.logger.Info("Instance already processed with FormData", "lark_instance_id", larkInstanceID, "id", existing.ID)
		return nil
	}

	s.logger.Info("Processing instance creation", "lark_instance_id", larkInstanceID)

	// Step 2: Fetch full details from Lark
	detail, err := s.larkClient.GetInstanceDetail(ctx, larkInstanceID)
	if err != nil {
		s.logger.Error("Failed to fetch instance detail from Lark", "error", err, "lark_instance_id", larkInstanceID)
		return fmt.Errorf("fetch instance detail: %w", err)
	}

	// Step 3: Execute transaction
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		now := time.Now()

		// 3a. Create or update ApprovalInstance
		instance := &entity.ApprovalInstance{
			LarkInstanceID:  larkInstanceID,
			Status:          entity.StatusCreated,
			ApplicantUserID: detail.UserID,
			FormData:        detail.FormData,
			SubmissionTime:  time.Unix(detail.StartTime, 0),
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if existing != nil {
			// Update existing instance
			instance.ID = existing.ID
			if err := s.instanceRepo.UpdateStatus(txCtx, instance.ID, instance.Status); err != nil {
				return fmt.Errorf("update instance status: %w", err)
			}
			s.logger.Info("Updated existing instance", "id", instance.ID, "lark_instance_id", larkInstanceID)
		} else {
			// Create new instance
			if err := s.instanceRepo.Create(txCtx, instance); err != nil {
				return fmt.Errorf("create instance: %w", err)
			}
			s.logger.Info("Created new instance", "id", instance.ID, "lark_instance_id", larkInstanceID)
		}

		// 3b. Parse form data with attachments
		if s.formParser == nil {
			s.logger.Info("FormParser not available, skipping item and attachment creation", "instance_id", instance.ID)
		} else if detail.FormData == "" {
			s.logger.Info("FormData is empty, skipping parsing", "instance_id", instance.ID)
		} else {
			items, attachmentRefs, err := s.formParser.ParseWithAttachments(detail.FormData)
			if err != nil {
				s.logger.Error("Failed to parse form data", "error", err, "instance_id", instance.ID)
				// Don't fail the entire transaction - continue without items
			} else {
				// 3c. Create ReimbursementItems
				for _, item := range items {
					item.InstanceID = instance.ID
					// Convert Amount to AmountCents (multiply by 100)
					item.AmountCents = int64(item.Amount * 100)
					item.CreatedAt = now

					if err := s.itemRepo.Create(txCtx, item); err != nil {
						return fmt.Errorf("create reimbursement item: %w", err)
					}
					s.logger.Info("Created reimbursement item", "item_id", item.ID, "instance_id", instance.ID, "amount_cents", item.AmountCents)
				}

				// 3d. Create Attachments
				for _, ref := range attachmentRefs {
					attachment := &entity.Attachment{
						InstanceID:     instance.ID,
						ItemID:         0, // Will be set by InvoiceWorker
						FileName:       ref.OriginalName,
						URL:            ref.URL,
						DownloadStatus: entity.AttachmentStatusPending,
						FileType:       "", // Empty - will be set by InvoiceWorker
						CreatedAt:      now,
					}

					if err := s.attachmentRepo.Create(txCtx, attachment); err != nil {
						return fmt.Errorf("create attachment: %w", err)
					}
					s.logger.Info("Created attachment", "attachment_id", attachment.ID, "instance_id", instance.ID, "file_name", ref.OriginalName)
				}
			}
		}

		// 3e. Create AI review task (sequence 0)
		aiTask := &entity.ApprovalTask{
			InstanceID:     instance.ID,
			TaskType:       entity.TaskTypeAIReview,
			SequenceNumber: 0,
			IsCurrent:      true,
			IsAIDecision:   true,
			AssigneeUserID: s.aiApproverUserID,
			AssigneeOpenID: s.aiApproverOpenID,
			Status:         entity.TaskStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := s.taskRepo.Create(txCtx, aiTask); err != nil {
			return fmt.Errorf("create AI review task: %w", err)
		}
		s.logger.Info("Created AI review task", "task_id", aiTask.ID, "instance_id", instance.ID)

		// 3f. Create human review tasks from Lark task list
		if detail.TaskList != nil && len(detail.TaskList) > 0 {
			for i, task := range detail.TaskList {
				humanTask := &entity.ApprovalTask{
					InstanceID:     instance.ID,
					LarkTaskID:     task.TaskID,
					TaskType:       entity.TaskTypeHumanReview,
					SequenceNumber: i + 1, // Start from 1 (AI task is 0)
					IsCurrent:      false,
					IsAIDecision:   false,
					AssigneeUserID: task.UserID,
					AssigneeOpenID: task.OpenID,
					NodeID:         task.NodeID,
					NodeName:       task.NodeName,
					CustomNodeID:   task.CustomNodeID,
					ApprovalType:   task.Type,
					Status:         mapLarkTaskStatus(task.Status),
					StartTime:      formatUnixTime(task.StartTime),
					EndTime:        formatUnixTime(task.EndTime),
					CreatedAt:      now,
					UpdatedAt:      now,
				}

				if err := s.taskRepo.Create(txCtx, humanTask); err != nil {
					return fmt.Errorf("create human review task: %w", err)
				}
				s.logger.Info("Created human review task", "task_id", humanTask.ID, "instance_id", instance.ID, "lark_task_id", task.TaskID, "sequence", humanTask.SequenceNumber)
			}
		} else {
			s.logger.Info("No human review tasks in Lark task list", "instance_id", instance.ID)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to create instance in transaction", "error", err, "lark_instance_id", larkInstanceID)
		return err
	}

	s.logger.Info("Successfully created instance with tasks", "lark_instance_id", larkInstanceID)
	return nil
}

// mapLarkTaskStatus maps Lark task status to internal status
func mapLarkTaskStatus(larkStatus string) string {
	switch larkStatus {
	case "PENDING":
		return entity.TaskStatusPending
	case "IN_PROGRESS":
		return entity.TaskStatusInProgress
	case "COMPLETED", "APPROVED":
		return entity.TaskStatusCompleted
	case "REJECTED":
		return entity.TaskStatusRejected
	default:
		return entity.TaskStatusPending
	}
}

// formatUnixTime converts Unix timestamp to ISO 8601 string
func formatUnixTime(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).Format(time.RFC3339)
}

// marshalJSON is a helper to convert data to JSON string
func marshalJSON(data interface{}) string {
	if data == nil {
		return ""
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(b)
}
