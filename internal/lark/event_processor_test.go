package lark

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

type stubWorkflowHandler struct {
	createdCalls   int
	approvedCalls  int
	rejectedCalls  int
	statusCalls    int
	lastInstanceID string
	lastEventData  map[string]interface{}
}

func (s *stubWorkflowHandler) HandleInstanceCreated(instanceID string, eventData map[string]interface{}) error {
	s.createdCalls++
	s.lastInstanceID = instanceID
	s.lastEventData = eventData
	return nil
}

func (s *stubWorkflowHandler) HandleInstanceApproved(instanceID string, eventData map[string]interface{}) error {
	s.approvedCalls++
	s.lastInstanceID = instanceID
	s.lastEventData = eventData
	return nil
}

func (s *stubWorkflowHandler) HandleInstanceRejected(instanceID string, eventData map[string]interface{}) error {
	s.rejectedCalls++
	s.lastInstanceID = instanceID
	s.lastEventData = eventData
	return nil
}

func (s *stubWorkflowHandler) HandleStatusChanged(instanceID string, eventData map[string]interface{}) error {
	s.statusCalls++
	s.lastInstanceID = instanceID
	s.lastEventData = eventData
	return nil
}

func buildEventPayload(eventType, instanceID, approvalCode string, extra map[string]interface{}) []byte {
	eventData := map[string]interface{}{
		"instance_code": instanceID,
		"approval_code": approvalCode,
	}
	for key, value := range extra {
		eventData[key] = value
	}
	payload := map[string]interface{}{
		"header": map[string]interface{}{
			"event_type": eventType,
		},
		"event": eventData,
	}
	data, _ := json.Marshal(payload)
	return data
}

func TestEventProcessorDispatchCreated(t *testing.T) {
	handler := &stubWorkflowHandler{}
	processor := NewEventProcessor("APPROVAL_CODE", handler, zap.NewNop())
	payload := buildEventPayload("approval.approval_instance.created_v4", "instance-1", "APPROVAL_CODE", nil)

	if err := processor.ProcessEvent(context.Background(), payload); err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	if handler.createdCalls != 1 {
		t.Fatalf("expected created handler to be called once, got %d", handler.createdCalls)
	}
	if handler.lastInstanceID != "instance-1" {
		t.Fatalf("expected instance ID instance-1, got %s", handler.lastInstanceID)
	}
}

func TestEventProcessorDispatchApproved(t *testing.T) {
	handler := &stubWorkflowHandler{}
	processor := NewEventProcessor("APPROVAL_CODE", handler, zap.NewNop())
	payload := buildEventPayload("approval.approval_instance.approved_v4", "instance-2", "APPROVAL_CODE", nil)

	if err := processor.ProcessEvent(context.Background(), payload); err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	if handler.approvedCalls != 1 {
		t.Fatalf("expected approved handler to be called once, got %d", handler.approvedCalls)
	}
}

func TestEventProcessorDispatchStatusChangedForGenericEvent(t *testing.T) {
	handler := &stubWorkflowHandler{}
	processor := NewEventProcessor("APPROVAL_CODE", handler, zap.NewNop())
	payload := buildEventPayload("approval_instance", "instance-3", "APPROVAL_CODE", map[string]interface{}{
		"status": "PENDING",
	})

	if err := processor.ProcessEvent(context.Background(), payload); err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	if handler.statusCalls != 1 {
		t.Fatalf("expected status handler to be called once, got %d", handler.statusCalls)
	}
}

func TestEventProcessorSkipsMismatchedApprovalCode(t *testing.T) {
	handler := &stubWorkflowHandler{}
	processor := NewEventProcessor("APPROVAL_CODE", handler, zap.NewNop())
	payload := buildEventPayload("approval.approval_instance.created_v4", "instance-4", "OTHER_CODE", nil)

	if err := processor.ProcessEvent(context.Background(), payload); err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	if handler.createdCalls != 0 {
		t.Fatalf("expected handler not to be called, got %d", handler.createdCalls)
	}
}
