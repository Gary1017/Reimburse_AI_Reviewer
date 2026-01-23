package event

import (
	"testing"
	"time"
)

func TestType_String(t *testing.T) {
	tests := []struct {
		name     string
		eventType Type
		want     string
	}{
		{
			name:     "instance created",
			eventType: TypeInstanceCreated,
			want:     "instance.created",
		},
		{
			name:     "instance approved",
			eventType: TypeInstanceApproved,
			want:     "instance.approved",
		},
		{
			name:     "instance rejected",
			eventType: TypeInstanceRejected,
			want:     "instance.rejected",
		},
		{
			name:     "status changed",
			eventType: TypeStatusChanged,
			want:     "instance.status_changed",
		},
		{
			name:     "attachment ready",
			eventType: TypeAttachmentReady,
			want:     "attachment.ready",
		},
		{
			name:     "audit completed",
			eventType: TypeAuditCompleted,
			want:     "audit.completed",
		},
		{
			name:     "voucher generated",
			eventType: TypeVoucherGenerated,
			want:     "voucher.generated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.want {
				t.Errorf("Type.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		eventType Type
		want     bool
	}{
		{
			name:     "valid - instance created",
			eventType: TypeInstanceCreated,
			want:     true,
		},
		{
			name:     "valid - instance approved",
			eventType: TypeInstanceApproved,
			want:     true,
		},
		{
			name:     "valid - instance rejected",
			eventType: TypeInstanceRejected,
			want:     true,
		},
		{
			name:     "valid - status changed",
			eventType: TypeStatusChanged,
			want:     true,
		},
		{
			name:     "valid - attachment ready",
			eventType: TypeAttachmentReady,
			want:     true,
		},
		{
			name:     "valid - audit completed",
			eventType: TypeAuditCompleted,
			want:     true,
		},
		{
			name:     "valid - voucher generated",
			eventType: TypeVoucherGenerated,
			want:     true,
		},
		{
			name:     "invalid - unknown type",
			eventType: Type("unknown.type"),
			want:     false,
		},
		{
			name:     "invalid - empty string",
			eventType: Type(""),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.IsValid(); got != tt.want {
				t.Errorf("Type.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewEvent(t *testing.T) {
	payload := map[string]interface{}{
		"status": "APPROVED",
		"amount": 100.50,
	}

	event := NewEvent(TypeInstanceApproved, 123, "lark-instance-456", payload)

	if event == nil {
		t.Fatal("NewEvent() returned nil")
	}

	if event.ID == "" {
		t.Error("Event ID should not be empty")
	}

	if event.Type != TypeInstanceApproved {
		t.Errorf("Event Type = %v, want %v", event.Type, TypeInstanceApproved)
	}

	if event.InstanceID != 123 {
		t.Errorf("Event InstanceID = %v, want %v", event.InstanceID, 123)
	}

	if event.LarkInstanceID != "lark-instance-456" {
		t.Errorf("Event LarkInstanceID = %v, want %v", event.LarkInstanceID, "lark-instance-456")
	}

	if event.Payload == nil {
		t.Fatal("Event Payload should not be nil")
	}

	if event.Payload["status"] != "APPROVED" {
		t.Errorf("Event Payload[status] = %v, want %v", event.Payload["status"], "APPROVED")
	}

	if event.Timestamp.IsZero() {
		t.Error("Event Timestamp should not be zero")
	}

	if event.CorrelationID == "" {
		t.Error("Event CorrelationID should not be empty")
	}

	// Timestamp should be recent
	if time.Since(event.Timestamp) > time.Second {
		t.Error("Event Timestamp should be recent")
	}
}

func TestNewEventWithCorrelation(t *testing.T) {
	correlationID := "test-correlation-123"
	payload := map[string]interface{}{
		"result": "success",
	}

	event := NewEventWithCorrelation(TypeAuditCompleted, 789, "lark-789", payload, correlationID)

	if event == nil {
		t.Fatal("NewEventWithCorrelation() returned nil")
	}

	if event.CorrelationID != correlationID {
		t.Errorf("Event CorrelationID = %v, want %v", event.CorrelationID, correlationID)
	}

	if event.Type != TypeAuditCompleted {
		t.Errorf("Event Type = %v, want %v", event.Type, TypeAuditCompleted)
	}

	if event.InstanceID != 789 {
		t.Errorf("Event InstanceID = %v, want %v", event.InstanceID, 789)
	}
}

func TestEvent_WithPayload(t *testing.T) {
	original := NewEvent(TypeInstanceCreated, 1, "lark-1", map[string]interface{}{
		"key1": "value1",
	})

	// Add a new payload key
	modified := original.WithPayload("key2", "value2")

	// Original should be unchanged (immutability)
	if _, exists := original.Payload["key2"]; exists {
		t.Error("Original event should not be modified")
	}

	if original.Payload["key1"] != "value1" {
		t.Error("Original event payload should remain intact")
	}

	// Modified should have both keys
	if modified.Payload["key1"] != "value1" {
		t.Error("Modified event should retain original payload")
	}

	if modified.Payload["key2"] != "value2" {
		t.Error("Modified event should have new payload")
	}

	// Other fields should be copied
	if modified.ID != original.ID {
		t.Error("Modified event should have same ID")
	}

	if modified.Type != original.Type {
		t.Error("Modified event should have same Type")
	}

	if modified.InstanceID != original.InstanceID {
		t.Error("Modified event should have same InstanceID")
	}

	if modified.CorrelationID != original.CorrelationID {
		t.Error("Modified event should have same CorrelationID")
	}
}

func TestEvent_GetPayloadString(t *testing.T) {
	event := NewEvent(TypeInstanceCreated, 1, "lark-1", map[string]interface{}{
		"status":  "APPROVED",
		"number":  123,
		"missing": nil,
	})

	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "existing string",
			key:  "status",
			want: "APPROVED",
		},
		{
			name: "non-string value",
			key:  "number",
			want: "",
		},
		{
			name: "missing key",
			key:  "nonexistent",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := event.GetPayloadString(tt.key); got != tt.want {
				t.Errorf("GetPayloadString(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEvent_GetPayloadInt(t *testing.T) {
	event := NewEvent(TypeInstanceCreated, 1, "lark-1", map[string]interface{}{
		"int64":   int64(100),
		"int":     50,
		"float64": 75.5,
		"string":  "not a number",
		"missing": nil,
	})

	tests := []struct {
		name string
		key  string
		want int64
	}{
		{
			name: "int64 value",
			key:  "int64",
			want: 100,
		},
		{
			name: "int value",
			key:  "int",
			want: 50,
		},
		{
			name: "float64 value (converted)",
			key:  "float64",
			want: 75,
		},
		{
			name: "non-int value",
			key:  "string",
			want: 0,
		},
		{
			name: "missing key",
			key:  "nonexistent",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := event.GetPayloadInt(tt.key); got != tt.want {
				t.Errorf("GetPayloadInt(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEvent_GetPayloadFloat(t *testing.T) {
	event := NewEvent(TypeInstanceCreated, 1, "lark-1", map[string]interface{}{
		"float64": 123.45,
		"int64":   int64(100),
		"int":     50,
		"string":  "not a number",
		"missing": nil,
	})

	tests := []struct {
		name string
		key  string
		want float64
	}{
		{
			name: "float64 value",
			key:  "float64",
			want: 123.45,
		},
		{
			name: "int64 value (converted)",
			key:  "int64",
			want: 100.0,
		},
		{
			name: "int value (converted)",
			key:  "int",
			want: 50.0,
		},
		{
			name: "non-numeric value",
			key:  "string",
			want: 0.0,
		},
		{
			name: "missing key",
			key:  "nonexistent",
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := event.GetPayloadFloat(tt.key); got != tt.want {
				t.Errorf("GetPayloadFloat(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEvent_GetPayloadBool(t *testing.T) {
	event := NewEvent(TypeInstanceCreated, 1, "lark-1", map[string]interface{}{
		"bool_true":  true,
		"bool_false": false,
		"string":     "not a bool",
		"missing":    nil,
	})

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "true value",
			key:  "bool_true",
			want: true,
		},
		{
			name: "false value",
			key:  "bool_false",
			want: false,
		},
		{
			name: "non-bool value",
			key:  "string",
			want: false,
		},
		{
			name: "missing key",
			key:  "nonexistent",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := event.GetPayloadBool(tt.key); got != tt.want {
				t.Errorf("GetPayloadBool(%v) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestEvent_UniqueIDs(t *testing.T) {
	// Create multiple events and verify IDs are unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		event := NewEvent(TypeInstanceCreated, int64(i), "lark-1", nil)
		if ids[event.ID] {
			t.Errorf("Duplicate event ID found: %s", event.ID)
		}
		ids[event.ID] = true
	}
}

func TestEvent_CorrelationChain(t *testing.T) {
	// First event in the chain
	event1 := NewEvent(TypeInstanceCreated, 1, "lark-1", nil)
	correlationID := event1.CorrelationID

	// Second event using same correlation ID
	event2 := NewEventWithCorrelation(TypeStatusChanged, 1, "lark-1", nil, correlationID)

	// Third event using same correlation ID
	event3 := NewEventWithCorrelation(TypeInstanceApproved, 1, "lark-1", nil, correlationID)

	if event2.CorrelationID != correlationID {
		t.Error("Event2 should have same correlation ID")
	}

	if event3.CorrelationID != correlationID {
		t.Error("Event3 should have same correlation ID")
	}

	// Each event should have unique ID
	if event1.ID == event2.ID || event1.ID == event3.ID || event2.ID == event3.ID {
		t.Error("Events should have unique IDs even with same correlation ID")
	}
}

func TestEvent_ImmutabilityChain(t *testing.T) {
	// Start with an event
	event1 := NewEvent(TypeInstanceCreated, 1, "lark-1", map[string]interface{}{
		"step": 1,
	})

	// Add payload multiple times
	event2 := event1.WithPayload("step", 2)
	event3 := event2.WithPayload("step", 3)

	// Verify each event is independent
	if event1.GetPayloadInt("step") != 1 {
		t.Error("Event1 should have step=1")
	}

	if event2.GetPayloadInt("step") != 2 {
		t.Error("Event2 should have step=2")
	}

	if event3.GetPayloadInt("step") != 3 {
		t.Error("Event3 should have step=3")
	}
}
