package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"go.uber.org/zap"
)

// AttachmentRepoInterface defines the attachment repository contract for notifier
type AttachmentRepoInterface interface {
	GetProcessedByInstanceID(instanceID int64) ([]*models.Attachment, error)
	GetUnprocessedCountByInstanceID(instanceID int64) (int, error)
	GetTotalCountByInstanceID(instanceID int64) (int, error)
}

// InstanceRepoInterface defines the instance repository contract for notifier
type InstanceRepoInterface interface {
	GetByID(id int64) (*models.ApprovalInstance, error)
}

// AuditNotifier orchestrates the notification flow for audit results
// ARCH-012: AI Audit Result Notification via Lark Approval Bot
type AuditNotifier struct {
	attachmentRepo   AttachmentRepoInterface
	instanceRepo     InstanceRepoInterface
	notificationRepo *repository.NotificationRepository
	approvalAPI      *lark.ApprovalAPI
	approvalBotAPI   *lark.ApprovalBotAPI
	aggregator       *AuditAggregator
	approvalCode     string
	logger           *zap.Logger
}

// NewAuditNotifier creates a new audit notifier
func NewAuditNotifier(
	attachmentRepo AttachmentRepoInterface,
	instanceRepo InstanceRepoInterface,
	notificationRepo *repository.NotificationRepository,
	approvalAPI *lark.ApprovalAPI,
	approvalBotAPI *lark.ApprovalBotAPI,
	aggregator *AuditAggregator,
	approvalCode string,
	logger *zap.Logger,
) *AuditNotifier {
	return &AuditNotifier{
		attachmentRepo:   attachmentRepo,
		instanceRepo:     instanceRepo,
		notificationRepo: notificationRepo,
		approvalAPI:      approvalAPI,
		approvalBotAPI:   approvalBotAPI,
		aggregator:       aggregator,
		approvalCode:     approvalCode,
		logger:           logger,
	}
}

