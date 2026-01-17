package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

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
	// Log entry point - verify requests are reaching this handler
	h.logger.Info("=== WEBHOOK HANDLER CALLED ===",
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("remote_addr", c.Request.RemoteAddr),
		zap.String("user_agent", c.Request.UserAgent()))

	// Read request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	h.logger.Info("Request body read successfully",
		zap.Int("body_size", len(body)),
		zap.String("content_type", c.GetHeader("Content-Type")))

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
	h.logger.Info("Verifying webhook signature",
		zap.String("timestamp", timestamp),
		zap.String("nonce", nonce),
		zap.Bool("has_signature", signature != ""))

	if !h.verifier.VerifySignature(timestamp, nonce, signature, string(body)) {
		h.logger.Warn("Invalid webhook signature - REJECTING REQUEST",
			zap.String("timestamp", timestamp),
			zap.String("nonce", nonce),
			zap.String("signature", signature))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	h.logger.Info("Webhook signature verified successfully")

	// Parse event
	var event ApprovalEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.logger.Error("Failed to parse event JSON",
			zap.Error(err),
			zap.String("body_preview", string(body[:min(len(body), 200)])))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse event"})
		return
	}

	h.logger.Info("Event parsed successfully",
		zap.String("event_id", event.Header.EventID),
		zap.String("event_type", event.Header.EventType),
		zap.String("create_time", event.Header.CreateTime))

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

	// Log event receipt with full details
	h.logger.Info("Received approval event",
		zap.String("event_id", event.Header.EventID),
		zap.String("event_type", event.Header.EventType),
		zap.String("create_time", event.Header.CreateTime),
		zap.Any("event_keys", getEventKeys(event.Event)))

	// Process event asynchronously to respond quickly to Lark
	h.logger.Info("Starting async event processing",
		zap.String("event_id", event.Header.EventID),
		zap.String("event_type", event.Header.EventType))
	go h.processEvent(&event)

	// Respond immediately to Lark
	h.logger.Info("Responding to Lark webhook",
		zap.String("event_id", event.Header.EventID),
		zap.String("status", "200 OK"))
	c.JSON(http.StatusOK, gin.H{"message": "Event received"})
}

// processEvent processes the approval event
func (h *Handler) processEvent(event *ApprovalEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("Panic in event processing",
				zap.Any("panic", r),
				zap.String("event_id", event.Header.EventID),
				zap.String("event_type", event.Header.EventType))
		}
	}()

	h.logger.Info("=== ASYNC EVENT PROCESSING STARTED ===",
		zap.String("event_id", event.Header.EventID),
		zap.String("event_type", event.Header.EventType),
		zap.Any("event_keys", getEventKeys(event.Event)))

	// Extract instance ID from event
	instanceID, ok := event.Event["instance_code"].(string)
	if !ok {
		// Try alternative field names
		if id, ok := event.Event["instance_id"].(string); ok {
			instanceID = id
			h.logger.Info("Found instance_id instead of instance_code", zap.String("instance_id", instanceID))
		} else {
			h.logger.Error("Instance ID not found in event - checking available keys",
				zap.Any("event_keys", getEventKeys(event.Event)),
				zap.Any("event_data", event.Event))
			return
		}
	}

	h.logger.Info("Extracted instance ID",
		zap.String("instance_id", instanceID))

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
		h.logger.Info("Processing status changed event",
			zap.String("instance_id", instanceID),
			zap.String("event_type", eventType),
			zap.Any("event_data", event.Event))
		if err := h.workflowEngine.HandleStatusChanged(instanceID, event.Event); err != nil {
			h.logger.Error("Failed to handle status changed",
				zap.String("instance_id", instanceID),
				zap.String("event_type", eventType),
				zap.Error(err))
		} else {
			h.logger.Info("Status changed event processed successfully",
				zap.String("instance_id", instanceID))
		}

	default:
		h.logger.Warn("Unhandled event type - status change may be in this event",
			zap.String("event_type", eventType),
			zap.String("instance_id", instanceID),
			zap.Any("event_data", event.Event))
		
		// Try to handle as status change if it contains status information
		if status, ok := event.Event["status"].(string); ok {
			h.logger.Info("Found status in unhandled event, attempting to process as status change",
				zap.String("event_type", eventType),
				zap.String("status", status))
			if err := h.workflowEngine.HandleStatusChanged(instanceID, event.Event); err != nil {
				h.logger.Error("Failed to handle unhandled event as status change",
					zap.String("instance_id", instanceID),
					zap.String("event_type", eventType),
					zap.Error(err))
			}
		}
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// getEventKeys returns all keys from event map (helper for logging)
func getEventKeys(event map[string]interface{}) []string {
	keys := make([]string, 0, len(event))
	for k := range event {
		keys = append(keys, k)
	}
	return keys
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
