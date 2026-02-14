package lark

import (
	"context"
	"encoding/base64"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkmail "github.com/larksuite/oapi-sdk-go/v3/service/mail/v1"
	"go.uber.org/zap"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
)

// EmailSender sends emails via Lark Mail API
type EmailSender struct {
	client       *lark.Client
	oauthManager *OAuthManager
	senderUserID string // AI Approver's user_id (to get access token)
	senderEmail  string // AI Approver's email (used as user_mailbox_id)
	logger       *zap.Logger
}

// NewEmailSender creates a new email sender
func NewEmailSender(client *lark.Client, oauthManager *OAuthManager, senderUserID string, senderEmail string, logger *zap.Logger) *EmailSender {
	return &EmailSender{
		client:       client,
		oauthManager: oauthManager,
		senderUserID: senderUserID,
		senderEmail:  senderEmail,
		logger:       logger,
	}
}

// SendEmail sends an email via Lark Mail API
func (s *EmailSender) SendEmail(ctx context.Context, to string, subject string, bodyText string, attachments []port.EmailAttachment) (messageID string, err error) {
	s.logger.Info("Sending email via Lark Mail API",
		zap.String("to", to),
		zap.String("subject", subject),
		zap.Int("attachment_count", len(attachments)))

	// Step 1: Get user access token
	accessToken, err := s.oauthManager.GetAccessToken(ctx, s.senderUserID)
	if err != nil {
		s.logger.Error("Failed to get access token",
			zap.String("user_id", s.senderUserID),
			zap.Error(err))
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Step 2: Build Lark mail attachments (base64url-encode)
	var mailAttachments []*larkmail.Attachment
	for _, att := range attachments {
		encoded := base64.URLEncoding.EncodeToString(att.Data)
		mailAttachments = append(mailAttachments, &larkmail.Attachment{
			Filename: &att.Filename,
			Body:     &encoded,
		})
	}

	// Step 3: Build recipients
	recipients := []*larkmail.MailAddress{
		{
			MailAddress: &to,
			Name:        &to,
		},
	}

	// Step 4: Build request
	req := larkmail.NewSendUserMailboxMessageReqBuilder().
		UserMailboxId(s.senderEmail).
		Body(larkmail.NewSendUserMailboxMessageReqBodyBuilder().
			Subject(subject).
			To(recipients).
			BodyPlainText(bodyText).
			Attachments(mailAttachments).
			Build()).
		Build()

	// Step 5: Send with user access token
	resp, err := s.client.Mail.UserMailboxMessage.Send(ctx, req, larkcore.WithUserAccessToken(accessToken))
	if err != nil {
		s.logger.Error("Failed to send email",
			zap.String("to", to),
			zap.Error(err))
		return "", fmt.Errorf("failed to send email: %w", err)
	}

	// Step 6: Check response
	if !resp.Success() {
		s.logger.Error("Lark API returned error",
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return "", fmt.Errorf("lark API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// Step 7: Extract message ID
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", fmt.Errorf("empty message_id in response")
	}

	messageID = *resp.Data.MessageId

	s.logger.Info("Email sent successfully",
		zap.String("to", to),
		zap.String("message_id", messageID))

	return messageID, nil
}

// Verify interface compliance
var _ port.LarkEmailSender = (*EmailSender)(nil)
