package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    State
		expected bool
	}{
		{StateCreated, false},
		{StatePending, false},
		{StateAIAuditing, false},
		{StateAIAudited, false},
		{StateInReview, false},
		{StateAutoApproved, false},
		{StateApproved, false},
		{StateVoucherGenerating, false},
		{StateRejected, true},
		{StateCompleted, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsTerminal(); got != tt.expected {
				t.Errorf("State.IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestState_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		expected bool
	}{
		{"valid state", StateCreated, true},
		{"valid state", StateCompleted, true},
		{"invalid state", State("INVALID"), false},
		{"empty state", State(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsValid(); got != tt.expected {
				t.Errorf("State.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestState_String(t *testing.T) {
	state := StateCreated
	if got := state.String(); got != "CREATED" {
		t.Errorf("State.String() = %v, want %v", got, "CREATED")
	}
}

func TestTrigger_String(t *testing.T) {
	trigger := TriggerSubmit
	if got := trigger.String(); got != "SUBMIT" {
		t.Errorf("Trigger.String() = %v, want %v", got, "SUBMIT")
	}
}

func TestBuilder_Configure(t *testing.T) {
	builder := NewBuilder()

	// Configure valid state
	config := builder.Configure(StateCreated)
	if config == nil {
		t.Fatal("Configure() returned nil")
	}

	// Configure same state again should return same config
	config2 := builder.Configure(StateCreated)
	if config != config2 {
		t.Error("Configure() should return same config for same state")
	}
}

func TestBuilder_ConfigurePanicsOnInvalidState(t *testing.T) {
	builder := NewBuilder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Configure() should panic on invalid state")
		}
	}()

	builder.Configure(State("INVALID"))
}

func TestBuilder_BuildPanicsOnInvalidInitialState(t *testing.T) {
	builder := NewBuilder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Build() should panic on invalid initial state")
		}
	}()

	builder.Build(State("INVALID"))
}

func TestStateConfiguration_Permit(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending)

	machine := builder.Build(StateCreated)

	if !machine.CanFire(TriggerSubmit) {
		t.Error("CanFire() should return true for permitted trigger")
	}

	if err := machine.Fire(context.Background(), TriggerSubmit); err != nil {
		t.Errorf("Fire() failed: %v", err)
	}

	if machine.State() != StatePending {
		t.Errorf("State after Fire() = %v, want %v", machine.State(), StatePending)
	}
}

func TestStateConfiguration_PermitIf_GuardPasses(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		PermitIf(TriggerSubmit, StatePending, func(ctx context.Context) bool {
			return true
		})

	machine := builder.Build(StateCreated)

	if err := machine.Fire(context.Background(), TriggerSubmit); err != nil {
		t.Errorf("Fire() failed: %v", err)
	}

	if machine.State() != StatePending {
		t.Errorf("State after Fire() = %v, want %v", machine.State(), StatePending)
	}
}

func TestStateConfiguration_PermitIf_GuardFails(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		PermitIf(TriggerSubmit, StatePending, func(ctx context.Context) bool {
			return false
		})

	machine := builder.Build(StateCreated)

	err := machine.Fire(context.Background(), TriggerSubmit)
	if err == nil {
		t.Fatal("Fire() should fail when guard fails")
	}

	if !errors.Is(err, ErrGuardFailed) {
		t.Errorf("Fire() error = %v, want %v", err, ErrGuardFailed)
	}

	if machine.State() != StateCreated {
		t.Errorf("State should remain %v after failed Fire(), got %v", StateCreated, machine.State())
	}
}

func TestStateConfiguration_PermitIf_MultipleTransitions(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		PermitIf(TriggerSubmit, StateAIAuditing, func(ctx context.Context) bool {
			return ctx.Value("auto").(bool)
		}).
		PermitIf(TriggerSubmit, StatePending, func(ctx context.Context) bool {
			return !ctx.Value("auto").(bool)
		})

	// Test first transition (guard passes)
	machine1 := builder.Build(StateCreated)
	ctx1 := context.WithValue(context.Background(), "auto", true)
	if err := machine1.Fire(ctx1, TriggerSubmit); err != nil {
		t.Errorf("Fire() failed: %v", err)
	}
	if machine1.State() != StateAIAuditing {
		t.Errorf("State after Fire() = %v, want %v", machine1.State(), StateAIAuditing)
	}

	// Test second transition (first guard fails, second passes)
	machine2 := builder.Build(StateCreated)
	ctx2 := context.WithValue(context.Background(), "auto", false)
	if err := machine2.Fire(ctx2, TriggerSubmit); err != nil {
		t.Errorf("Fire() failed: %v", err)
	}
	if machine2.State() != StatePending {
		t.Errorf("State after Fire() = %v, want %v", machine2.State(), StatePending)
	}
}

func TestStateConfiguration_PermitPanicsOnInvalidState(t *testing.T) {
	builder := NewBuilder()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Permit() should panic on invalid target state")
		}
	}()

	builder.Configure(StateCreated).Permit(TriggerSubmit, State("INVALID"))
}

