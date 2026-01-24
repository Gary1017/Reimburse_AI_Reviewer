// Package websocket provides WebSocket adapters for external event sources.
// This package translates protocol-specific events into domain events.
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"
)

// LarkAdapter wraps the Lark WebSocket SDK client and translates
// Lark approval events into domain events for the application dispatcher.
type LarkAdapter struct {
	appID        string
	appSecret    string
	approvalCode string
	dispatcher   dispatcher.Dispatcher
	logger       *zap.Logger

	wsClient *larkws.Client
	mu       sync.RWMutex
	started  bool
}

// LarkAdapterConfig holds configuration for the Lark WebSocket adapter.
type LarkAdapterConfig struct {
	AppID        string
	AppSecret    string
	ApprovalCode string // Optional: filter events by approval code
}

// NewLarkAdapter creates a new Lark WebSocket adapter.
// It accepts app credentials and the application dispatcher to publish domain events.
func NewLarkAdapter(cfg LarkAdapterConfig, d dispatcher.Dispatcher, logger *zap.Logger) *LarkAdapter {
	return &LarkAdapter{
		appID:        cfg.AppID,
		appSecret:    cfg.AppSecret,
		approvalCode: cfg.ApprovalCode,
		dispatcher:   d,
		logger:       logger,
	}
}

// Start initializes the WebSocket connection and begins listening for Lark events.
// This method blocks until the context is cancelled or an error occurs.
func (a *LarkAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return fmt.Errorf("adapter already started")
	}

	// Create Lark SDK event dispatcher
	// Empty strings for verification token and encrypt key (not needed for WebSocket mode)
	sdkDispatcher := larkdispatcher.NewEventDispatcher("", "")

	// Register handler for approval_instance events
	sdkDispatcher.OnCustomizedEvent("approval_instance", a.handleLarkEvent)

	// Create WebSocket client with event dispatcher
	a.wsClient = larkws.NewClient(
		a.appID,
		a.appSecret,
		larkws.WithEventHandler(sdkDispatcher),
	)

	a.started = true
	a.mu.Unlock()

	a.logger.Info("Starting Lark WebSocket adapter",
		zap.String("app_id", a.appID),
		zap.String("approval_code", a.approvalCode))

	// Start blocks until context is cancelled or error occurs
	if err := a.wsClient.Start(ctx); err != nil {
		a.logger.Error("Lark WebSocket client error", zap.Error(err))
		return fmt.Errorf("websocket client error: %w", err)
	}

	return nil
}

// Stop gracefully stops the WebSocket adapter.
// Note: The Lark SDK WebSocket client is stopped by cancelling the context passed to Start.
func (a *LarkAdapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started {
		return nil
	}

	a.started = false
	a.logger.Info("Lark WebSocket adapter stopped")
	return nil
}

// IsRunning returns whether the adapter is currently running.
func (a *LarkAdapter) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.started
}

// larkApprovalEvent represents the structure of a Lark approval event payload.
type larkApprovalEvent struct {
	Header struct {
		EventType string `json:"event_type"`
	} `json:"header"`
	Event struct {
		InstanceCode string `json:"instance_code"`
		ApprovalCode string `json:"approval_code"`
		Status       string `json:"status"`
		Comment      string `json:"comment"`
		OperateTime  string `json:"operate_time"`
		UserID       string `json:"user_id"`
	} `json:"event"`
}

// handleLarkEvent is called by the Lark SDK when an approval event is received.
// It translates the Lark event into a domain event and dispatches it.
func (a *LarkAdapter) handleLarkEvent(ctx context.Context, evt *larkevent.EventReq) error {
	a.logger.Debug("Received Lark event",
		zap.Int("body_length", len(evt.Body)))

	// Parse the event payload
	var larkEvent larkApprovalEvent
	if err := json.Unmarshal(evt.Body, &larkEvent); err != nil {
		a.logger.Error("Failed to parse Lark event payload",
			zap.Error(err),
			zap.String("body", string(evt.Body)))
		return fmt.Errorf("failed to parse event payload: %w", err)
	}

	// Filter by approval code if configured
	if a.approvalCode != "" && larkEvent.Event.ApprovalCode != "" {
		if larkEvent.Event.ApprovalCode != a.approvalCode {
			a.logger.Debug("Ignoring event for different approval code",
				zap.String("event_approval_code", larkEvent.Event.ApprovalCode),
				zap.String("configured_approval_code", a.approvalCode))
			return nil
		}
	}

	instanceCode := larkEvent.Event.InstanceCode
	if instanceCode == "" {
		a.logger.Warn("Instance code not found in Lark event",
			zap.String("event_type", larkEvent.Header.EventType))
		return nil
	}

	// Translate Lark event to domain event
	domainEvent := a.translateToDomainEvent(larkEvent)
	if domainEvent == nil {
		a.logger.Debug("Event type not translated to domain event",
			zap.String("lark_event_type", larkEvent.Header.EventType))
		return nil
	}

	// Dispatch to application dispatcher
	if err := a.dispatcher.Dispatch(ctx, domainEvent); err != nil {
		a.logger.Error("Failed to dispatch domain event",
			zap.Error(err),
			zap.String("event_type", domainEvent.Type.String()),
			zap.String("instance_code", instanceCode))
		return fmt.Errorf("failed to dispatch event: %w", err)
	}

	a.logger.Info("Domain event dispatched",
		zap.String("event_type", domainEvent.Type.String()),
		zap.String("event_id", domainEvent.ID),
		zap.String("instance_code", instanceCode))

	return nil
}

// translateToDomainEvent converts a Lark approval event to a domain event.
// Returns nil if the event type should not be translated.
func (a *LarkAdapter) translateToDomainEvent(larkEvent larkApprovalEvent) *event.Event {
	eventType := strings.ToLower(larkEvent.Header.EventType)
	status := strings.ToUpper(larkEvent.Event.Status)

	// Build payload from Lark event data
	payload := map[string]interface{}{
		"lark_event_type": larkEvent.Header.EventType,
		"approval_code":   larkEvent.Event.ApprovalCode,
		"status":          larkEvent.Event.Status,
	}

	if larkEvent.Event.Comment != "" {
		payload["comment"] = larkEvent.Event.Comment
	}
	if larkEvent.Event.OperateTime != "" {
		payload["operate_time"] = larkEvent.Event.OperateTime
	}
	if larkEvent.Event.UserID != "" {
		payload["user_id"] = larkEvent.Event.UserID
	}

	// Determine domain event type based on Lark event
	var domainEventType event.Type

	switch {
	case strings.Contains(eventType, "created"):
		domainEventType = event.TypeInstanceCreated

	case strings.Contains(eventType, "approved") || status == "APPROVED":
		domainEventType = event.TypeInstanceApproved

	case strings.Contains(eventType, "rejected") || status == "REJECTED":
		domainEventType = event.TypeInstanceRejected

	case strings.Contains(eventType, "status_changed"):
		domainEventType = event.TypeStatusChanged

	default:
		// Check if status field indicates a meaningful event
		if status == "APPROVED" {
			domainEventType = event.TypeInstanceApproved
		} else if status == "REJECTED" {
			domainEventType = event.TypeInstanceRejected
		} else if status != "" {
			// Generic status change
			domainEventType = event.TypeStatusChanged
		} else {
			// Unknown event type, don't translate
			return nil
		}
	}

	// Create domain event
	// Note: InstanceID (int64) is not available from Lark events directly,
	// it will be looked up by the handler using LarkInstanceID
	return event.NewEvent(
		domainEventType,
		0, // InstanceID will be resolved by handler
		larkEvent.Event.InstanceCode,
		payload,
	)
}
