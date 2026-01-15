package lark

import (
	"context"
	"fmt"

	larkIm "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"go.uber.org/zap"
)

// MessageAPI handles Lark messaging operations
type MessageAPI struct {
	client *Client
	logger *zap.Logger
}

// NewMessageAPI creates a new message API handler
func NewMessageAPI(client *Client, logger *zap.Logger) *MessageAPI {
	return &MessageAPI{
		client: client,
		logger: logger,
	}
}

// SendMessage sends a message to a user or group
func (m *MessageAPI) SendMessage(ctx context.Context, receiveIDType, receiveID, msgType, content string) (string, error) {
	req := larkIm.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIDType).
		Body(larkIm.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()

	resp, err := m.client.client.Im.Message.Create(ctx, req)
	if err != nil {
		m.logger.Error("Failed to send message",
			zap.String("receive_id", receiveID),
			zap.Error(err))
		return "", fmt.Errorf("failed to send message: %w", err)
	}

	if !resp.Success() {
		m.logger.Error("API returned failure",
			zap.String("receive_id", receiveID),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return "", fmt.Errorf("API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	messageID := ""
	if resp.Data != nil && resp.Data.MessageId != nil {
		messageID = *resp.Data.MessageId
	}

	m.logger.Info("Message sent successfully",
		zap.String("message_id", messageID),
		zap.String("receive_id", receiveID))

	return messageID, nil
}

// SendEmailWithAttachment sends an email-style message with attachment references
func (m *MessageAPI) SendEmailWithAttachment(ctx context.Context, email, subject, body string, attachments []string) (string, error) {
	// Build message content
	content := fmt.Sprintf(`{
		"email": "%s",
		"title": "%s",
		"content": [[{"tag": "text", "text": "%s"}]]
	}`, email, subject, body)

	// Note: For actual file attachments, we need to upload them first and reference the file_key
	// This is a simplified implementation
	m.logger.Info("Sending email message",
		zap.String("email", email),
		zap.String("subject", subject),
		zap.Int("attachments", len(attachments)))

	// Use "email" as receive_id_type for sending to external email
	// Note: This requires Lark admin configuration for external email sending
	return m.SendMessage(ctx, "email", email, "post", content)
}

// UploadFile uploads a file to Lark
func (m *MessageAPI) UploadFile(ctx context.Context, fileName string, fileData []byte) (string, error) {
	// TODO: Implement file upload using drive API
	m.logger.Warn("File upload not yet implemented", zap.String("file_name", fileName))
	return "", fmt.Errorf("file upload not implemented")
}
