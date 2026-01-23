package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
	domainwf "github.com/garyjia/ai-reimbursement/internal/domain/workflow"
)

// Mock implementations

type mockInstanceRepo struct {
	instances map[int64]*entity.ApprovalInstance
	updateErr error
}

func (m *mockInstanceRepo) Create(ctx context.Context, instance *entity.ApprovalInstance) error {
	m.instances[instance.ID] = instance
	return nil
}

func (m *mockInstanceRepo) GetByID(ctx context.Context, id int64) (*entity.ApprovalInstance, error) {
	instance, exists := m.instances[id]
	if !exists {
		return nil, errors.New("instance not found")
	}
	return instance, nil
}

func (m *mockInstanceRepo) GetByLarkInstanceID(ctx context.Context, larkID string) (*entity.ApprovalInstance, error) {
	for _, instance := range m.instances {
		if instance.LarkInstanceID == larkID {
			return instance, nil
		}
	}
	return nil, errors.New("instance not found")
}

func (m *mockInstanceRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if instance, exists := m.instances[id]; exists {
		instance.Status = status
		return nil
	}
	return errors.New("instance not found")
}

func (m *mockInstanceRepo) SetApprovalTime(ctx context.Context, id int64, t time.Time) error {
	if instance, exists := m.instances[id]; exists {
		instance.ApprovalTime = &t
		return nil
	}
	return errors.New("instance not found")
}

func (m *mockInstanceRepo) List(ctx context.Context, limit, offset int) ([]*entity.ApprovalInstance, error) {
	return nil, nil
}

type mockHistoryRepo struct {
	histories []*entity.ApprovalHistory
	createErr error
}

func (m *mockHistoryRepo) Create(ctx context.Context, history *entity.ApprovalHistory) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.histories = append(m.histories, history)
	return nil
}

func (m *mockHistoryRepo) GetByInstanceID(ctx context.Context, instanceID int64) ([]*entity.ApprovalHistory, error) {
	var result []*entity.ApprovalHistory
	for _, h := range m.histories {
		if h.InstanceID == instanceID {
			result = append(result, h)
		}
	}
	return result, nil
}

type mockTxManager struct {
	commitErr error
}

func (m *mockTxManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.commitErr != nil {
		return m.commitErr
	}
	return fn(ctx)
}

type mockDispatcher struct {
	events []*event.Event
}

func (m *mockDispatcher) Subscribe(eventType event.Type, handler dispatcher.Handler) {}

func (m *mockDispatcher) SubscribeNamed(eventType event.Type, name string, handler dispatcher.Handler) {}

func (m *mockDispatcher) Unsubscribe(eventType event.Type, name string) {}

func (m *mockDispatcher) Dispatch(ctx context.Context, evt *event.Event) error {
	m.events = append(m.events, evt)
	return nil
}

func (m *mockDispatcher) DispatchAsync(ctx context.Context, evt *event.Event) {
	m.events = append(m.events, evt)
}

func (m *mockDispatcher) ListHandlers(eventType event.Type) []dispatcher.HandlerInfo {
	return nil
}

func (m *mockDispatcher) Close() error {
	return nil
}

// Test factory

