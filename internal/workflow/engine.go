package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/pkg/database"
	"go.uber.org/zap"
)

// Engine orchestrates the approval workflow
type Engine struct {
	db                *database.DB
	instanceRepo      *repository.InstanceRepository
	historyRepo       *repository.HistoryRepository
	itemRepo          *repository.ReimbursementItemRepository
	attachmentRepo    *repository.AttachmentRepository
	statusTracker     *StatusTracker
	approvalAPI       *lark.ApprovalAPI
	formParser        *lark.FormParser
	attachmentHandler *lark.AttachmentHandler
	logger            *zap.Logger
}

// NewEngine creates a new workflow engine
func NewEngine(
	db *database.DB,
	instanceRepo *repository.InstanceRepository,
	historyRepo *repository.HistoryRepository,
	itemRepo *repository.ReimbursementItemRepository,
	attachmentRepo *repository.AttachmentRepository,
	approvalAPI *lark.ApprovalAPI,
	attachmentHandler *lark.AttachmentHandler,
	logger *zap.Logger,
) *Engine {
	statusTracker := NewStatusTracker(db, instanceRepo, historyRepo, logger)
	formParser := lark.NewFormParser(logger)

	return &Engine{
		db:                db,
		instanceRepo:      instanceRepo,
		historyRepo:       historyRepo,
		itemRepo:          itemRepo,
		attachmentRepo:    attachmentRepo,
		statusTracker:     statusTracker,
		approvalAPI:       approvalAPI,
		formParser:        formParser,
		attachmentHandler: attachmentHandler,
		logger:            logger,
	}
}

// HandleInstanceCreated handles the creation of a new approval instance
// Implements ARCH-005: Non-blocking attachment integration into workflow
func (e *Engine) HandleInstanceCreated(instanceID string, eventData map[string]interface{}) error {
	ctx := context.Background()

	// Check if instance already exists (idempotency)
	existing, err := e.instanceRepo.GetByLarkInstanceID(instanceID)
	if err != nil {
		return fmt.Errorf("failed to check existing instance: %w", err)
	}
	if existing != nil {
		e.logger.Info("Instance already exists, skipping creation", zap.String("instance_id", instanceID))
		return nil
	}

	// Get detailed information from Lark API
	detail, err := e.approvalAPI.GetInstanceDetail(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance detail: %w", err)
	}

	// Parse form data
	formDataJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to marshal form data: %w", err)
	}

	// Extract applicant user ID
	applicantUserID := ""
	if detail.UserId != nil {
		applicantUserID = *detail.UserId
	}

	// Parse reimbursement items from form data
	reimbursementItems, err := e.formParser.Parse(string(formDataJSON))
	if err != nil {
		e.logger.Warn("Failed to parse reimbursement items from form data",
			zap.String("instance_id", instanceID),
			zap.Error(err))
		// Don't fail the instance creation, but log for debugging
		reimbursementItems = nil
	}

	// Extract attachments from form data (ARCH-005: non-blocking)
	attachmentRefs, attachErr := e.attachmentHandler.ExtractAttachmentURLs(string(formDataJSON))
	if attachErr != nil {
		e.logger.Warn("Failed to extract attachments from form data",
			zap.String("instance_id", instanceID),
			zap.Error(attachErr))
		// Don't fail the workflow, attachments are secondary
	}

	// Create instance in database
	instance := &models.ApprovalInstance{
		LarkInstanceID:  instanceID,
		Status:          models.StatusCreated,
		ApplicantUserID: applicantUserID,
		SubmissionTime:  time.Now(),
		FormData:        string(formDataJSON),
	}

	return e.db.WithTransaction(func(tx *sql.Tx) error {
		if err := e.instanceRepo.Create(tx, instance); err != nil {
			return fmt.Errorf("failed to create instance: %w", err)
		}

		// Create audit record
		history := &models.ApprovalHistory{
			InstanceID:     instance.ID,
			PreviousStatus: "",
			NewStatus:      models.StatusCreated,
			ActionType:     models.ActionTypeWebhook,
			ActionData:     string(formDataJSON),
		}
		if err := e.historyRepo.Create(tx, history); err != nil {
			return fmt.Errorf("failed to create history: %w", err)
		}

		// Save parsed reimbursement items if available
		if reimbursementItems != nil && len(reimbursementItems) > 0 {
			itemCount := 0
			for _, item := range reimbursementItems {
				item.InstanceID = instance.ID
				if err := e.itemRepo.Create(tx, item); err != nil {
					// Fail fast on item save error to maintain data integrity
					// This rolls back the entire transaction
					return fmt.Errorf("failed to save reimbursement item (instance_id=%d, description=%s): %w",
						instance.ID, item.Description, err)
				}
				itemCount++
			}
			e.logger.Info("Saved reimbursement items",
				zap.Int64("instance_id", instance.ID),
				zap.Int("count", itemCount))
		} else {
			e.logger.Warn("No reimbursement items parsed or saved",
				zap.Int64("instance_id", instance.ID))
		}

		// Process attachments (ARCH-005: async, non-blocking)
		if attachmentRefs != nil && len(attachmentRefs) > 0 {
			attachmentCount := 0
			for idx, ref := range attachmentRefs {
				// Link to item if available
				// ARCH-005: Skip attachments without items to prevent FK constraint violations
				if len(reimbursementItems) == 0 {
					e.logger.Warn("Skipping attachment creation: no reimbursement items to link to",
						zap.Int64("instance_id", instance.ID),
						zap.String("attachment_name", ref.OriginalName))
					continue
				}

				ref.ItemID = reimbursementItems[idx%len(reimbursementItems)].ID

			// Create attachment record in PENDING status
			attachment := &models.Attachment{
				ItemID:         ref.ItemID,
				InstanceID:     instance.ID,
				FileName:       ref.OriginalName,
				URL:            ref.URL, // Store download URL from Lark API
				DownloadStatus: models.AttachmentStatusPending,
				CreatedAt:      time.Now(),
			}

				if err := e.attachmentRepo.Create(tx, attachment); err != nil {
					e.logger.Error("Failed to create attachment record",
						zap.Int64("instance_id", instance.ID),
						zap.Int64("item_id", ref.ItemID),
						zap.String("file_name", ref.OriginalName),
						zap.Error(err))
					// Don't fail the whole transaction for attachment creation
					continue
				}
				attachmentCount++
			}
			e.logger.Info("Created attachment records",
				zap.Int64("instance_id", instance.ID),
				zap.Int("count", attachmentCount))
		} else if attachmentRefs != nil {
			e.logger.Info("No attachments found in form",
				zap.Int64("instance_id", instance.ID))
		}

		e.logger.Info("Created approval instance",
			zap.Int64("id", instance.ID),
			zap.String("lark_instance_id", instanceID),
			zap.String("applicant", applicantUserID))

		return nil
	})
}

