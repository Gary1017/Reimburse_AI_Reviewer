package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// DeterministicCheckService runs deterministic (non-AI) validation checks
type DeterministicCheckService interface {
	RunChecks(ctx context.Context, instanceID int64) (*DeterministicCheckResult, error)
}

// DeterministicCheckResult holds the outcome of all deterministic checks
type DeterministicCheckResult struct {
	Passed     bool
	Violations []CheckViolation
}

// CheckViolation represents a single check failure
type CheckViolation struct {
	CheckType   string // COMPANY_NAME, VAT_NUMBER, MISSING_INVOICE, DUPLICATE_INVOICE, DATE_INVALID
	Severity    string // HARD_REJECT
	ItemID      int64
	InvoiceID   int64
	Description string // Chinese language
}

type deterministicCheckServiceImpl struct {
	itemRepo       port.ItemRepository
	invoiceV2Repo  port.InvoiceV2Repository
	attachmentRepo port.AttachmentRepository
	instanceRepo   port.InstanceRepository
	companyName    string
	companyTaxID   string
	logger         Logger
}

// NewDeterministicCheckService creates a new deterministic check service
func NewDeterministicCheckService(
	itemRepo port.ItemRepository,
	invoiceV2Repo port.InvoiceV2Repository,
	attachmentRepo port.AttachmentRepository,
	instanceRepo port.InstanceRepository,
	companyName string,
	companyTaxID string,
	logger Logger,
) DeterministicCheckService {
	return &deterministicCheckServiceImpl{
		itemRepo:       itemRepo,
		invoiceV2Repo:  invoiceV2Repo,
		attachmentRepo: attachmentRepo,
		instanceRepo:   instanceRepo,
		companyName:    companyName,
		companyTaxID:   companyTaxID,
		logger:         logger,
	}
}

// RunChecks executes all deterministic checks for an instance
func (s *deterministicCheckServiceImpl) RunChecks(ctx context.Context, instanceID int64) (*DeterministicCheckResult, error) {
	s.logger.Info("Running deterministic checks", "instance_id", instanceID)

	var violations []CheckViolation

	// Load data
	invoices, err := s.invoiceV2Repo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get invoices: %w", err)
	}

	items, err := s.itemRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	attachments, err := s.attachmentRepo.GetByInstanceIDAndStatus(ctx, instanceID, entity.AttachmentStatusProcessed)
	if err != nil {
		return nil, fmt.Errorf("get processed attachments: %w", err)
	}

	// Check 1: Company name and VAT number
	companyViolations := s.checkCompanyAndVAT(invoices)
	violations = append(violations, companyViolations...)

	// Check 2: Missing invoices
	missingViolations := s.checkMissingInvoices(items, attachments)
	violations = append(violations, missingViolations...)

	// Check 3: Duplicate invoices
	dupViolations, err := s.checkDuplicateInvoices(ctx, instanceID, invoices)
	if err != nil {
		s.logger.Error("Duplicate invoice check failed", "error", err, "instance_id", instanceID)
		// Don't fail entire check, just log the error
	} else {
		violations = append(violations, dupViolations...)
	}

	// Check 4: Date validation
	dateViolations := s.checkDates(items, invoices, attachments)
	violations = append(violations, dateViolations...)

	result := &DeterministicCheckResult{
		Passed:     len(violations) == 0,
		Violations: violations,
	}

	s.logger.Info("Deterministic checks completed",
		"instance_id", instanceID,
		"passed", result.Passed,
		"violation_count", len(violations))

	return result, nil
}

// checkCompanyAndVAT verifies buyer name and tax ID match company config
func (s *deterministicCheckServiceImpl) checkCompanyAndVAT(invoices []*entity.InvoiceV2) []CheckViolation {
	var violations []CheckViolation

	for _, inv := range invoices {
		if s.companyName != "" && strings.TrimSpace(inv.BuyerName) != "" {
			if strings.TrimSpace(inv.BuyerName) != strings.TrimSpace(s.companyName) {
				violations = append(violations, CheckViolation{
					CheckType:   "COMPANY_NAME",
					Severity:    "HARD_REJECT",
					InvoiceID:   inv.ID,
					Description: fmt.Sprintf("发票抬头与公司名称不一致（发票: %s, 公司: %s）", inv.BuyerName, s.companyName),
				})
			}
		}

		if s.companyTaxID != "" && strings.TrimSpace(inv.BuyerTaxID) != "" {
			if strings.TrimSpace(inv.BuyerTaxID) != strings.TrimSpace(s.companyTaxID) {
				violations = append(violations, CheckViolation{
					CheckType:   "VAT_NUMBER",
					Severity:    "HARD_REJECT",
					InvoiceID:   inv.ID,
					Description: fmt.Sprintf("发票税号与公司税号不一致（发票: %s, 公司: %s）", inv.BuyerTaxID, s.companyTaxID),
				})
			}
		}
	}

	return violations
}