func TestBuildReimbursementStateMachine(t *testing.T) {
	tests := []struct {
		name         string
		initialState domainwf.State
		trigger      domainwf.Trigger
		wantState    domainwf.State
		wantError    bool
	}{
		{
			name:         "CREATED -> PENDING on SUBMIT",
			initialState: domainwf.StateCreated,
			trigger:      domainwf.TriggerSubmit,
			wantState:    domainwf.StatePending,
			wantError:    false,
		},
		{
			name:         "CREATED -> AI_AUDITING on START_AUDIT",
			initialState: domainwf.StateCreated,
			trigger:      domainwf.TriggerStartAudit,
			wantState:    domainwf.StateAIAuditing,
			wantError:    false,
		},
		{
			name:         "PENDING -> AI_AUDITING on START_AUDIT",
			initialState: domainwf.StatePending,
			trigger:      domainwf.TriggerStartAudit,
			wantState:    domainwf.StateAIAuditing,
			wantError:    false,
		},
		{
			name:         "AI_AUDITING -> AI_AUDITED on COMPLETE_AUDIT",
			initialState: domainwf.StateAIAuditing,
			trigger:      domainwf.TriggerCompleteAudit,
			wantState:    domainwf.StateAIAudited,
			wantError:    false,
		},
		{
			name:         "AI_AUDITED -> AUTO_APPROVED on AUTO_APPROVE",
			initialState: domainwf.StateAIAudited,
			trigger:      domainwf.TriggerAutoApprove,
			wantState:    domainwf.StateAutoApproved,
			wantError:    false,
		},
		{
			name:         "AI_AUDITED -> IN_REVIEW on REQUEST_REVIEW",
			initialState: domainwf.StateAIAudited,
			trigger:      domainwf.TriggerRequestReview,
			wantState:    domainwf.StateInReview,
			wantError:    false,
		},
		{
			name:         "IN_REVIEW -> APPROVED on APPROVE",
			initialState: domainwf.StateInReview,
			trigger:      domainwf.TriggerApprove,
			wantState:    domainwf.StateApproved,
			wantError:    false,
		},
		{
			name:         "AUTO_APPROVED -> APPROVED on APPROVE",
			initialState: domainwf.StateAutoApproved,
			trigger:      domainwf.TriggerApprove,
			wantState:    domainwf.StateApproved,
			wantError:    false,
		},
		{
			name:         "APPROVED -> VOUCHER_GENERATING on START_VOUCHER",
			initialState: domainwf.StateApproved,
			trigger:      domainwf.TriggerStartVoucher,
			wantState:    domainwf.StateVoucherGenerating,
			wantError:    false,
		},
		{
			name:         "VOUCHER_GENERATING -> COMPLETED on COMPLETE_VOUCHER",
			initialState: domainwf.StateVoucherGenerating,
			trigger:      domainwf.TriggerCompleteVoucher,
			wantState:    domainwf.StateCompleted,
			wantError:    false,
		},
		{
			name:         "VOUCHER_GENERATING -> APPROVED on RETRY",
			initialState: domainwf.StateVoucherGenerating,
			trigger:      domainwf.TriggerRetry,
			wantState:    domainwf.StateApproved,
			wantError:    false,
		},
		{
			name:         "Invalid transition",
			initialState: domainwf.StateCompleted,
			trigger:      domainwf.TriggerSubmit,
			wantState:    domainwf.StateCompleted,
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := BuildReimbursementStateMachine(tt.initialState)

			err := machine.Fire(context.Background(), tt.trigger)

			if tt.wantError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if machine.State() != tt.wantState {
				t.Errorf("expected state %s, got %s", tt.wantState, machine.State())
			}
		})
	}
}

// Test engine

func TestNewEngine(t *testing.T) {
	instanceRepo := &mockInstanceRepo{instances: make(map[int64]*entity.ApprovalInstance)}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	if engine == nil {
		t.Fatal("expected engine to be created")
	}
}

func TestEngineGetStateMachine(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		instances: map[int64]*entity.ApprovalInstance{
			1: {ID: 1, Status: string(domainwf.StateCreated)},
			2: {ID: 2, Status: string(domainwf.StatePending)},
		},
	}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	// Test getting machine for instance 1
	machine, err := engine.GetStateMachine(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if machine.State() != domainwf.StateCreated {
		t.Errorf("expected state CREATED, got %s", machine.State())
	}

	// Test getting machine for instance 2
	machine2, err := engine.GetStateMachine(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if machine2.State() != domainwf.StatePending {
		t.Errorf("expected state PENDING, got %s", machine2.State())
	}

	// Test cache hit
	machine3, err := engine.GetStateMachine(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if machine3.State() != domainwf.StateCreated {
		t.Errorf("expected cached state CREATED, got %s", machine3.State())
	}
}

func TestEngineGetStateMachineNotFound(t *testing.T) {
	instanceRepo := &mockInstanceRepo{instances: make(map[int64]*entity.ApprovalInstance)}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	_, err := engine.GetStateMachine(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent instance")
	}
}

func TestEngineTransitionState(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		instances: map[int64]*entity.ApprovalInstance{
			1: {ID: 1, Status: string(domainwf.StateCreated)},
		},
	}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}
	disp := &mockDispatcher{}

	engine := NewEngine(instanceRepo, historyRepo, txManager, WithDispatcher(disp))

	// Transition from CREATED to PENDING
	err := engine.TransitionState(context.Background(), 1, domainwf.TriggerSubmit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify status was updated
	instance := instanceRepo.instances[1]
	if instance.Status != string(domainwf.StatePending) {
		t.Errorf("expected status PENDING, got %s", instance.Status)
	}

	// Verify history was created
	if len(historyRepo.histories) != 1 {
		t.Errorf("expected 1 history record, got %d", len(historyRepo.histories))
	}

	history := historyRepo.histories[0]
	if history.PreviousStatus != string(domainwf.StateCreated) {
		t.Errorf("expected previous status CREATED, got %s", history.PreviousStatus)
	}
	if history.NewStatus != string(domainwf.StatePending) {
		t.Errorf("expected new status PENDING, got %s", history.NewStatus)
	}

	// Verify event was dispatched
	if len(disp.events) != 1 {
		t.Errorf("expected 1 event dispatched, got %d", len(disp.events))
	}
}

func TestEngineTransitionStateInvalidTransition(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		instances: map[int64]*entity.ApprovalInstance{
			1: {ID: 1, Status: string(domainwf.StateCompleted)},
		},
	}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	// Try invalid transition from COMPLETED
	err := engine.TransitionState(context.Background(), 1, domainwf.TriggerSubmit)
	if err == nil {
		t.Error("expected error for invalid transition")
	}
}

