package lark

import (
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"go.uber.org/zap"
)

// NewEventDispatcher creates an event dispatcher for WebSocket event subscription
// This registers custom event handlers for approval instance events
func NewEventDispatcher(eventProcessor *EventProcessor, logger *zap.Logger) *dispatcher.EventDispatcher {
	logger.Info("Creating event dispatcher for approval events")

	// Create SDK event dispatcher
	// No verification token or encrypt key needed for WebSocket (long connection mode)
	d := dispatcher.NewEventDispatcher("", "")

	// Register custom event handler for approval instance events
	// The event key for approval instances is "approval_instance"
	// This will handle all approval events: create, approve, reject, status_changed
	d.OnCustomizedEvent("approval_instance", eventProcessor.HandleCustomizedEvent)

	logger.Info("Event dispatcher created with approval_instance handler registered")

	return d
}
