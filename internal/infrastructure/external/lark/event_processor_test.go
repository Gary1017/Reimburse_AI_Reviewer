package lark

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"go.uber.org/zap"
)

// loadFixtureBytes reads a fixture file and returns raw bytes.
// Reuses fixtureDir const from form_parser_test.go.
func loadFixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(fixtureDir + "/" + name)
	if err != nil {
		t.Fatalf("Failed to load fixture %s: %v", name, err)
	}
	return data
}

// mockWorkflowHandler records calls from EventProcessor
type mockWorkflowHandler struct {
	handleInstanceCreatedCalls  []mockHandlerCall
	handleInstanceApprovedCalls []mockHandlerCall
	handleInstanceRejectedCalls []mockHandlerCall
	handleStatusChangedCalls    []mockHandlerCall
}

type mockHandlerCall struct {
	instanceID string
	eventData  map[string]interface{}
}

func (m *mockWorkflowHandler) HandleInstanceCreated(instanceID string, eventData map[string]interface{}) error {
	m.handleInstanceCreatedCalls = append(m.handleInstanceCreatedCalls, mockHandlerCall{instanceID, eventData})
	return nil
}

func (m *mockWorkflowHandler) HandleInstanceApproved(instanceID string, eventData map[string]interface{}) error {
	m.handleInstanceApprovedCalls = append(m.handleInstanceApprovedCalls, mockHandlerCall{instanceID, eventData})
	return nil
}

func (m *mockWorkflowHandler) HandleInstanceRejected(instanceID string, eventData map[string]interface{}) error {
	m.handleInstanceRejectedCalls = append(m.handleInstanceRejectedCalls, mockHandlerCall{instanceID, eventData})
	return nil
}

func (m *mockWorkflowHandler) HandleStatusChanged(instanceID string, eventData map[string]interface{}) error {
	m.handleStatusChangedCalls = append(m.handleStatusChangedCalls, mockHandlerCall{instanceID, eventData})
	return nil
}

func TestEventProcessor_ProcessEvent_PendingRoutesToHandleStatusChanged(t *testing.T) {
	// Load the real Lark event fixture
	eventData := loadFixtureBytes(t, "20260217_172016_01_event.json")

	logger, _ := zap.NewDevelopment()
	handler := &mockWorkflowHandler{}
	processor := NewEventProcessor("7B927381-2D23-420B-9EFA-CA6C5534BC73", handler, logger)

	err := processor.ProcessEvent(context.Background(), eventData)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	// The fixture event has "status": "PENDING" but no header.event_type.
	// The processor falls back to HandleStatusChanged when the event has a "status" field
	// but the header.event_type doesn't match any known pattern.
	if len(handler.handleStatusChangedCalls) != 1 {
		t.Fatalf("Expected 1 HandleStatusChanged call, got %d (Created: %d, Approved: %d, Rejected: %d)",
			len(handler.handleStatusChangedCalls),
			len(handler.handleInstanceCreatedCalls),
			len(handler.handleInstanceApprovedCalls),
			len(handler.handleInstanceRejectedCalls))
	}

	call := handler.handleStatusChangedCalls[0]
	if call.instanceID != "BBD14B93-13A7-4ECE-8F78-901C9E94C820" {
		t.Errorf("Expected instance_code 'BBD14B93-13A7-4ECE-8F78-901C9E94C820', got %q", call.instanceID)
	}

	if call.eventData["approval_code"] != "7B927381-2D23-420B-9EFA-CA6C5534BC73" {
		t.Errorf("Expected approval_code in event data, got %v", call.eventData["approval_code"])
	}
}

