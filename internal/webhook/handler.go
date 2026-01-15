package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/garyjia/ai-reimbursement/internal/workflow"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler handles webhook requests
type Handler struct {
	verifier       *Verifier
	workflowEngine *workflow.Engine
	approvalCode   string
	logger         *zap.Logger
}

// NewHandler creates a new webhook handler
func NewHandler(verifier *Verifier, workflowEngine *workflow.Engine, approvalCode string, logger *zap.Logger) *Handler {
	return &Handler{
		verifier:       verifier,
		workflowEngine: workflowEngine,
		approvalCode:   approvalCode,
		logger:         logger,
	}
}

// ApprovalEvent represents a Lark approval event
type ApprovalEvent struct {
	Schema string                 `json:"schema"`
	Header EventHeader            `json:"header"`
	Event  map[string]interface{} `json:"event"`
}

// EventHeader contains event metadata
type EventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	CreateTime string `json:"create_time"`
	Token      string `json:"token"`
	AppID      string `json:"app_id"`
	TenantKey  string `json:"tenant_key"`
}

// Handle processes incoming webhook requests
func (h *Handler) Handle(c *gin.Context) {
	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	// Get headers for verification
	timestamp := c.GetHeader("X-Lark-Request-Timestamp")
	nonce := c.GetHeader("X-Lark-Request-Nonce")
	signature := c.GetHeader("X-Lark-Signature")

	// Check if this is a challenge request
	var challengeCheck struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &challengeCheck); err == nil && challengeCheck.Type == "url_verification" {
		challenge, err := h.verifier.VerifyChallenge(body)
		if err != nil {
			h.logger.Error("Challenge verification failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "Challenge verification failed"})
			return
		}

		h.logger.Info("Challenge verified successfully")
		c.JSON(http.StatusOK, gin.H{"challenge": challenge})
		return
	}

	// Verify webhook signature
	if !h.verifier.VerifySignature(timestamp, nonce, signature, string(body)) {
		h.logger.Warn("Invalid webhook signature",
			zap.String("timestamp", timestamp),
			zap.String("nonce", nonce))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse event
	var event ApprovalEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error("Failed to parse event", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse event"})
		return
	}

	// Filter by approval code if provided
	if h.approvalCode != "" {
		if approvalCode, ok := event.Event["approval_code"].(string); ok {
			if approvalCode != h.approvalCode {
				h.logger.Info("Ignoring event for different approval code",
					zap.String("event_approval_code", approvalCode))
				c.JSON(http.StatusOK, gin.H{"message": "Event ignored"})
				return
			}
		}
	}

	// Validate event type
	if !h.verifier.ValidateEventType(event.Header.EventType) {
		h.logger.Warn("Invalid event type", zap.String("event_type", event.Header.EventType))
		c.JSON(http.StatusOK, gin.H{"message": "Event type not supported"})
		return
	}

	// Log event receipt
	h.logger.Info("Received approval event",
		zap.String("event_id", event.Header.EventID),
		zap.String("event_type", event.Header.EventType))

	// Process event asynchronously to respond quickly to Lark
	go h.processEvent(&event)

	// Respond immediately to Lark
	c.JSON(http.StatusOK, gin.H{"message": "Event received"})
}

// processEvent processes the approval event
func (h *Handler) processEvent(event *ApprovalEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in event processing", zap.Any("panic", r))
		}
	}()

	// Extract instance ID from event
	instanceID, ok := event.Event["instance_code"].(string)
	if !ok {
		h.logger.Error("Instance ID not found in event")
		return
	}

	// Determine event type and handle accordingly
	eventType := event.Header.EventType

	switch {
	case contains(eventType, "created"):
		h.logger.Info("Processing instance created event", zap.String("instance_id", instanceID))
		if err := h.workflowEngine.HandleInstanceCreated(instanceID, event.Event); err != nil {
			h.logger.Error("Failed to handle instance created", zap.String("instance_id", instanceID), zap.Error(err))
		}

	case contains(eventType, "approved"):
		h.logger.Info("Processing instance approved event", zap.String("instance_id", instanceID))
		if err := h.workflowEngine.HandleInstanceApproved(instanceID, event.Event); err != nil {
			h.logger.Error("Failed to handle instance approved", zap.String("instance_id", instanceID), zap.Error(err))
		}

	case contains(eventType, "rejected"):
		h.logger.Info("Processing instance rejected event", zap.String("instance_id", instanceID))
		if err := h.workflowEngine.HandleInstanceRejected(instanceID, event.Event); err != nil {
			h.logger.Error("Failed to handle instance rejected", zap.String("instance_id", instanceID), zap.Error(err))
		}

	case contains(eventType, "status_changed"):
		h.logger.Info("Processing status changed event", zap.String("instance_id", instanceID))
		if err := h.workflowEngine.HandleStatusChanged(instanceID, event.Event); err != nil {
			h.logger.Error("Failed to handle status changed", zap.String("instance_id", instanceID), zap.Error(err))
		}

	default:
		h.logger.Info("Unhandled event type", zap.String("event_type", eventType))
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		 fmt.Sprintf("%s", s) != s)) // simplified check
}
