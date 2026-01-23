package lark

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"go.uber.org/zap"
)

// Messenger implements port.LarkMessageSender interface
type Messenger struct {
	messageAPI *lark.MessageAPI
	logger     *zap.Logger
}

// NewMessenger creates a new Lark message sender adapter
func NewMessenger(larkClient *lark.Client, logger *zap.Logger) *Messenger {
	return &Messenger{
		messageAPI: lark.NewMessageAPI(larkClient, logger),
		logger:     logger,
	}
}

// SendMessage sends a text message to a user
// Implements port.LarkMessageSender interface
func (m *Messenger) SendMessage(ctx context.Context, openID string, content string) error {
	if openID == "" {
		return fmt.Errorf("openID cannot be empty")
	}

	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	// Build text message content in Lark format
	textContent := fmt.Sprintf(`{"text": "%s"}`, escapeJSON(content))

	// Send message using open_id as receive_id_type
	_, err := m.messageAPI.SendMessage(ctx, "open_id", openID, "text", textContent)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return nil
}

// SendCardMessage sends a card message to a user
// Implements port.LarkMessageSender interface
func (m *Messenger) SendCardMessage(ctx context.Context, openID string, cardContent interface{}) error {
	if openID == "" {
		return fmt.Errorf("openID cannot be empty")
	}

	if cardContent == nil {
		return fmt.Errorf("cardContent cannot be nil")
	}

	// Serialize card content to JSON string
	cardJSON, err := json.Marshal(cardContent)
	if err != nil {
		return fmt.Errorf("failed to marshal card content: %w", err)
	}

	// Send card message using open_id as receive_id_type
	_, err = m.messageAPI.SendMessage(ctx, "open_id", openID, "interactive", string(cardJSON))
	if err != nil {
		return fmt.Errorf("failed to send card message: %w", err)
	}

	return nil
}

// escapeJSON escapes special characters in JSON strings
func escapeJSON(s string) string {
	// Simple escape for common cases
	// For production, consider using json.Marshal for proper escaping
	s = replaceAll(s, "\\", "\\\\")
	s = replaceAll(s, "\"", "\\\"")
	s = replaceAll(s, "\n", "\\n")
	s = replaceAll(s, "\r", "\\r")
	s = replaceAll(s, "\t", "\\t")
	return s
}

// replaceAll is a helper function for string replacement
func replaceAll(s, old, new string) string {
	result := ""
	for {
		i := indexOf(s, old)
		if i == -1 {
			result += s
			break
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
	return result
}

// indexOf finds the index of a substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
