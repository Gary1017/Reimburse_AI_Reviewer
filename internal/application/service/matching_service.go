package service

import (
	"context"
	"fmt"

	"github.com/garyjia/ai-reimbursement/internal/application/dispatcher"
	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/event"
)

// MatchingService manages invoice-to-item matching operations
type MatchingService interface {
	MatchInstance(ctx context.Context, instanceID int64) error
}

type matchingServiceImpl struct {
	itemRepo       port.ItemRepository
	invoiceV2Repo  port.InvoiceV2Repository
	attachmentRepo port.AttachmentRepository
	matcher        port.AIInvoiceMatcher
	txManager      port.TransactionManager
	dispatcher     dispatcher.Dispatcher
	logger         Logger
}

// NewMatchingService creates a new MatchingService
func NewMatchingService(
	itemRepo port.ItemRepository,
	invoiceV2Repo port.InvoiceV2Repository,
	attachmentRepo port.AttachmentRepository,
	matcher port.AIInvoiceMatcher,
	txManager port.TransactionManager,
	dispatcher dispatcher.Dispatcher,
	logger Logger,
) MatchingService {
	return &matchingServiceImpl{
		itemRepo:       itemRepo,
		invoiceV2Repo:  invoiceV2Repo,
		attachmentRepo: attachmentRepo,
		matcher:        matcher,
		txManager:      txManager,
		dispatcher:     dispatcher,
		logger:         logger,
	}
}

// MatchInstance performs AI-powered matching of invoices to reimbursement items
func (s *matchingServiceImpl) MatchInstance(ctx context.Context, instanceID int64) error {
	s.logger.Info("Starting invoice matching", "instance_id", instanceID)

	// Load all items
	items, err := s.itemRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get items", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get items: %w", err)
	}

	// Load all invoices
	invoices, err := s.invoiceV2Repo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		s.logger.Error("Failed to get invoices", "error", err, "instance_id", instanceID)
		return fmt.Errorf("get invoices: %w", err)
	}

	matchCount := 0

	// If no invoices, skip matching but still emit DataReady
	if len(invoices) == 0 {
		s.logger.Info("No invoices to match", "instance_id", instanceID)
	} else if s.matcher == nil {
		s.logger.Info("Matcher not configured, skipping matching", "instance_id", instanceID)
	} else {
		// Call AI matcher
		result, err := s.matcher.MatchInvoicesToItems(ctx, items, invoices)
		if err != nil {
			s.logger.Error("Failed to match invoices", "error", err, "instance_id", instanceID)
			return fmt.Errorf("match invoices: %w", err)
		}

		s.logger.Info("Matching completed",
			"instance_id", instanceID,
			"match_count", len(result.Matches),
			"confidence", result.Confidence,
			"reasoning", result.Reasoning)

		// Update attachments with matched item IDs
		if len(result.Matches) > 0 {
			err = s.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
				for _, match := range result.Matches {
					if err := s.attachmentRepo.UpdateItemID(txCtx, match.AttachmentID, match.ItemID); err != nil {
						return fmt.Errorf("update attachment %d item_id: %w", match.AttachmentID, err)
					}
					s.logger.Info("Matched invoice to item",
						"attachment_id", match.AttachmentID,
						"item_id", match.ItemID,
						"invoice_id", match.InvoiceID,
						"confidence", match.Confidence,
						"reasoning", match.Reasoning)
				}
				return nil
			})

			if err != nil {
				s.logger.Error("Failed to update attachment item IDs", "error", err, "instance_id", instanceID)
				return fmt.Errorf("update attachment item IDs: %w", err)
			}

			matchCount = len(result.Matches)
		}
	}

	// Emit DataReady event
	evt := event.NewEvent(event.TypeDataReady, instanceID, "", map[string]interface{}{
		"instance_id":  instanceID,
		"matches_count": matchCount,
	})
	s.dispatcher.DispatchAsync(ctx, evt)

	s.logger.Info("DataReady event dispatched",
		"instance_id", instanceID,
		"matches_count", matchCount)

	return nil
}
