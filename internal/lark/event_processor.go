package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"go.uber.org/zap"
)

// WorkflowHandler defines the workflow operations triggered by Lark events.
type WorkflowHandler interface {
	HandleInstanceCreated(instanceID string, eventData map[string]interface{}) error
	HandleInstanceApproved(instanceID string, eventData map[string]interface{}) error
	HandleInstanceRejected(instanceID string, eventData map[string]interface{}) error
	HandleStatusChanged(instanceID string, eventData map[string]interface{}) error
}

// ApprovalEvent represents a Lark approval event payload.
type ApprovalEvent struct {
	Header EventHeader            `json:"header"`
	Event  map[string]interface{} `json:"event"`
}

// EventHeader contains event metadata.
type EventHeader struct {
	EventType string `json:"event_type"`
}

// EventProcessor routes Lark events into workflow handlers.
type EventProcessor struct {
	approvalCode string
	handler      WorkflowHandler
	logger       *zap.Logger
}

// NewEventProcessor creates a new EventProcessor.
func NewEventProcessor(approvalCode string, handler WorkflowHandler, logger *zap.Logger) *EventProcessor {
	return &EventProcessor{
		approvalCode: approvalCode,
		handler:      handler,
		logger:       logger,
	}
}

// SetWorkflowHandler sets the workflow handler (used for late binding)
func (p *EventProcessor) SetWorkflowHandler(handler WorkflowHandler) {
	p.handler = handler
	p.logger.Info("Workflow handler set for event processor")
}

// HandleCustomizedEvent adapts the SDK event payload for processing.
func (p *EventProcessor) HandleCustomizedEvent(ctx context.Context, event *larkevent.EventReq) error {
	return p.ProcessEvent(ctx, event.Body)
}

// ProcessEvent parses and dispatches an approval event payload.
func (p *EventProcessor) ProcessEvent(ctx context.Context, payload []byte) error {
	var event ApprovalEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse approval event payload: %w", err)
	}

	if event.Event == nil {
		p.logger.Warn("Approval event payload missing event data")
		return nil
	}

	approvalCode, _ := event.Event["approval_code"].(string)
	if p.approvalCode != "" && approvalCode != "" && approvalCode != p.approvalCode {
		p.logger.Info("Ignoring approval event for different approval code",
			zap.String("event_type", event.Header.EventType),
			zap.String("approval_code", approvalCode))
		return nil
	}

	instanceID, ok := event.Event["instance_code"].(string)
	if !ok || instanceID == "" {
		p.logger.Warn("Instance ID not found in approval event",
			zap.String("event_type", event.Header.EventType))
		return nil
	}

	eventType := strings.ToLower(event.Header.EventType)
	switch {
	case strings.Contains(eventType, "created"):
		return p.handler.HandleInstanceCreated(instanceID, event.Event)
	case strings.Contains(eventType, "approved"):
		return p.handler.HandleInstanceApproved(instanceID, event.Event)
	case strings.Contains(eventType, "rejected"):
		return p.handler.HandleInstanceRejected(instanceID, event.Event)
	case strings.Contains(eventType, "status_changed"):
		return p.handler.HandleStatusChanged(instanceID, event.Event)
	default:
		if _, ok := event.Event["status"]; ok {
			return p.handler.HandleStatusChanged(instanceID, event.Event)
		}
		p.logger.Info("Unhandled approval event type",
			zap.String("event_type", event.Header.EventType))
		return nil
	}
}
