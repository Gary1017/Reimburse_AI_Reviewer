package event

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Event represents a domain event
type Event struct {
	ID             string                 `json:"id"`
	Type           Type                   `json:"type"`
	InstanceID     int64                  `json:"instance_id"`
	LarkInstanceID string                 `json:"lark_instance_id"`
	Payload        map[string]interface{} `json:"payload"`
	Timestamp      time.Time              `json:"timestamp"`
	CorrelationID  string                 `json:"correlation_id"`
}

// NewEvent creates a new domain event with auto-generated ID and timestamp
func NewEvent(eventType Type, instanceID int64, larkInstanceID string, payload map[string]interface{}) *Event {
	return &Event{
		ID:             generateID(),
		Type:           eventType,
		InstanceID:     instanceID,
		LarkInstanceID: larkInstanceID,
		Payload:        payload,
		Timestamp:      time.Now(),
		CorrelationID:  generateID(),
	}
}

// NewEventWithCorrelation creates an event linked to a correlation chain
func NewEventWithCorrelation(eventType Type, instanceID int64, larkInstanceID string, payload map[string]interface{}, correlationID string) *Event {
	return &Event{
		ID:             generateID(),
		Type:           eventType,
		InstanceID:     instanceID,
		LarkInstanceID: larkInstanceID,
		Payload:        payload,
		Timestamp:      time.Now(),
		CorrelationID:  correlationID,
	}
}

// WithPayload returns a new Event with an added payload key-value pair (immutable operation)
func (e *Event) WithPayload(key string, value interface{}) *Event {
	// Create a deep copy of the payload map
	newPayload := make(map[string]interface{}, len(e.Payload)+1)
	for k, v := range e.Payload {
		newPayload[k] = v
	}
	newPayload[key] = value

	// Return a new Event with the updated payload
	return &Event{
		ID:             e.ID,
		Type:           e.Type,
		InstanceID:     e.InstanceID,
		LarkInstanceID: e.LarkInstanceID,
		Payload:        newPayload,
		Timestamp:      e.Timestamp,
		CorrelationID:  e.CorrelationID,
	}
}

// GetPayloadString retrieves a string value from the payload
func (e *Event) GetPayloadString(key string) string {
	if val, ok := e.Payload[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetPayloadInt retrieves an int64 value from the payload
func (e *Event) GetPayloadInt(key string) int64 {
	if val, ok := e.Payload[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

// GetPayloadFloat retrieves a float64 value from the payload
func (e *Event) GetPayloadFloat(key string) float64 {
	if val, ok := e.Payload[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int64:
			return float64(v)
		case int:
			return float64(v)
		}
	}
	return 0.0
}

// GetPayloadBool retrieves a bool value from the payload
func (e *Event) GetPayloadBool(key string) bool {
	if val, ok := e.Payload[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// generateID creates a unique ID using timestamp and random bytes
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}