func TestEventProcessor_ProcessEvent_CreatedEventType(t *testing.T) {
	// Simulate an event with header.event_type containing "created"
	payload := map[string]interface{}{
		"header": map[string]interface{}{
			"event_type": "approval_instance.created",
		},
		"event": map[string]interface{}{
			"instance_code": "BBD14B93-13A7-4ECE-8F78-901C9E94C820",
			"approval_code": "7B927381-2D23-420B-9EFA-CA6C5534BC73",
			"status":        "PENDING",
		},
	}
	data, _ := json.Marshal(payload)

	logger, _ := zap.NewDevelopment()
	handler := &mockWorkflowHandler{}
	processor := NewEventProcessor("7B927381-2D23-420B-9EFA-CA6C5534BC73", handler, logger)

	err := processor.ProcessEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	if len(handler.handleInstanceCreatedCalls) != 1 {
		t.Fatalf("Expected 1 HandleInstanceCreated call, got %d", len(handler.handleInstanceCreatedCalls))
	}

	call := handler.handleInstanceCreatedCalls[0]
	if call.instanceID != "BBD14B93-13A7-4ECE-8F78-901C9E94C820" {
		t.Errorf("Expected instance_code 'BBD14B93-13A7-4ECE-8F78-901C9E94C820', got %q", call.instanceID)
	}
}

func TestEventProcessor_ProcessEvent_FilterByApprovalCode(t *testing.T) {
	payload := map[string]interface{}{
		"header": map[string]interface{}{
			"event_type": "approval_instance.created",
		},
		"event": map[string]interface{}{
			"instance_code": "SOME-INSTANCE",
			"approval_code": "DIFFERENT-APPROVAL-CODE",
			"status":        "PENDING",
		},
	}
	data, _ := json.Marshal(payload)

	logger, _ := zap.NewDevelopment()
	handler := &mockWorkflowHandler{}
	processor := NewEventProcessor("7B927381-2D23-420B-9EFA-CA6C5534BC73", handler, logger)

	err := processor.ProcessEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v", err)
	}

	totalCalls := len(handler.handleInstanceCreatedCalls) +
		len(handler.handleInstanceApprovedCalls) +
		len(handler.handleInstanceRejectedCalls) +
		len(handler.handleStatusChangedCalls)
	if totalCalls != 0 {
		t.Errorf("Expected 0 handler calls after filtering, got %d", totalCalls)
	}
}

func TestEventProcessor_ProcessEvent_MissingInstanceCode(t *testing.T) {
	payload := map[string]interface{}{
		"header": map[string]interface{}{
			"event_type": "approval_instance.created",
		},
		"event": map[string]interface{}{
			"approval_code": "7B927381-2D23-420B-9EFA-CA6C5534BC73",
			"status":        "PENDING",
		},
	}
	data, _ := json.Marshal(payload)

	logger, _ := zap.NewDevelopment()
	handler := &mockWorkflowHandler{}
	processor := NewEventProcessor("7B927381-2D23-420B-9EFA-CA6C5534BC73", handler, logger)

	err := processor.ProcessEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, expected nil", err)
	}

	totalCalls := len(handler.handleInstanceCreatedCalls) +
		len(handler.handleInstanceApprovedCalls) +
		len(handler.handleInstanceRejectedCalls) +
		len(handler.handleStatusChangedCalls)
	if totalCalls != 0 {
		t.Errorf("Expected 0 handler calls when instance_code missing, got %d", totalCalls)
	}
}

func TestEventProcessor_ProcessEvent_MissingEventData(t *testing.T) {
	payload := map[string]interface{}{
		"header": map[string]interface{}{
			"event_type": "approval_instance.created",
		},
	}
	data, _ := json.Marshal(payload)

	logger, _ := zap.NewDevelopment()
	handler := &mockWorkflowHandler{}
	processor := NewEventProcessor("7B927381-2D23-420B-9EFA-CA6C5534BC73", handler, logger)

	err := processor.ProcessEvent(context.Background(), data)
	if err != nil {
		t.Fatalf("ProcessEvent() error = %v, expected nil", err)
	}

	totalCalls := len(handler.handleInstanceCreatedCalls) +
		len(handler.handleInstanceApprovedCalls) +
		len(handler.handleInstanceRejectedCalls) +
		len(handler.handleStatusChangedCalls)
	if totalCalls != 0 {
		t.Errorf("Expected 0 handler calls when event data missing, got %d", totalCalls)
	}
}
