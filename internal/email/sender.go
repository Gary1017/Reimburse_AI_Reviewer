package email

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"go.uber.org/zap"
)

// Sender handles sending vouchers via email
type Sender struct {
	messageAPI  *lark.MessageAPI
	voucherRepo *repository.VoucherRepository
	logger      *zap.Logger
}

// NewSender creates a new email sender
func NewSender(
	messageAPI *lark.MessageAPI,
	voucherRepo *repository.VoucherRepository,
	logger *zap.Logger,
) *Sender {
	return &Sender{
		messageAPI:  messageAPI,
		voucherRepo: voucherRepo,
		logger:      logger,
	}
}

// SendVoucherEmail sends a voucher email to the accountant
func (s *Sender) SendVoucherEmail(ctx context.Context, voucher *models.GeneratedVoucher, instance *models.ApprovalInstance, attachmentPaths []string) error {
	s.logger.Info("Sending voucher email",
		zap.Int64("voucher_id", voucher.ID),
		zap.String("voucher_number", voucher.VoucherNumber),
		zap.String("accountant_email", voucher.AccountantEmail))

	// Build email subject
	subject := fmt.Sprintf("报销凭证 - %s - %s", voucher.VoucherNumber, instance.ApplicantUserID)

	// Build email body
	body := s.buildEmailBody(voucher, instance, attachmentPaths)

	// Prepare attachment list (voucher + supporting documents)
	allAttachments := append([]string{voucher.FilePath}, attachmentPaths...)

	// Send email via Lark message API
	messageID, err := s.messageAPI.SendEmailWithAttachment(
		ctx,
		voucher.AccountantEmail,
		subject,
		body,
		allAttachments,
	)
	if err != nil {
		s.logger.Error("Failed to send email",
			zap.String("voucher_number", voucher.VoucherNumber),
			zap.Error(err))
		return fmt.Errorf("failed to send email: %w", err)
	}

	// Update voucher record with email info
	sendTime := time.Now()
	if err := s.voucherRepo.UpdateEmailSent(nil, voucher.ID, messageID, sendTime); err != nil {
		s.logger.Error("Failed to update voucher email status", zap.Error(err))
		// Don't fail the entire operation
	}

	s.logger.Info("Voucher email sent successfully",
		zap.String("voucher_number", voucher.VoucherNumber),
		zap.String("message_id", messageID))

	return nil
}

// buildEmailBody builds the email body content
func (s *Sender) buildEmailBody(voucher *models.GeneratedVoucher, instance *models.ApprovalInstance, attachments []string) string {
	body := fmt.Sprintf(`尊敬的财务人员，

您好！

请查收报销凭证及相关附件。

**凭证信息：**
- 凭证编号：%s
- 报销人：%s
- 部门：%s
- 提交日期：%s
- 审批日期：%s

**附件清单：**
1. 报销凭证：%s
`,
		voucher.VoucherNumber,
		instance.ApplicantUserID,
		instance.Department,
		instance.SubmissionTime.Format("2006-01-02"),
		formatOptionalTime(instance.ApprovalTime),
		filepath.Base(voucher.FilePath),
	)

	// Add supporting documents
	if len(attachments) > 0 {
		body += "\n**支持性文件：**\n"
		for i, attachment := range attachments {
			body += fmt.Sprintf("%d. %s\n", i+2, filepath.Base(attachment))
		}
	}

	body += `
此邮件由 AI 报销系统自动发送，请勿回复。
如有疑问，请联系相关审批人员。

谢谢！

---
AI Reimbursement Workflow System
`

	return body
}

// formatOptionalTime formats a time pointer
func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return "N/A"
	}
	return t.Format("2006-01-02")
}

// SendNotification sends a notification message to a user
func (s *Sender) SendNotification(ctx context.Context, userID, message string) error {
	_, err := s.messageAPI.SendMessage(ctx, "user_id", userID, "text", fmt.Sprintf(`{"text": "%s"}`, message))
	if err != nil {
		s.logger.Error("Failed to send notification",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to send notification: %w", err)
	}

	s.logger.Info("Notification sent", zap.String("user_id", userID))
	return nil
}
