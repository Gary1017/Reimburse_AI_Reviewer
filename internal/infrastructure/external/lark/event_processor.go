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

// TaskWorkflowHandler defines the workflow operations triggered by Lark task events.
type TaskWorkflowHandler interface {
	HandleTaskCreated(instanceID string, taskID string, eventData map[string]interface{}) error
	HandleTaskStatusChanged(instanceID string, taskID string, eventData map[string]interface{}) error
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
	taskHandler  TaskWorkflowHandler
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

// SetTaskWorkflowHandler sets the task workflow handler (used for late binding)
func (p *EventProcessor) SetTaskWorkflowHandler(handler TaskWorkflowHandler) {
	p.taskHandler = handler
	p.logger.Info("Task workflow handler set for event processor")
}

// HandleCustomizedEvent adapts the SDK event payload for processing.
func (p *EventProcessor) HandleCustomizedEvent(ctx context.Context, event *larkevent.EventReq) error {
	return p.ProcessEvent(ctx, event.Body)
}

// HandleTaskEvent adapts the SDK task event payload for processing.
func (p *EventProcessor) HandleTaskEvent(ctx context.Context, event *larkevent.EventReq) error {
	return p.ProcessTaskEvent(ctx, event.Body)
}

// ProcessTaskEvent parses and dispatches an approval task event payload.
func (p *EventProcessor) ProcessTaskEvent(ctx context.Context, payload []byte) error {
	var evt ApprovalEvent
	if err := json.Unmarshal(payload, &evt); err != nil {
		return fmt.Errorf("failed to parse approval task event payload: %w", err)
	}

	if evt.Event == nil {
		p.logger.Warn("Approval task event payload missing event data")
		return nil
	}

	// Filter by approval code
	approvalCode, _ := evt.Event["approval_code"].(string)
	if p.approvalCode != "" && approvalCode != "" && approvalCode != p.approvalCode {
		p.logger.Info("Ignoring task event for different approval code",
			zap.String("approval_code", approvalCode))
		return nil
	}

	instanceCode, _ := evt.Event["instance_code"].(string)
	if instanceCode == "" {
		p.logger.Warn("Instance code not found in task event")
		return nil
	}

	taskID, _ := evt.Event["task_id"].(string)
	if taskID == "" {
		p.logger.Warn("Task ID not found in task event",
			zap.String("instance_code", instanceCode))
		return nil
	}

	status, _ := evt.Event["status"].(string)
	p.logger.Info("Processing approval task event",
		zap.String("instance_code", instanceCode),
		zap.String("task_id", taskID),
		zap.String("status", status))

	if p.taskHandler == nil {
		p.logger.Warn("No task workflow handler set, ignoring task event",
			zap.String("instance_code", instanceCode),
			zap.String("task_id", taskID))
		return nil
	}

	switch strings.ToUpper(status) {
	case "PENDING":
		return p.taskHandler.HandleTaskCreated(instanceCode, taskID, evt.Event)
	case "APPROVED", "REJECTED", "DONE", "TRANSFERRED":
		return p.taskHandler.HandleTaskStatusChanged(instanceCode, taskID, evt.Event)
	default:
		p.logger.Info("Unhandled task status",
			zap.String("status", status),
			zap.String("instance_code", instanceCode))
		return nil
	}
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