// NotifyApproversOnAuditComplete checks if all attachments are processed
// and sends notification to approvers if ready
func (n *AuditNotifier) NotifyApproversOnAuditComplete(ctx context.Context, instanceID int64) error {
	n.logger.Debug("Checking if instance is fully audited",
		zap.Int64("instance_id", instanceID))

	// Step 1: Check if all attachments are processed
	isReady, err := n.IsInstanceFullyAudited(instanceID)
	if err != nil {
		return fmt.Errorf("failed to check if instance is fully audited: %w", err)
	}

	if !isReady {
		n.logger.Debug("Instance not fully audited yet, skipping notification",
			zap.Int64("instance_id", instanceID))
		return nil
	}

	// Step 2: Check idempotency - has notification already been sent?
	existing, err := n.notificationRepo.GetByInstanceID(instanceID)
	if err != nil {
		return fmt.Errorf("failed to check existing notification: %w", err)
	}

	if existing != nil {
		if existing.Status == models.NotificationStatusSent {
			n.logger.Debug("Notification already sent, skipping",
				zap.Int64("instance_id", instanceID),
				zap.Int64("notification_id", existing.ID))
			return nil
		}
		// If PENDING or FAILED, we'll retry
		n.logger.Info("Retrying notification",
			zap.Int64("instance_id", instanceID),
			zap.String("previous_status", existing.Status))
	}

	// Step 3: Get instance details
	instance, err := n.instanceRepo.GetByID(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance not found: %d", instanceID)
	}

	// Step 4: Get processed attachments and aggregate results
	attachments, err := n.attachmentRepo.GetProcessedByInstanceID(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get processed attachments: %w", err)
	}

	auditResult, err := n.aggregator.Aggregate(attachments, instance)
	if err != nil {
		return fmt.Errorf("failed to aggregate audit results: %w", err)
	}

	// Step 5: Get approvers from Lark
	approvers, err := n.approvalAPI.GetApproversForInstance(ctx, instance.LarkInstanceID)
	if err != nil {
		n.logger.Warn("Failed to get approvers from Lark, will use applicant instead",
			zap.Error(err))
		// Fallback: try to notify the applicant
		approvers = []lark.ApproverInfo{{
			UserID: instance.ApplicantUserID,
		}}
	}

	if len(approvers) == 0 {
		n.logger.Warn("No approvers found for instance",
			zap.Int64("instance_id", instanceID))
		return nil
	}

	// Step 6: Create or update notification record
	violationsJSON, _ := json.Marshal(auditResult.Violations)
	notification := &models.AuditNotification{
		InstanceID:     instanceID,
		LarkInstanceID: instance.LarkInstanceID,
		Status:         models.NotificationStatusPending,
		AuditDecision:  auditResult.Decision,
		Confidence:     auditResult.Confidence,
		TotalAmount:    auditResult.TotalAmount,
		ApproverCount:  len(approvers),
		Violations:     string(violationsJSON),
	}

	if existing == nil {
		if err := n.notificationRepo.Create(nil, notification); err != nil {
			return fmt.Errorf("failed to create notification record: %w", err)
		}
	} else {
		notification.ID = existing.ID
	}

	// Step 7: Send notification to each approver
	var lastErr error
	successCount := 0
	for _, approver := range approvers {
		openID := approver.OpenID
		if openID == "" {
			n.logger.Warn("Approver has no open_id, skipping",
				zap.String("user_id", approver.UserID))
			continue
		}

		req := &models.AuditNotificationRequest{
			ApprovalCode:   n.approvalCode,
			InstanceCode:   instance.LarkInstanceID, // Use lark_instance_id as instance_code
			LarkInstanceID: instance.LarkInstanceID,
			OpenID:         openID,
			AuditResult:    auditResult,
		}

		resp, err := n.approvalBotAPI.SendAuditResultMessage(ctx, req)
		if err != nil {
			n.logger.Error("Failed to send notification",
				zap.String("open_id", openID),
				zap.Error(err))
			lastErr = err
			continue
		}

		if !resp.Success {
			n.logger.Error("Notification send failed",
				zap.String("open_id", openID),
				zap.Int("error_code", resp.ErrorCode),
				zap.String("error_message", resp.ErrorMessage))
			lastErr = fmt.Errorf("notification failed: %s", resp.ErrorMessage)
			continue
		}

		successCount++
		n.logger.Info("Notification sent successfully",
			zap.Int64("instance_id", instanceID),
			zap.String("open_id", openID),
			zap.String("message_id", resp.MessageID))
	}

	// Step 8: Update notification status
	if successCount > 0 {
		if err := n.notificationRepo.UpdateStatus(nil, notification.ID,
			models.NotificationStatusSent, ""); err != nil {
			n.logger.Error("Failed to update notification status to SENT",
				zap.Error(err))
		}
		n.logger.Info("Audit notification completed",
			zap.Int64("instance_id", instanceID),
			zap.Int("success_count", successCount),
			zap.Int("total_approvers", len(approvers)))
	} else if lastErr != nil {
		errMsg := lastErr.Error()
		if err := n.notificationRepo.UpdateStatus(nil, notification.ID,
			models.NotificationStatusFailed, errMsg); err != nil {
			n.logger.Error("Failed to update notification status to FAILED",
				zap.Error(err))
		}
		return lastErr
	}

	return nil
}

// IsInstanceFullyAudited checks if all attachments for an instance have been processed
func (n *AuditNotifier) IsInstanceFullyAudited(instanceID int64) (bool, error) {
	totalCount, err := n.attachmentRepo.GetTotalCountByInstanceID(instanceID)
	if err != nil {
		return false, err
	}

	if totalCount == 0 {
		// No attachments, nothing to audit
		return false, nil
	}

	unprocessedCount, err := n.attachmentRepo.GetUnprocessedCountByInstanceID(instanceID)
	if err != nil {
		return false, err
	}

	return unprocessedCount == 0, nil
}

// NotifyApproversOnAuditCompleteAsync is a non-blocking version that logs errors
// Use this from InvoiceProcessor to avoid blocking invoice processing
func (n *AuditNotifier) NotifyApproversOnAuditCompleteAsync(ctx context.Context, instanceID int64) {
	go func() {
		if err := n.NotifyApproversOnAuditComplete(ctx, instanceID); err != nil {
			n.logger.Warn("Failed to send audit notification (non-blocking)",
				zap.Int64("instance_id", instanceID),
				zap.Error(err))
		}
	}()
}

// Ensure sql.Tx is available for repository methods that need it
var _ *sql.Tx = nil
