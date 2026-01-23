package dispatcher

import (
	"context"

	"github.com/garyjia/ai-reimbursement/internal/domain/event"
)

// Handler processes domain events
type Handler func(ctx context.Context, evt *event.Event) error

// HandlerInfo contains handler metadata for debugging
type HandlerInfo struct {
	Name        string
	EventType   event.Type
	Handler     Handler
	Description string
}
