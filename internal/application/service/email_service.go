package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
)

type EmailService interface {
	SendVoucherEmail(ctx context.Context, instanceID int64, folderPath string) error
}

type emailServiceImpl struct {
	emailSender     port.LarkEmailSender
	voucherRepo     port.VoucherRepository
	instanceRepo    port.InstanceRepository
	messenger       port.LarkMessageSender
	diagnoser       port.AIErrorDiagnoser
	accountantEmail string
	adminOpenID     string
	retryInterval   time.Duration
	logger          Logger
}

func NewEmailService(
	emailSender port.LarkEmailSender,
	voucherRepo port.VoucherRepository,
	instanceRepo port.InstanceRepository,
	messenger port.LarkMessageSender,
	diagnoser port.AIErrorDiagnoser,
	accountantEmail string,
	adminOpenID string,
	logger Logger,
) EmailService {
	return &emailServiceImpl{
		emailSender:     emailSender,
		voucherRepo:     voucherRepo,
		instanceRepo:    instanceRepo,
		messenger:       messenger,
		diagnoser:       diagnoser,
		accountantEmail: accountantEmail,
		adminOpenID:     adminOpenID,
		retryInterval:   10 * time.Second, // Default 10 seconds
		logger:          logger,
	}
}

// NewEmailServiceWithRetryInterval creates an EmailService with custom retry interval (for testing)
func NewEmailServiceWithRetryInterval(
	emailSender port.LarkEmailSender,
	voucherRepo port.VoucherRepository,
	instanceRepo port.InstanceRepository,
	messenger port.LarkMessageSender,
	diagnoser port.AIErrorDiagnoser,
	accountantEmail string,
	adminOpenID string,
	retryInterval time.Duration,
	logger Logger,
) EmailService {
	return &emailServiceImpl{
		emailSender:     emailSender,
		voucherRepo:     voucherRepo,
		instanceRepo:    instanceRepo,
		messenger:       messenger,
		diagnoser:       diagnoser,
		accountantEmail: accountantEmail,
		adminOpenID:     adminOpenID,
		retryInterval:   retryInterval,
		logger:          logger,
	}
}

func (s *emailServiceImpl) SendVoucherEmail(ctx context.Context, instanceID int64, folderPath string) error {
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("get instance: %w", err)
	}

	// Zip the folder
	zipData, err := zipFolder(folderPath)
	if err != nil {
		return fmt.Errorf("zip folder: %w", err)
	}

	subject := fmt.Sprintf("报销凭证 - %s", instance.LarkInstanceID)
	body := fmt.Sprintf("报销实例 %s 的凭证已生成，请查收附件。", instance.LarkInstanceID)
	zipFilename := filepath.Base(folderPath) + ".zip"

	attachment := port.EmailAttachment{
		Filename: zipFilename,
		Data:     zipData,
	}

	// Retry up to 5 times with 10s interval
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		messageID, sendErr := s.emailSender.SendEmail(ctx, s.accountantEmail, subject, body, []port.EmailAttachment{attachment})
		if sendErr == nil {
			s.updateVoucherEmailInfo(ctx, instanceID, messageID)
			s.logger.Info("Voucher email sent successfully",
				"instance_id", instanceID,
				"message_id", messageID,
				"attempt", attempt,
			)
			return nil
		}
		lastErr = sendErr
		s.logger.Error("Email send failed, retrying",
			"instance_id", instanceID,
			"attempt", attempt,
			"error", sendErr,
		)
		if attempt < 5 {
			time.Sleep(s.retryInterval)
		}
	}

	// All retries failed - diagnose and notify admin
	s.handleEmailFailure(ctx, instanceID, lastErr)
	return fmt.Errorf("email send failed after 5 retries: %w", lastErr)
}

func (s *emailServiceImpl) handleEmailFailure(ctx context.Context, instanceID int64, err error) {
	errorMsg := fmt.Sprintf(
		"Email sending failed for reimbursement instance %d after 5 retries. Error: %s. "+
			"Please describe the possible reasons and suggest solutions.",
		instanceID, err.Error(),
	)

	// Try GPT diagnosis
	diagnosis, diagErr := s.diagnoser.DiagnoseError(ctx, errorMsg)
	if diagErr != nil {
		s.logger.Error("GPT diagnosis failed", "instance_id", instanceID, "error", diagErr)
		// Fallback to simple error message
		diagnosis = fmt.Sprintf("邮件发送失败\n\n实例ID: %d\n错误: %s\n\n请检查:\n1. OAuth token 是否过期\n2. 邮箱地址是否正确\n3. 附件大小是否超过37MB限制\n4. 网络连接是否正常",
			instanceID, err.Error())
	}

	// Send diagnosis to admin via Lark bot message
	notifyErr := s.messenger.SendMessage(ctx, s.adminOpenID, diagnosis)
	if notifyErr != nil {
		s.logger.Error("Failed to notify admin about email failure",
			"instance_id", instanceID,
			"error", notifyErr,
		)
	}
}

func (s *emailServiceImpl) updateVoucherEmailInfo(ctx context.Context, instanceID int64, messageID string) {
	voucher, err := s.voucherRepo.GetByInstanceID(ctx, instanceID)
	if err != nil || voucher == nil {
		s.logger.Error("Failed to get voucher for email update",
			"instance_id", instanceID,
			"error", err,
		)
		return
	}
	now := time.Now()
	voucher.EmailMessageID = messageID
	voucher.SentAt = &now
	voucher.AccountantEmail = s.accountantEmail
	if err := s.voucherRepo.Update(ctx, voucher); err != nil {
		s.logger.Error("Failed to update voucher email info",
			"instance_id", instanceID,
			"error", err,
		)
	}
}

// zipFolder creates a zip archive of the folder contents
func zipFolder(folderPath string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	err := filepath.WalkDir(folderPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(folderPath, path)
		f, createErr := w.Create(relPath)
		if createErr != nil {
			return createErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		_, writeErr := f.Write(data)
		return writeErr
	})
	if err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
