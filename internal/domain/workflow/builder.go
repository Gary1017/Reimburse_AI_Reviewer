package workflow

import (
	"context"
	"fmt"
)

// GuardFunc is a function that evaluates whether a transition should be allowed
type GuardFunc func(ctx context.Context) bool

// StateMachineBuilder builds a configured state machine
type StateMachineBuilder interface {
	// Configure returns a state configuration for the given state
	Configure(state State) StateConfiguration

	// Build creates a new state machine instance with the given initial state
	Build(initialState State) StateMachine
}

// StateConfiguration configures transitions for a specific state
type StateConfiguration interface {
	// Permit allows a trigger to transition to the target state
	Permit(trigger Trigger, toState State) StateConfiguration

	// PermitIf allows a trigger to transition to the target state if the guard condition passes
	PermitIf(trigger Trigger, toState State, guard GuardFunc) StateConfiguration
}

// transition represents a state transition with optional guard
type transition struct {
	toState State
	guard   GuardFunc
}

// stateConfig implements StateConfiguration
type stateConfig struct {
	builder       *stateMachineBuilder
	fromState     State
	transitions   map[Trigger][]transition
}

// stateMachineBuilder implements StateMachineBuilder
type stateMachineBuilder struct {
	configurations map[State]*stateConfig
}

// stateMachine implements StateMachine
type stateMachine struct {
	currentState   State
	configurations map[State]*stateConfig
}

// NewBuilder creates a new state machine builder
func NewBuilder() StateMachineBuilder {
	return &stateMachineBuilder{
		configurations: make(map[State]*stateConfig),
	}
}

// Configure returns a state configuration for the given state
func (b *stateMachineBuilder) Configure(state State) StateConfiguration {
	if !state.IsValid() {
		panic(fmt.Sprintf("invalid state: %s", state))
	}

	config, exists := b.configurations[state]
	if !exists {
		config = &stateConfig{
			builder:     b,
			fromState:   state,
			transitions: make(map[Trigger][]transition),
		}
		b.configurations[state] = config
	}

	return config
}

// Build creates a new state machine instance with the given initial state
func (b *stateMachineBuilder) Build(initialState State) StateMachine {
	if !initialState.IsValid() {
		panic(fmt.Sprintf("invalid initial state: %s", initialState))
	}

	// Deep copy configurations to ensure immutability
	configsCopy := make(map[State]*stateConfig)
	for state, config := range b.configurations {
		transitionsCopy := make(map[Trigger][]transition)
		for trigger, transitions := range config.transitions {
			transitionsCopy[trigger] = append([]transition{}, transitions...)
		}
		configsCopy[state] = &stateConfig{
			builder:     nil, // No need for builder reference in copy
			fromState:   state,
			transitions: transitionsCopy,
		}
	}

	return &stateMachine{
		currentState:   initialState,
		configurations: configsCopy,
	}
}

// Permit allows a trigger to transition to the target state
func (c *stateConfig) Permit(trigger Trigger, toState State) StateConfiguration {
	return c.PermitIf(trigger, toState, nil)
}

// PermitIf allows a trigger to transition to the target state if the guard condition passes
func (c *stateConfig) PermitIf(trigger Trigger, toState State, guard GuardFunc) StateConfiguration {
	if !toState.IsValid() {
		panic(fmt.Sprintf("invalid target state: %s", toState))
	}

	c.transitions[trigger] = append(c.transitions[trigger], transition{
		toState: toState,
		guard:   guard,
	})

	return c
}

// State returns the current state
func (m *stateMachine) State() State {
	return m.currentState
}

// CanFire returns true if the trigger is permitted in the current state
func (m *stateMachine) CanFire(trigger Trigger) bool {
	config, exists := m.configurations[m.currentState]
	if !exists {
		return false
	}

	transitions, exists := config.transitions[trigger]
	if !exists || len(transitions) == 0 {
		return false
	}

	// If there's at least one transition without a guard or with a guard that could pass, return true
	// We can't evaluate guards without context here, so we return true if any transition exists
	return true
}

// Fire attempts to execute the trigger, transitioning to the new state if allowed
func (m *stateMachine) Fire(ctx context.Context, trigger Trigger) error {
	config, exists := m.configurations[m.currentState]
	if !exists {
		return fmt.Errorf("%w: cannot fire trigger %s from state %s (no configuration)", ErrInvalidTransition, trigger, m.currentState)
	}

	transitions, exists := config.transitions[trigger]
	if !exists || len(transitions) == 0 {
		return fmt.Errorf("%w: cannot fire trigger %s from state %s", ErrInvalidTransition, trigger, m.currentState)
	}

	// Try each transition in order until one succeeds
	for _, t := range transitions {
		if t.guard == nil || t.guard(ctx) {
			m.currentState = t.toState
			return nil
		}
	}

	// All guards failed
	return fmt.Errorf("%w: trigger %s from state %s", ErrGuardFailed, trigger, m.currentState)
}

// PermittedTriggers returns all triggers that can be fired in the current state
func (m *stateMachine) PermittedTriggers() []Trigger {
	config, exists := m.configurations[m.currentState]
	if !exists {
		return []Trigger{}
	}

	triggers := make([]Trigger, 0, len(config.transitions))
	for trigger := range config.transitions {
		triggers = append(triggers, trigger)
	}

	return triggers
}
