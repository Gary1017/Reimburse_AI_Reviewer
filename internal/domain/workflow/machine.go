package workflow

import "context"

// StateMachine represents a state machine that tracks current state and validates transitions
type StateMachine interface {
	// State returns the current state
	State() State

	// CanFire returns true if the trigger is permitted in the current state
	CanFire(trigger Trigger) bool

	// Fire attempts to execute the trigger, transitioning to the new state if allowed
	Fire(ctx context.Context, trigger Trigger) error

	// PermittedTriggers returns all triggers that can be fired in the current state
	PermittedTriggers() []Trigger
}
