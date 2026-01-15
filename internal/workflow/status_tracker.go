package workflow

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"go.uber.org/zap"
)

// StatusTracker manages status transitions
type StatusTracker struct {
	db           *database.DB
	instanceRepo *repository.InstanceRepository
	historyRepo  *repository.HistoryRepository
	logger       *zap.Logger
}

// NewStatusTracker creates a new status tracker
func NewStatusTracker(
	db *database.DB,
	instanceRepo *repository.InstanceRepository,
	historyRepo *repository.HistoryRepository,
	logger *zap.Logger,
) *StatusTracker {
	return &StatusTracker{
		db:           db,
		instanceRepo: instanceRepo,
		historyRepo:  historyRepo,
		logger:       logger,
	}
}

// UpdateStatus updates the status of an instance with audit trail
func (s *StatusTracker) UpdateStatus(instanceID int64, newStatus string, actionData interface{}) error {
	// Get current instance
	instance, err := s.instanceRepo.GetByID(instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance not found: %d", instanceID)
	}

	previousStatus := instance.Status

	// Check if status transition is valid
	if !s.isValidTransition(previousStatus, newStatus) {
		s.logger.Warn("Invalid status transition",
			zap.Int64("instance_id", instanceID),
			zap.String("from", previousStatus),
			zap.String("to", newStatus))
		return fmt.Errorf("invalid status transition from %s to %s", previousStatus, newStatus)
	}

	// Marshal action data
	actionDataJSON, err := json.Marshal(actionData)
	if err != nil {
		return fmt.Errorf("failed to marshal action data: %w", err)
	}

	// Update in transaction
	return s.db.WithTransaction(func(tx *sql.Tx) error {
		// Update instance status
		if err := s.instanceRepo.UpdateStatus(tx, instanceID, newStatus); err != nil {
			return fmt.Errorf("failed to update status: %w", err)
		}

		// Create audit record
		history := &models.ApprovalHistory{
			InstanceID:     instanceID,
			PreviousStatus: previousStatus,
			NewStatus:      newStatus,
			ActionType:     models.ActionTypeStatusChange,
			ActionData:     string(actionDataJSON),
		}
		if err := s.historyRepo.Create(tx, history); err != nil {
			return fmt.Errorf("failed to create history: %w", err)
		}

		s.logger.Info("Status updated",
			zap.Int64("instance_id", instanceID),
			zap.String("from", previousStatus),
			zap.String("to", newStatus))

		return nil
	})
}

// isValidTransition checks if a status transition is valid
func (s *StatusTracker) isValidTransition(from, to string) bool {
	// Define valid state transitions
	validTransitions := map[string][]string{
		models.StatusCreated: {
			models.StatusPending,
			models.StatusAIAuditing,
			models.StatusRejected,
		},
		models.StatusPending: {
			models.StatusAIAuditing,
			models.StatusInReview,
			models.StatusApproved,
			models.StatusRejected,
		},
		models.StatusAIAuditing: {
			models.StatusAIAudited,
			models.StatusRejected,
		},
		models.StatusAIAudited: {
			models.StatusInReview,
			models.StatusAutoApproved,
			models.StatusApproved,
			models.StatusRejected,
		},
		models.StatusInReview: {
			models.StatusApproved,
			models.StatusRejected,
		},
		models.StatusAutoApproved: {
			models.StatusApproved,
			models.StatusVoucherGenerating,
		},
		models.StatusApproved: {
			models.StatusVoucherGenerating,
			models.StatusCompleted,
		},
		models.StatusVoucherGenerating: {
			models.StatusCompleted,
			models.StatusApproved, // Retry
		},
		models.StatusRejected: {
			// Terminal state - no transitions allowed
		},
		models.StatusCompleted: {
			// Terminal state - no transitions allowed
		},
	}

	// Allow same status (idempotent)
	if from == to {
		return true
	}

	allowedNextStates, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, allowed := range allowedNextStates {
		if allowed == to {
			return true
		}
	}

	return false
}

// GetTransitionHistory gets the complete transition history for an instance
func (s *StatusTracker) GetTransitionHistory(instanceID int64) ([]*models.ApprovalHistory, error) {
	return s.historyRepo.GetByInstanceID(instanceID)
}