func TestStateMachine_CanFire(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending)

	machine := builder.Build(StateCreated)

	tests := []struct {
		trigger  Trigger
		expected bool
	}{
		{TriggerSubmit, true},
		{TriggerApprove, false},
		{TriggerReject, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.trigger), func(t *testing.T) {
			if got := machine.CanFire(tt.trigger); got != tt.expected {
				t.Errorf("CanFire() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestStateMachine_Fire_InvalidTransition(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending)

	machine := builder.Build(StateCreated)

	err := machine.Fire(context.Background(), TriggerApprove)
	if err == nil {
		t.Fatal("Fire() should fail for invalid transition")
	}

	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("Fire() error = %v, want %v", err, ErrInvalidTransition)
	}

	if machine.State() != StateCreated {
		t.Errorf("State should remain %v after failed Fire(), got %v", StateCreated, machine.State())
	}
}

func TestStateMachine_Fire_NoConfiguration(t *testing.T) {
	builder := NewBuilder()
	// Build without configuring StateCreated
	machine := builder.Build(StateCreated)

	err := machine.Fire(context.Background(), TriggerSubmit)
	if err == nil {
		t.Fatal("Fire() should fail when no configuration exists")
	}

	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("Fire() error = %v, want %v", err, ErrInvalidTransition)
	}
}

func TestStateMachine_PermittedTriggers(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending).
		Permit(TriggerStartAudit, StateAIAuditing)

	machine := builder.Build(StateCreated)

	triggers := machine.PermittedTriggers()
	if len(triggers) != 2 {
		t.Errorf("PermittedTriggers() returned %d triggers, want 2", len(triggers))
	}

	// Check that both triggers are present (order doesn't matter)
	hasSubmit := false
	hasStartAudit := false
	for _, trigger := range triggers {
		if trigger == TriggerSubmit {
			hasSubmit = true
		}
		if trigger == TriggerStartAudit {
			hasStartAudit = true
		}
	}

	if !hasSubmit || !hasStartAudit {
		t.Errorf("PermittedTriggers() = %v, want both TriggerSubmit and TriggerStartAudit", triggers)
	}
}

func TestStateMachine_PermittedTriggers_NoConfiguration(t *testing.T) {
	builder := NewBuilder()
	machine := builder.Build(StateCreated)

	triggers := machine.PermittedTriggers()
	if len(triggers) != 0 {
		t.Errorf("PermittedTriggers() returned %d triggers, want 0", len(triggers))
	}
}

func TestStateMachine_Immutability(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending)

	// Build two machines from same builder
	machine1 := builder.Build(StateCreated)
	machine2 := builder.Build(StateCreated)

	// Fire trigger on machine1
	if err := machine1.Fire(context.Background(), TriggerSubmit); err != nil {
		t.Errorf("Fire() failed: %v", err)
	}

	// machine2 should remain in initial state
	if machine2.State() != StateCreated {
		t.Errorf("machine2 state = %v, want %v (machines should be independent)", machine2.State(), StateCreated)
	}

	// machine1 should be in new state
	if machine1.State() != StatePending {
		t.Errorf("machine1 state = %v, want %v", machine1.State(), StatePending)
	}
}

