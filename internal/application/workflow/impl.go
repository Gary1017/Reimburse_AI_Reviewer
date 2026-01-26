package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
	domainwf "github.com/garyjia/ai-reimbursement/internal/domain/workflow"
)

// engineImpl is the concrete implementation of WorkflowEngine
type engineImpl struct {
	instanceRepo port.InstanceRepository
	historyRepo  port.HistoryRepository
	txManager    port.TransactionManager
	dispatcher   dispatcher.Dispatcher

	// Cache state machines per instance
	mu          sync.RWMutex
	machines    map[int64]domainwf.StateMachine
	lastAccess  map[int64]time.Time
	cacheExpiry time.Duration
}

// EngineOption configures the workflow engine
type EngineOption func(*engineImpl)

// WithDispatcher sets the event dispatcher for emitting events
func WithDispatcher(d dispatcher.Dispatcher) EngineOption {
	return func(e *engineImpl) {
		e.dispatcher = d
	}
}

// WithCacheExpiry sets the cache expiry duration for state machines
func WithCacheExpiry(expiry time.Duration) EngineOption {
	return func(e *engineImpl) {
		e.cacheExpiry = expiry
	}
}

// NewEngine creates a new workflow engine
func NewEngine(
	instanceRepo port.InstanceRepository,
	historyRepo port.HistoryRepository,
	txManager port.TransactionManager,
	opts ...EngineOption,
) WorkflowEngine {
	e := &engineImpl{
		instanceRepo: instanceRepo,
		historyRepo:  historyRepo,
		txManager:    txManager,
		machines:     make(map[int64]domainwf.StateMachine),
		lastAccess:   make(map[int64]time.Time),
		cacheExpiry:  30 * time.Minute, // Default cache expiry
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// HandleEvent processes a domain event through the workflow
func (e *engineImpl) HandleEvent(ctx context.Context, evt *event.Event) error {
	if evt == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Resolve InstanceID from LarkInstanceID if not set
	instanceID := evt.InstanceID
	if instanceID == 0 && evt.LarkInstanceID != "" {
		instance, err := e.instanceRepo.GetByLarkInstanceID(ctx, evt.LarkInstanceID)
		if err != nil {
			return fmt.Errorf("failed to resolve instance from Lark ID %s: %w", evt.LarkInstanceID, err)
		}
		if instance == nil {
			return fmt.Errorf("instance not found for Lark ID %s", evt.LarkInstanceID)
		}
		instanceID = instance.ID
	}

	if instanceID == 0 {
		return fmt.Errorf("event has no instance ID or Lark instance ID")
	}

	// Map event type to trigger
	trigger, err := e.mapEventToTrigger(evt)
	if err != nil {
		return err
	}

	// If no trigger mapping, skip (not all events drive state transitions)
	if trigger == "" {
		return nil
	}

	// Execute the state transition
	return e.TransitionState(ctx, instanceID, trigger)
}

// GetStateMachine returns a state machine for an instance (creates if not cached)
func (e *engineImpl) GetStateMachine(ctx context.Context, instanceID int64) (domainwf.StateMachine, error) {
	// Check cache first
	e.mu.RLock()
	machine, exists := e.machines[instanceID]
	lastAccess := e.lastAccess[instanceID]
	e.mu.RUnlock()

	// Check if cached machine is still valid
	if exists && time.Since(lastAccess) < e.cacheExpiry {
		e.mu.Lock()
		e.lastAccess[instanceID] = time.Now()
		e.mu.Unlock()
		return machine, nil
	}

	// Fetch instance from repository
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch instance: %w", err)
	}

	// Create state machine with current state
	currentState := domainwf.State(instance.Status)
	if !currentState.IsValid() {
		return nil, fmt.Errorf("invalid state in instance: %s", instance.Status)
	}

	machine = BuildReimbursementStateMachine(currentState)

	// Update cache
	e.mu.Lock()
	e.machines[instanceID] = machine
	e.lastAccess[instanceID] = time.Now()
	e.mu.Unlock()

	return machine, nil
}

// TransitionState triggers a state transition for an instance
func (e *engineImpl) TransitionState(ctx context.Context, instanceID int64, trigger domainwf.Trigger) error {
	// Get state machine
	machine, err := e.GetStateMachine(ctx, instanceID)
	if err != nil {
		return err
	}

	// Get current state before transition
	previousState := machine.State()

	// Check if transition is allowed
	if !machine.CanFire(trigger) {
		return fmt.Errorf("transition not allowed: trigger %s from state %s", trigger, previousState)
	}

	// Execute transition within transaction
	err = e.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		// Fire the state machine
		if err := machine.Fire(txCtx, trigger); err != nil {
			return fmt.Errorf("state machine fire failed: %w", err)
		}

		newState := machine.State()

		// Update instance status
		if err := e.instanceRepo.UpdateStatus(txCtx, instanceID, newState.String()); err != nil {
			return fmt.Errorf("failed to update instance status: %w", err)
		}

		// Create history record
		history := &entity.ApprovalHistory{
			InstanceID:     instanceID,
			ReviewerUserID: "system", // Default to system, can be overridden by caller
			PreviousStatus: previousState.String(),
			NewStatus:      newState.String(),
			ActionType:     trigger.String(),
			ActionData:     "",
			Timestamp:      time.Now(),
		}

		if err := e.historyRepo.Create(txCtx, history); err != nil {
			return fmt.Errorf("failed to create history record: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Invalidate cache after successful transition
	e.mu.Lock()
	delete(e.machines, instanceID)
	delete(e.lastAccess, instanceID)
	e.mu.Unlock()

	// Emit status changed event if dispatcher is available
	if e.dispatcher != nil {
		statusEvent := event.NewEvent(
			event.TypeStatusChanged,
			instanceID,
			"", // LarkInstanceID not needed here
			map[string]interface{}{
				"previous_status": previousState.String(),
				"new_status":      machine.State().String(),
				"trigger":         trigger.String(),
			},
		)
		// Fire async to avoid blocking
		e.dispatcher.DispatchAsync(ctx, statusEvent)
	}

	return nil
}

// GetCurrentState returns the current state of an instance
func (e *engineImpl) GetCurrentState(ctx context.Context, instanceID int64) (domainwf.State, error) {
	machine, err := e.GetStateMachine(ctx, instanceID)
	if err != nil {
		return "", err
	}
	return machine.State(), nil
}

// mapEventToTrigger maps domain events to state machine triggers
func (e *engineImpl) mapEventToTrigger(evt *event.Event) (domainwf.Trigger, error) {
	switch evt.Type {
	case event.TypeInstanceCreated:
		// On creation, we can either submit or start audit depending on configuration
		// Default behavior: start audit immediately
		return domainwf.TriggerStartAudit, nil

	case event.TypeInstanceApproved:
		return domainwf.TriggerApprove, nil

	case event.TypeInstanceRejected:
		return domainwf.TriggerReject, nil

	case event.TypeAuditCompleted:
		// First complete the audit to move to AI_AUDITED state
		return domainwf.TriggerCompleteAudit, nil

	case event.TypeVoucherGenerated:
		return domainwf.TriggerCompleteVoucher, nil

	case event.TypeStatusChanged:
		// Status changed events don't trigger transitions (they're results of transitions)
		return "", nil

	case event.TypeAttachmentReady:
		// Attachment ready events don't directly trigger state transitions
		return "", nil

	default:
		return "", fmt.Errorf("unknown event type: %s", evt.Type)
	}
}