func TestEngineTransitionStateTransactionFailure(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		instances: map[int64]*entity.ApprovalInstance{
			1: {ID: 1, Status: string(domainwf.StateCreated)},
		},
		updateErr: errors.New("update failed"),
	}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	err := engine.TransitionState(context.Background(), 1, domainwf.TriggerSubmit)
	if err == nil {
		t.Error("expected error when update fails")
	}
}

func TestEngineGetCurrentState(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		instances: map[int64]*entity.ApprovalInstance{
			1: {ID: 1, Status: string(domainwf.StateCreated)},
		},
	}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	state, err := engine.GetCurrentState(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state != domainwf.StateCreated {
		t.Errorf("expected state CREATED, got %s", state)
	}
}

func TestEngineHandleEvent(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  string
		event          *event.Event
		expectedStatus string
		expectError    bool
	}{
		{
			name:          "instance.created triggers START_AUDIT",
			initialStatus: string(domainwf.StateCreated),
			event: event.NewEvent(
				event.TypeInstanceCreated,
				1,
				"lark-123",
				map[string]interface{}{},
			),
			expectedStatus: string(domainwf.StateAIAuditing),
			expectError:    false,
		},
		{
			name:          "instance.approved triggers APPROVE",
			initialStatus: string(domainwf.StateInReview),
			event: event.NewEvent(
				event.TypeInstanceApproved,
				1,
				"lark-123",
				map[string]interface{}{},
			),
			expectedStatus: string(domainwf.StateApproved),
			expectError:    false,
		},
		{
			name:          "instance.rejected triggers REJECT",
			initialStatus: string(domainwf.StateInReview),
			event: event.NewEvent(
				event.TypeInstanceRejected,
				1,
				"lark-123",
				map[string]interface{}{},
			),
			expectedStatus: string(domainwf.StateRejected),
			expectError:    false,
		},
		{
			name:          "audit.completed moves to AI_AUDITED",
			initialStatus: string(domainwf.StateAIAuditing),
			event: event.NewEvent(
				event.TypeAuditCompleted,
				1,
				"lark-123",
				map[string]interface{}{},
			),
			expectedStatus: string(domainwf.StateAIAudited),
			expectError:    false,
		},
		{
			name:          "voucher.generated triggers COMPLETE_VOUCHER",
			initialStatus: string(domainwf.StateVoucherGenerating),
			event: event.NewEvent(
				event.TypeVoucherGenerated,
				1,
				"lark-123",
				map[string]interface{}{},
			),
			expectedStatus: string(domainwf.StateCompleted),
			expectError:    false,
		},
		{
			name:          "status.changed does not trigger transition",
			initialStatus: string(domainwf.StateCreated),
			event: event.NewEvent(
				event.TypeStatusChanged,
				1,
				"lark-123",
				map[string]interface{}{},
			),
			expectedStatus: string(domainwf.StateCreated),
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceRepo := &mockInstanceRepo{
				instances: map[int64]*entity.ApprovalInstance{
					1: {ID: 1, Status: tt.initialStatus},
				},
			}
			historyRepo := &mockHistoryRepo{}
			txManager := &mockTxManager{}

			engine := NewEngine(instanceRepo, historyRepo, txManager)

			err := engine.HandleEvent(context.Background(), tt.event)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			instance := instanceRepo.instances[1]
			if instance.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, instance.Status)
			}
		})
	}
}

func TestEngineHandleEventNil(t *testing.T) {
	instanceRepo := &mockInstanceRepo{instances: make(map[int64]*entity.ApprovalInstance)}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	engine := NewEngine(instanceRepo, historyRepo, txManager)

	err := engine.HandleEvent(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil event")
	}
}

func TestEngineCacheExpiry(t *testing.T) {
	instanceRepo := &mockInstanceRepo{
		instances: map[int64]*entity.ApprovalInstance{
			1: {ID: 1, Status: string(domainwf.StateCreated)},
		},
	}
	historyRepo := &mockHistoryRepo{}
	txManager := &mockTxManager{}

	// Set very short cache expiry
	engine := NewEngine(
		instanceRepo,
		historyRepo,
		txManager,
		WithCacheExpiry(1*time.Millisecond),
	)

	// Get machine first time
	_, err := engine.GetStateMachine(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(2 * time.Millisecond)

	// Update instance status directly
	instanceRepo.instances[1].Status = string(domainwf.StatePending)

	// Get machine again - should fetch from repo with updated state
	machine, err := engine.GetStateMachine(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if machine.State() != domainwf.StatePending {
		t.Errorf("expected state PENDING after cache expiry, got %s", machine.State())
	}
}