func TestStateMachine_ComplexWorkflow(t *testing.T) {
	// Build a state machine matching the existing workflow
	builder := NewBuilder()

	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending).
		Permit(TriggerStartAudit, StateAIAuditing).
		Permit(TriggerApprove, StateApproved).
		Permit(TriggerReject, StateRejected)

	builder.Configure(StatePending).
		Permit(TriggerStartAudit, StateAIAuditing).
		Permit(TriggerRequestReview, StateInReview).
		Permit(TriggerApprove, StateApproved).
		Permit(TriggerReject, StateRejected)

	builder.Configure(StateAIAuditing).
		Permit(TriggerCompleteAudit, StateAIAudited).
		Permit(TriggerReject, StateRejected)

	builder.Configure(StateAIAudited).
		Permit(TriggerRequestReview, StateInReview).
		Permit(TriggerAutoApprove, StateAutoApproved).
		Permit(TriggerApprove, StateApproved).
		Permit(TriggerReject, StateRejected)

	builder.Configure(StateInReview).
		Permit(TriggerApprove, StateApproved).
		Permit(TriggerReject, StateRejected)

	builder.Configure(StateAutoApproved).
		Permit(TriggerApprove, StateApproved).
		Permit(TriggerStartVoucher, StateVoucherGenerating)

	builder.Configure(StateApproved).
		Permit(TriggerStartVoucher, StateVoucherGenerating).
		Permit(TriggerCompleteVoucher, StateCompleted)

	builder.Configure(StateVoucherGenerating).
		Permit(TriggerCompleteVoucher, StateCompleted).
		Permit(TriggerRetry, StateApproved)

	// Test a complete workflow path
	machine := builder.Build(StateCreated)

	steps := []struct {
		trigger       Trigger
		expectedState State
	}{
		{TriggerSubmit, StatePending},
		{TriggerStartAudit, StateAIAuditing},
		{TriggerCompleteAudit, StateAIAudited},
		{TriggerAutoApprove, StateAutoApproved},
		{TriggerStartVoucher, StateVoucherGenerating},
		{TriggerCompleteVoucher, StateCompleted},
	}

	for i, step := range steps {
		if err := machine.Fire(context.Background(), step.trigger); err != nil {
			t.Errorf("Step %d: Fire(%v) failed: %v", i, step.trigger, err)
		}

		if machine.State() != step.expectedState {
			t.Errorf("Step %d: State after Fire(%v) = %v, want %v", i, step.trigger, machine.State(), step.expectedState)
		}
	}

	// Verify terminal state
	if !machine.State().IsTerminal() {
		t.Error("Final state should be terminal")
	}

	// Verify no more transitions allowed
	triggers := machine.PermittedTriggers()
	if len(triggers) != 0 {
		t.Errorf("Terminal state should have 0 permitted triggers, got %d", len(triggers))
	}
}

func TestStateMachine_RejectionPath(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateCreated).
		Permit(TriggerSubmit, StatePending).
		Permit(TriggerReject, StateRejected)

	builder.Configure(StatePending).
		Permit(TriggerReject, StateRejected)

	machine := builder.Build(StateCreated)

	// Submit and then reject
	if err := machine.Fire(context.Background(), TriggerSubmit); err != nil {
		t.Errorf("Fire(TriggerSubmit) failed: %v", err)
	}

	if err := machine.Fire(context.Background(), TriggerReject); err != nil {
		t.Errorf("Fire(TriggerReject) failed: %v", err)
	}

	if machine.State() != StateRejected {
		t.Errorf("State = %v, want %v", machine.State(), StateRejected)
	}

	if !machine.State().IsTerminal() {
		t.Error("Rejected state should be terminal")
	}
}

func TestStateMachine_VoucherRetryPath(t *testing.T) {
	builder := NewBuilder()
	builder.Configure(StateApproved).
		Permit(TriggerStartVoucher, StateVoucherGenerating)

	builder.Configure(StateVoucherGenerating).
		Permit(TriggerRetry, StateApproved).
		Permit(TriggerCompleteVoucher, StateCompleted)

	machine := builder.Build(StateApproved)

	// Start voucher generation
	if err := machine.Fire(context.Background(), TriggerStartVoucher); err != nil {
		t.Errorf("Fire(TriggerStartVoucher) failed: %v", err)
	}

	if machine.State() != StateVoucherGenerating {
		t.Errorf("State = %v, want %v", machine.State(), StateVoucherGenerating)
	}

	// Retry (e.g., generation failed)
	if err := machine.Fire(context.Background(), TriggerRetry); err != nil {
		t.Errorf("Fire(TriggerRetry) failed: %v", err)
	}

	if machine.State() != StateApproved {
		t.Errorf("State = %v, want %v", machine.State(), StateApproved)
	}

	// Try again and succeed
	if err := machine.Fire(context.Background(), TriggerStartVoucher); err != nil {
		t.Errorf("Fire(TriggerStartVoucher) failed: %v", err)
	}

	if err := machine.Fire(context.Background(), TriggerCompleteVoucher); err != nil {
		t.Errorf("Fire(TriggerCompleteVoucher) failed: %v", err)
	}

	if machine.State() != StateCompleted {
		t.Errorf("State = %v, want %v", machine.State(), StateCompleted)
	}
}