// HandleStatusChanged handles status change events
func (e *Engine) HandleStatusChanged(instanceID string, eventData map[string]interface{}) error {
	e.logger.Info("Handling status changed event",
		zap.String("instance_id", instanceID),
		zap.Any("event_data", eventData))

	instance, err := e.instanceRepo.GetByLarkInstanceID(instanceID)
	if err != nil {
		e.logger.Error("Failed to get instance", zap.String("instance_id", instanceID), zap.Error(err))
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		e.logger.Info("Instance not found, creating it first", zap.String("instance_id", instanceID))
		// Instance not yet created, create it first
		return e.HandleInstanceCreated(instanceID, eventData)
	}

	e.logger.Info("Found existing instance",
		zap.Int64("db_id", instance.ID),
		zap.String("lark_instance_id", instanceID),
		zap.String("current_status", instance.Status))

	// Extract new status from event data
	newStatus, ok := eventData["status"].(string)
	if !ok {
		// Try alternative field names that Lark might use
		if statusObj, ok := eventData["status"].(map[string]interface{}); ok {
			if statusStr, ok := statusObj["status"].(string); ok {
				newStatus = statusStr
			} else if statusStr, ok := statusObj["type"].(string); ok {
				newStatus = statusStr
			}
		}
		
		if newStatus == "" {
			e.logger.Warn("Status not found in event data",
				zap.String("instance_id", instanceID),
				zap.Any("event_data_keys", getMapKeys(eventData)))
			return nil
		}
	}

	e.logger.Info("Extracted status from event",
		zap.String("instance_id", instanceID),
		zap.String("lark_status", newStatus))

	// Map Lark status to internal status
	internalStatus := mapLarkStatus(newStatus)

	e.logger.Info("Mapped status",
		zap.String("instance_id", instanceID),
		zap.String("lark_status", newStatus),
		zap.String("internal_status", internalStatus))

	// Update status
	if err := e.statusTracker.UpdateStatus(instance.ID, internalStatus, eventData); err != nil {
		e.logger.Error("Failed to update status",
			zap.String("instance_id", instanceID),
			zap.String("new_status", internalStatus),
			zap.Error(err))
		return err
	}

	e.logger.Info("Status changed successfully",
		zap.String("instance_id", instanceID),
		zap.String("new_status", internalStatus))

	return nil
}

