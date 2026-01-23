package workflow

import (
	"context"

	"github.com/garyjia/ai-reimbursement/internal/domain/event"
	domainwf "github.com/garyjia/ai-reimbursement/internal/domain/workflow"
)

// WorkflowEngine orchestrates the approval workflow
type WorkflowEngine interface {
	// HandleEvent processes a domain event through the workflow
	HandleEvent(ctx context.Context, evt *event.Event) error

	// GetStateMachine returns a state machine for an instance (creates if not cached)
	GetStateMachine(ctx context.Context, instanceID int64) (domainwf.StateMachine, error)

	// TransitionState triggers a state transition for an instance
	TransitionState(ctx context.Context, instanceID int64, trigger domainwf.Trigger) error

	// GetCurrentState returns the current state of an instance
	GetCurrentState(ctx context.Context, instanceID int64) (domainwf.State, error)
}
