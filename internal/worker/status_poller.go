package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/lark"
	"github.com/garyjia/ai-reimbursement/internal/models"
	"github.com/garyjia/ai-reimbursement/internal/repository"
	"github.com/garyjia/ai-reimbursement/internal/workflow"
	"go.uber.org/zap"
)

// StatusPoller polls Lark API to detect approval status changes
// This is a fallback mechanism when webhooks are not available (e.g., localhost)
type StatusPoller struct {
	instanceRepo *repository.InstanceRepository
	approvalAPI  *lark.ApprovalAPI
	engine       *workflow.Engine
	logger       *zap.Logger

	// Configuration
	pollInterval time.Duration // How often to poll (default: 30 seconds)
	batchSize    int           // How many instances to check per poll (default: 50)

	// State
	mu        sync.RWMutex
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
}

// NewStatusPoller creates a new status poller
func NewStatusPoller(
	instanceRepo *repository.InstanceRepository,
	approvalAPI *lark.ApprovalAPI,
	engine *workflow.Engine,
	logger *zap.Logger,
) *StatusPoller {
	return &StatusPoller{
		instanceRepo: instanceRepo,
		approvalAPI:  approvalAPI,
		engine:       engine,
		logger:       logger,
		pollInterval: 30 * time.Second,
		batchSize:    50,
	}
}

// Start starts the status polling worker
func (p *StatusPoller) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning {
		return fmt.Errorf("status poller is already running")
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.isRunning = true
	p.startTime = time.Now()

	p.logger.Info("StatusPoller started",
		zap.Duration("poll_interval", p.pollInterval),
		zap.Int("batch_size", p.batchSize))

	go p.pollLoop()

	return nil
}

// Stop stops the status polling worker
func (p *StatusPoller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning {
		return
	}

	p.isRunning = false
	if p.cancel != nil {
		p.cancel()
	}

	p.logger.Info("StatusPoller stopped")
}

// Name returns the worker name for identification
func (p *StatusPoller) Name() string {
	return "StatusPoller"
}

// pollLoop runs the main polling loop
func (p *StatusPoller) pollLoop() {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Poll immediately on start
	p.pollStatusChanges()

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debug("Poll loop context cancelled")
			return

		case <-ticker.C:
			p.pollStatusChanges()
		}
	}
}

// pollStatusChanges polls Lark API for status changes
func (p *StatusPoller) pollStatusChanges() {
	// Get recent instances that might have changed status
	instances, err := p.instanceRepo.List(p.batchSize, 0)
	if err != nil {
		p.logger.Error("Failed to get instances for polling", zap.Error(err))
		return
	}

	if len(instances) == 0 {
		return
	}

	p.logger.Debug("Polling status for instances",
		zap.Int("count", len(instances)))

	ctx, cancel := context.WithTimeout(p.ctx, 10*time.Second)
	defer cancel()

	checkedCount := 0
	updatedCount := 0

	for _, instance := range instances {
		// Skip if already in final states
		if instance.Status == models.StatusApproved ||
			instance.Status == models.StatusRejected ||
			instance.Status == models.StatusCompleted {
			continue
		}

		// Query Lark API for current status
		larkInstance, err := p.approvalAPI.GetInstanceDetail(ctx, instance.LarkInstanceID)
		if err != nil {
			p.logger.Warn("Failed to get instance detail from Lark",
				zap.String("lark_instance_id", instance.LarkInstanceID),
				zap.Error(err))
			continue
		}

		checkedCount++

		// Map Lark status to internal status
		larkStatus := ""
		if larkInstance.Status != nil {
			larkStatus = *larkInstance.Status
		}

		internalStatus := p.mapLarkStatus(larkStatus)

		// Check if status changed
		if instance.Status != internalStatus {
			p.logger.Info("Status change detected via polling",
				zap.String("lark_instance_id", instance.LarkInstanceID),
				zap.String("old_status", instance.Status),
				zap.String("new_status", internalStatus),
				zap.String("lark_status", larkStatus))

			// Update status using workflow engine
			eventData := map[string]interface{}{
				"status":        larkStatus,
				"instance_code": instance.LarkInstanceID,
				"polled":        true, // Mark as polled, not webhook
			}

			if err := p.engine.HandleStatusChanged(instance.LarkInstanceID, eventData); err != nil {
				p.logger.Error("Failed to handle status change",
					zap.String("lark_instance_id", instance.LarkInstanceID),
					zap.Error(err))
				continue
			}

			updatedCount++
		}
	}

	if checkedCount > 0 {
		p.logger.Info("Status polling completed",
			zap.Int("checked", checkedCount),
			zap.Int("updated", updatedCount))
	}
}

// mapLarkStatus maps Lark status to internal status
func (p *StatusPoller) mapLarkStatus(larkStatus string) string {
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