// getMapKeys returns all keys from a map (helper for logging)
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// HandleInstanceApproved handles approval events
func (e *Engine) HandleInstanceApproved(instanceID string, eventData map[string]interface{}) error {
	e.logger.Info("Handling instance approved event",
		zap.String("instance_id", instanceID),
		zap.Any("event_data", eventData))

	instance, err := e.instanceRepo.GetByLarkInstanceID(instanceID)
	if err != nil {
		e.logger.Error("Failed to get instance", zap.String("instance_id", instanceID), zap.Error(err))
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		e.logger.Error("Instance not found", zap.String("instance_id", instanceID))
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	previousStatus := instance.Status
	e.logger.Info("Found instance, updating status to APPROVED",
		zap.Int64("db_id", instance.ID),
		zap.String("lark_instance_id", instanceID),
		zap.String("current_status", previousStatus))

	// Update status to approved
	if err := e.statusTracker.UpdateStatus(instance.ID, models.StatusApproved, eventData); err != nil {
		e.logger.Error("Failed to update status to APPROVED",
			zap.String("instance_id", instanceID),
			zap.Error(err))
		return err
	}

	// Set approval time
	if err := e.instanceRepo.SetApprovalTime(nil, instance.ID, time.Now()); err != nil {
		e.logger.Error("Failed to set approval time",
			zap.String("instance_id", instanceID),
			zap.Error(err))
		return fmt.Errorf("failed to set approval time: %w", err)
	}

	e.logger.Info("Instance approved successfully - STATUS CHANGE",
		zap.Int64("id", instance.ID),
		zap.String("lark_instance_id", instanceID),
		zap.String("previous_status", previousStatus),
		zap.String("new_status", models.StatusApproved))

	// TODO: Trigger voucher generation (Phase 4)
	// This will be handled by the voucher generator

	return nil
}

// HandleInstanceRejected handles rejection events
func (e *Engine) HandleInstanceRejected(instanceID string, eventData map[string]interface{}) error {
	e.logger.Info("Handling instance rejected event",
		zap.String("instance_id", instanceID),
		zap.Any("event_data", eventData))

	instance, err := e.instanceRepo.GetByLarkInstanceID(instanceID)
	if err != nil {
		e.logger.Error("Failed to get instance", zap.String("instance_id", instanceID), zap.Error(err))
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		e.logger.Error("Instance not found", zap.String("instance_id", instanceID))
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	previousStatus := instance.Status
	e.logger.Info("Found instance, updating status to REJECTED",
		zap.Int64("db_id", instance.ID),
		zap.String("lark_instance_id", instanceID),
		zap.String("current_status", previousStatus))

	// Update status to rejected
	if err := e.statusTracker.UpdateStatus(instance.ID, models.StatusRejected, eventData); err != nil {
		e.logger.Error("Failed to update status to REJECTED",
			zap.String("instance_id", instanceID),
			zap.Error(err))
		return err
	}

	e.logger.Info("Instance rejected successfully - STATUS CHANGE",
		zap.Int64("id", instance.ID),
		zap.String("lark_instance_id", instanceID),
		zap.String("previous_status", previousStatus),
		zap.String("new_status", models.StatusRejected))

	return nil
}

// mapLarkStatus maps Lark status to internal status
func mapLarkStatus(larkStatus string) string {
	// Map Lark approval statuses to internal statuses
	// This mapping depends on Lark's actual status values
	statusMap := map[string]string{
		"PENDING":  models.StatusPending,
		"APPROVED": models.StatusApproved,
		"REJECTED": models.StatusRejected,
		"CANCELED": models.StatusRejected,
		"DELETED":  models.StatusRejected,
	}

	if internal, ok := statusMap[larkStatus]; ok {
		return internal
	}

	return models.StatusPending
}
