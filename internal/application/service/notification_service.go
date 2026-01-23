package service

import (
	"context"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// NotificationService manages notifications
type NotificationService interface {
	NotifyApplicant(ctx context.Context, instanceID int64, message string) error
	NotifyAuditResult(ctx context.Context, instanceID int64, result *AuditResult) error
	NotifyVoucherReady(ctx context.Context, instanceID int64, voucherPath string) error
}

type notificationServiceImpl struct {
	instanceRepo     port.InstanceRepository
	notificationRepo port.NotificationRepository
	larkClient       port.LarkClient
	messageSender    port.LarkMessageSender
	txManager        port.TransactionManager
	logger           Logger
}

// NewNotificationService creates a new NotificationService
func NewNotificationService(
	instanceRepo port.InstanceRepository,
	notificationRepo port.NotificationRepository,
	larkClient port.LarkClient,
	messageSender port.LarkMessageSender,
	txManager port.TransactionManager,
	logger Logger,
) NotificationService {
	return &notificationServiceImpl{
		instanceRepo:     instanceRepo,
		notificationRepo: notificationRepo,
		larkClient:       larkClient,
		messageSender:    messageSender,
		txManager:        txManager,
		logger:           logger,
	}
}

// NotifyApplicant sends a generic notification to the applicant
func (s *notificationServiceImpl) NotifyApplicant(ctx context.Context, instanceID int64, message string) error {
	s.logger.Info("Sending notification to applicant", "instance_id", instanceID)

	// Get instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get instance", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get instance: %w", err)
	}

	// Get applicant OpenID from Lark
	detail, err := s.larkClient.GetInstanceDetail(ctx, instance.LarkInstanceID)
	if err != nil {
		s.logger.Error("Failed to get instance detail from Lark", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get instance detail: %w", err)
	}

	// Send message via Lark
	err = s.messageSender.SendMessage(ctx, detail.OpenID, message)
	if err != nil {
		s.logger.Error("Failed to send message", "error", err, "instance_id", instanceID, "open_id", detail.OpenID)
		return fmt.Errorf("send message: %w", err)
	}

	s.logger.Info("Notification sent successfully",
		"instance_id", instanceID,
		"open_id", detail.OpenID,
		"message_length", len(message),
	)

	return nil
}

// NotifyAuditResult sends audit result notification to the applicant
func (s *notificationServiceImpl) NotifyAuditResult(ctx context.Context, instanceID int64, result *AuditResult) error {
	s.logger.Info("Sending audit result notification", "instance_id", instanceID, "overall_pass", result.OverallPass)

	// Get instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get instance", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get instance: %w", err)
	}

	// Get applicant OpenID from Lark
	detail, err := s.larkClient.GetInstanceDetail(ctx, instance.LarkInstanceID)
	if err != nil {
		s.logger.Error("Failed to get instance detail from Lark", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get instance detail: %w", err)
	}

	// Build message
	message := s.buildAuditMessage(result)

	// Create notification record
	notification := &entity.AuditNotification{
		InstanceID:     instanceID,
		LarkInstanceID: instance.LarkInstanceID,
		Status:         "PENDING",
		AuditDecision:  determineDecision(result),
		Confidence:     result.Confidence,
		Violations:     buildViolationsString(result),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Send message and save notification in transaction
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Save notification record
		if err := s.notificationRepo.Create(txCtx, notification); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}

		// Send message via Lark
		if err := s.messageSender.SendMessage(txCtx, detail.OpenID, message); err != nil {
			// Mark as failed
			s.notificationRepo.UpdateStatus(txCtx, notification.ID, "FAILED", err.Error())
			return fmt.Errorf("send message: %w", err)
		}

		// Mark as sent
		if err := s.notificationRepo.MarkSent(txCtx, notification.ID); err != nil {
			return fmt.Errorf("mark notification sent: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to send audit result notification", "error", err, "instance_id", instanceID)
		return err
	}

	s.logger.Info("Audit result notification sent successfully",
		"instance_id", instanceID,
		"notification_id", notification.ID,
		"open_id", detail.OpenID,
	)

	return nil
}

// NotifyVoucherReady sends voucher ready notification to the applicant
func (s *notificationServiceImpl) NotifyVoucherReady(ctx context.Context, instanceID int64, voucherPath string) error {
	s.logger.Info("Sending voucher ready notification", "instance_id", instanceID, "voucher_path", voucherPath)

	// Get instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get instance", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get instance: %w", err)
	}

	// Get applicant OpenID from Lark
	detail, err := s.larkClient.GetInstanceDetail(ctx, instance.LarkInstanceID)
	if err != nil {
		s.logger.Error("Failed to get instance detail from Lark", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get instance detail: %w", err)
	}

	// Build message
	message := fmt.Sprintf(
		"您的报销凭证已生成完成！\n\n实例ID: %d\n凭证路径: %s\n\n请联系财务部门获取凭证文件。",
		instanceID,
		voucherPath,
	)

	// Create notification record
	notification := &entity.AuditNotification{
		InstanceID:     instanceID,
		LarkInstanceID: instance.LarkInstanceID,
		Status:         "PENDING",
		AuditDecision:  "VOUCHER_READY",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Send message and save notification in transaction
	err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Save notification record
		if err := s.notificationRepo.Create(txCtx, notification); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}

		// Send message via Lark
		if err := s.messageSender.SendMessage(txCtx, detail.OpenID, message); err != nil {
			// Mark as failed
			s.notificationRepo.UpdateStatus(txCtx, notification.ID, "FAILED", err.Error())
			return fmt.Errorf("send message: %w", err)
		}

		// Mark as sent
		if err := s.notificationRepo.MarkSent(txCtx, notification.ID); err != nil {
			return fmt.Errorf("mark notification sent: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to send voucher ready notification", "error", err, "instance_id", instanceID)
		return err
	}

	s.logger.Info("Voucher ready notification sent successfully",
		"instance_id", instanceID,
		"notification_id", notification.ID,
		"open_id", detail.OpenID,
	)

	return nil
}

// buildAuditMessage builds a human-readable audit message
func (s *notificationServiceImpl) buildAuditMessage(result *AuditResult) string {
	if result.OverallPass {
		return fmt.Sprintf(
			"您的报销申请已通过AI审核！\n\n审核结果: 通过 ✓\n置信度: %.2f%%\n\n%s\n\n您的申请将继续进入审批流程。",
			result.Confidence*100,
			result.Reasoning,
		)
	}

	// Build violation details
	violations := ""
	if result.PolicyResult != nil && len(result.PolicyResult.Violations) > 0 {
		violations = "\n政策违规项:\n"
		for i, v := range result.PolicyResult.Violations {
			violations += fmt.Sprintf("  %d. %s\n", i+1, v)
		}
	}

	return fmt.Sprintf(
		"您的报销申请未通过AI审核。\n\n审核结果: 未通过 ✗\n置信度: %.2f%%\n\n%s%s\n\n请修改后重新提交申请。",
		result.Confidence*100,
		result.Reasoning,
		violations,
	)
}

// determineDecision converts AuditResult to decision string
func determineDecision(result *AuditResult) string {
	if result.OverallPass {
		return "APPROVED"
	}
	return "REJECTED"
}

// buildViolationsString converts violations to JSON string
func buildViolationsString(result *AuditResult) string {
	if result.PolicyResult == nil || len(result.PolicyResult.Violations) == 0 {
		return ""
	}
	// Simple implementation - in production, use json.Marshal
	violations := ""
	for _, v := range result.PolicyResult.Violations {
		violations += v + ";"
	}
	return violations
}
