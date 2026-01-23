package workflow

import "errors"

var (
	// ErrInvalidTransition is returned when a state transition is not allowed
	ErrInvalidTransition = errors.New("invalid state transition")

	// ErrInvalidState is returned when a state is not valid
	ErrInvalidState = errors.New("invalid state")

	// ErrGuardFailed is returned when a guard condition fails
	ErrGuardFailed = errors.New("guard condition failed")
)