// checkMissingInvoices verifies each item has at least one linked invoice attachment
func (s *deterministicCheckServiceImpl) checkMissingInvoices(items []*entity.ReimbursementItem, attachments []*entity.Attachment) []CheckViolation {
	var violations []CheckViolation

	// Build set of item IDs that have linked attachments
	itemsWithAttachment := make(map[int64]bool)
	for _, att := range attachments {
		if att.ItemID > 0 && att.FileType == entity.FileTypeInvoice {
			itemsWithAttachment[att.ItemID] = true
		}
	}

	for _, item := range items {
		if !itemsWithAttachment[item.ID] {
			violations = append(violations, CheckViolation{
				CheckType:   "MISSING_INVOICE",
				Severity:    "HARD_REJECT",
				ItemID:      item.ID,
				Description: fmt.Sprintf("报销项目缺少对应发票（项目: %s, 金额: %.2f元）", item.Description, item.Amount),
			})
		}
	}

	return violations
}

// checkDuplicateInvoices verifies no invoice is used in another approved/completed instance
func (s *deterministicCheckServiceImpl) checkDuplicateInvoices(ctx context.Context, instanceID int64, invoices []*entity.InvoiceV2) ([]CheckViolation, error) {
	var violations []CheckViolation

	for _, inv := range invoices {
		if inv.UniqueID == "" {
			continue
		}

		existing, err := s.invoiceV2Repo.GetByUniqueID(ctx, inv.UniqueID)
		if err != nil {
			return nil, fmt.Errorf("check unique ID %s: %w", inv.UniqueID, err)
		}

		if existing == nil || existing.ID == inv.ID || existing.InstanceID == instanceID {
			continue
		}

		// Check if the other instance is approved or completed
		otherInstance, err := s.instanceRepo.GetByID(ctx, existing.InstanceID)
		if err != nil {
			s.logger.Error("Failed to get instance for duplicate check",
				"error", err, "instance_id", existing.InstanceID)
			continue
		}

		if otherInstance != nil &&
			(otherInstance.Status == entity.StatusApproved || otherInstance.Status == entity.StatusCompleted) {
			violations = append(violations, CheckViolation{
				CheckType:   "DUPLICATE_INVOICE",
				Severity:    "HARD_REJECT",
				InvoiceID:   inv.ID,
				Description: fmt.Sprintf("发票重复使用（已在其他报销单中使用，发票号: %s-%s）", inv.InvoiceCode, inv.InvoiceNumber),
			})
		}
	}

	return violations, nil
}

// checkDates verifies invoice dates are not before item expense dates
func (s *deterministicCheckServiceImpl) checkDates(items []*entity.ReimbursementItem, invoices []*entity.InvoiceV2, attachments []*entity.Attachment) []CheckViolation {
	var violations []CheckViolation

	// Build invoice lookup by attachment ID
	invoiceByAttachment := make(map[int64]*entity.InvoiceV2)
	for _, inv := range invoices {
		invoiceByAttachment[inv.AttachmentID] = inv
	}

	// Build attachment lookup by item ID
	attachmentsByItem := make(map[int64][]*entity.Attachment)
	for _, att := range attachments {
		if att.ItemID > 0 {
			attachmentsByItem[att.ItemID] = append(attachmentsByItem[att.ItemID], att)
		}
	}

	for _, item := range items {
		if item.ExpenseDate == nil {
			continue
		}

		itemAttachments := attachmentsByItem[item.ID]
		for _, att := range itemAttachments {
			inv := invoiceByAttachment[att.ID]
			if inv == nil || inv.InvoiceDate == nil {
				continue
			}

			// Check that invoice date >= expense date
			if inv.InvoiceDate.Before(*item.ExpenseDate) {
				violations = append(violations, CheckViolation{
					CheckType:   "DATE_INVALID",
					Severity:    "HARD_REJECT",
					ItemID:      item.ID,
					InvoiceID:   inv.ID,
					Description: fmt.Sprintf("发票日期早于费用发生日期（发票日期: %s, 费用日期: %s）",
						inv.InvoiceDate.Format("2006-01-02"),
						item.ExpenseDate.Format("2006-01-02")),
				})
			}
		}
	}

	return violations
}
