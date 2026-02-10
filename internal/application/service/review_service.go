package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// ReviewService orchestrates the AI review pipeline
type ReviewService interface {
	ReviewInstance(ctx context.Context, instanceID int64) (*ReviewDecision, error)
}

// ReviewDecision holds the final decision and supporting data
type ReviewDecision struct {
	Decision                string
	Confidence              float64
	DeterministicViolations []CheckViolation
	PolicyResult            *port.PolicyAuditResult
	PriceResult             *port.PriceAuditResult
	Comment                 string
}

type reviewServiceImpl struct {
	instanceRepo   port.InstanceRepository
	itemRepo       port.ItemRepository
	invoiceV2Repo  port.InvoiceV2Repository
	attachmentRepo port.AttachmentRepository
	taskRepo       port.ApprovalTaskRepository
	deterChecks    DeterministicCheckService
	aiAuditor      port.AIAuditor
	commentGen     port.AICommentGenerator
	taskApprover   port.LarkTaskApprover
	txManager      port.TransactionManager
	approvalCode   string
	logger         Logger
}

// NewReviewService creates a new review service
func NewReviewService(
	instanceRepo port.InstanceRepository,
	itemRepo port.ItemRepository,
	invoiceV2Repo port.InvoiceV2Repository,
	attachmentRepo port.AttachmentRepository,
	taskRepo port.ApprovalTaskRepository,
	deterChecks DeterministicCheckService,
	aiAuditor port.AIAuditor,
	commentGen port.AICommentGenerator,
	taskApprover port.LarkTaskApprover,
	txManager port.TransactionManager,
	approvalCode string,
	logger Logger,
) ReviewService {
	return &reviewServiceImpl{
		instanceRepo:   instanceRepo,
		itemRepo:       itemRepo,
		invoiceV2Repo:  invoiceV2Repo,
		attachmentRepo: attachmentRepo,
		taskRepo:       taskRepo,
		deterChecks:    deterChecks,
		aiAuditor:      aiAuditor,
		commentGen:     commentGen,
		taskApprover:   taskApprover,
		txManager:      txManager,
		approvalCode:   approvalCode,
		logger:         logger,
	}
}

// ReviewInstance performs the full AI review for an instance
func (s *reviewServiceImpl) ReviewInstance(ctx context.Context, instanceID int64) (*ReviewDecision, error) {
	s.logger.Info("Starting AI review", "instance_id", instanceID)

	// Step 1: Verify instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get instance: %w", err)
	}
	if instance == nil {
		return nil, fmt.Errorf("instance %d not found", instanceID)
	}

	// Step 2: Get AI review task and mark IN_PROGRESS
	aiTask, err := s.taskRepo.GetAIReviewTask(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get AI review task: %w", err)
	}
	if aiTask == nil {
		return nil, fmt.Errorf("no AI review task found for instance %d", instanceID)
	}

	aiTask.Status = entity.TaskStatusInProgress
	aiTask.StartTime = time.Now().Format(time.RFC3339)
	aiTask.UpdatedAt = time.Now()
	if err := s.taskRepo.Update(ctx, aiTask); err != nil {
		return nil, fmt.Errorf("update AI task status: %w", err)
	}

	// Step 3: Run deterministic checks
	deterResult, err := s.deterChecks.RunChecks(ctx, instanceID)
	if err != nil {
		s.logger.Error("Deterministic checks failed", "error", err, "instance_id", instanceID)
		return nil, fmt.Errorf("deterministic checks: %w", err)
	}

	// Step 4: If deterministic checks fail, reject immediately
	if !deterResult.Passed {
		s.logger.Info("Deterministic checks failed, rejecting",
			"instance_id", instanceID,
			"violation_count", len(deterResult.Violations))

		return s.handleReject(ctx, instance, aiTask, deterResult.Violations, nil, nil)
	}

	// Step 5: Run AI audits on each item
	items, err := s.itemRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}

	invoices, err := s.invoiceV2Repo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("get invoices: %w", err)
	}

	var allPolicyResult *port.PolicyAuditResult
	var allPriceResult *port.PriceAuditResult
	var aiFindings []string
	overallCompliant := true
	overallReasonable := true
	minConfidence := 1.0

	// Build invoice lookup by item
	invoiceByItem := s.buildInvoiceByItemMap(invoices, items, ctx, instanceID)

	for _, item := range items {
		invoiceData := invoiceByItem[item.ID]

		// Policy audit
		if s.aiAuditor != nil {
			policyResult, err := s.aiAuditor.AuditPolicy(ctx, item, invoiceData)
			if err != nil {
				s.logger.Error("Policy audit failed for item",
					"error", err, "item_id", item.ID)
				aiFindings = append(aiFindings, fmt.Sprintf("政策审核失败: %s (%v)", item.Description, err))
				continue
			}

			if !policyResult.Compliant {
				overallCompliant = false
				for _, v := range policyResult.Violations {
					aiFindings = append(aiFindings, v)
				}
			}
			if policyResult.Confidence < minConfidence {
				minConfidence = policyResult.Confidence
			}
			allPolicyResult = policyResult

			// Price audit
			priceResult, err := s.aiAuditor.AuditPrice(ctx, item, invoiceData)
			if err != nil {
				s.logger.Error("Price audit failed for item",
					"error", err, "item_id", item.ID)
				aiFindings = append(aiFindings, fmt.Sprintf("价格审核失败: %s (%v)", item.Description, err))
				continue
			}

			if !priceResult.Reasonable {
				overallReasonable = false
				aiFindings = append(aiFindings, fmt.Sprintf("价格偏离市场价: %s (偏离%.1f%%)",
					item.Description, priceResult.DeviationPercentage))
			}
			if priceResult.Confidence < minConfidence {
				minConfidence = priceResult.Confidence
			}
			allPriceResult = priceResult
		}
	}

	// Step 6: Determine final decision
	if !overallCompliant {
		return s.handleReject(ctx, instance, aiTask, nil, aiFindings, &ReviewDecision{
			PolicyResult: allPolicyResult,
			PriceResult:  allPriceResult,
			Confidence:   minConfidence,
		})
	}

	if !overallReasonable || len(aiFindings) > 0 {
		return s.handleApproveWithWarnings(ctx, instance, aiTask, items, invoices, aiFindings, allPolicyResult, allPriceResult, minConfidence)
	}

	return s.handleApprove(ctx, instance, aiTask, items, invoices, allPolicyResult, allPriceResult, minConfidence)
}

// handleReject handles a rejection decision
func (s *reviewServiceImpl) handleReject(
	ctx context.Context,
	instance *entity.ApprovalInstance,
	aiTask *entity.ApprovalTask,
	deterViolations []CheckViolation,
	aiFindings []string,
	partial *ReviewDecision,
) (*ReviewDecision, error) {
	// Build violation strings for comment
	var violations []string
	for _, v := range deterViolations {
		violations = append(violations, v.Description)
	}
	violations = append(violations, aiFindings...)

	// Generate comment
	comment := s.generateComment(ctx, "REJECT", instance, violations, aiFindings)

	// Execute Lark reject
	if s.taskApprover != nil && aiTask.LarkTaskID != "" {
		if err := s.taskApprover.RejectTask(ctx, s.approvalCode, instance.LarkInstanceID,
			aiTask.AssigneeUserID, aiTask.LarkTaskID, comment); err != nil {
			s.logger.Error("Failed to reject task via Lark",
				"error", err, "task_id", aiTask.LarkTaskID)
		}
	}

	// Update AI task
	s.completeTask(ctx, aiTask, entity.DecisionRejected, comment, violations)

	decision := &ReviewDecision{
		Decision:                "REJECT",
		DeterministicViolations: deterViolations,
		Comment:                 comment,
	}
	if partial != nil {
		decision.PolicyResult = partial.PolicyResult
		decision.PriceResult = partial.PriceResult
		decision.Confidence = partial.Confidence
	}

	s.logger.Info("Instance rejected", "instance_id", instance.ID)
	return decision, nil
}

// handleApproveWithWarnings handles an approval with warnings
func (s *reviewServiceImpl) handleApproveWithWarnings(
	ctx context.Context,
	instance *entity.ApprovalInstance,
	aiTask *entity.ApprovalTask,
	items []*entity.ReimbursementItem,
	invoices []*entity.InvoiceV2,
	aiFindings []string,
	policyResult *port.PolicyAuditResult,
	priceResult *port.PriceAuditResult,
	confidence float64,
) (*ReviewDecision, error) {
	comment := s.generateComment(ctx, "APPROVE_WITH_WARNINGS", instance, nil, aiFindings)

	// Execute Lark approve (with warnings in comment)
	if s.taskApprover != nil && aiTask.LarkTaskID != "" {
		if err := s.taskApprover.ApproveTask(ctx, s.approvalCode, instance.LarkInstanceID,
			aiTask.AssigneeUserID, aiTask.LarkTaskID, comment); err != nil {
			s.logger.Error("Failed to approve task via Lark",
				"error", err, "task_id", aiTask.LarkTaskID)
		}
	}

	s.completeTask(ctx, aiTask, entity.DecisionApproved, comment, aiFindings)

	s.logger.Info("Instance approved with warnings", "instance_id", instance.ID)
	return &ReviewDecision{
		Decision:     "APPROVE_WITH_WARNINGS",
		Confidence:   confidence,
		PolicyResult: policyResult,
		PriceResult:  priceResult,
		Comment:      comment,
	}, nil
}

// handleApprove handles a clean approval
func (s *reviewServiceImpl) handleApprove(
	ctx context.Context,
	instance *entity.ApprovalInstance,
	aiTask *entity.ApprovalTask,
	items []*entity.ReimbursementItem,
	invoices []*entity.InvoiceV2,
	policyResult *port.PolicyAuditResult,
	priceResult *port.PriceAuditResult,
	confidence float64,
) (*ReviewDecision, error) {
	comment := s.generateComment(ctx, "APPROVE", instance, nil, nil)

	// Execute Lark approve
	if s.taskApprover != nil && aiTask.LarkTaskID != "" {
		if err := s.taskApprover.ApproveTask(ctx, s.approvalCode, instance.LarkInstanceID,
			aiTask.AssigneeUserID, aiTask.LarkTaskID, comment); err != nil {
			s.logger.Error("Failed to approve task via Lark",
				"error", err, "task_id", aiTask.LarkTaskID)
		}
	}

	s.completeTask(ctx, aiTask, entity.DecisionApproved, comment, nil)

	s.logger.Info("Instance approved", "instance_id", instance.ID)
	return &ReviewDecision{
		Decision:     "APPROVE",
		Confidence:   confidence,
		PolicyResult: policyResult,
		PriceResult:  priceResult,
		Comment:      comment,
	}, nil
}

// generateComment generates an audit comment, falling back to simple text if AI fails
func (s *reviewServiceImpl) generateComment(
	ctx context.Context,
	decision string,
	instance *entity.ApprovalInstance,
	violations []string,
	aiFindings []string,
) string {
	if s.commentGen == nil {
		return s.fallbackComment(decision, violations, aiFindings)
	}

	items, _ := s.itemRepo.GetByInstanceID(ctx, instance.ID)
	invoices, _ := s.invoiceV2Repo.GetByInstanceID(ctx, instance.ID)

	var totalAmount int64
	for _, item := range items {
		totalAmount += item.AmountCents
	}

	req := &port.CommentGenerationRequest{
		Decision:    decision,
		Items:       items,
		Invoices:    invoices,
		Violations:  violations,
		AIFindings:  aiFindings,
		TotalAmount: totalAmount,
	}

	comment, err := s.commentGen.GenerateComment(ctx, req)
	if err != nil {
		s.logger.Error("Comment generation failed, using fallback", "error", err)
		return s.fallbackComment(decision, violations, aiFindings)
	}

	return comment
}

// fallbackComment generates a simple comment without AI
func (s *reviewServiceImpl) fallbackComment(decision string, violations []string, aiFindings []string) string {
	switch decision {
	case "REJECT":
		if len(violations) > 0 {
			return "AI审核不通过：" + joinStrings(violations, "；")
		}
		return "AI审核不通过。"
	case "APPROVE_WITH_WARNINGS":
		if len(aiFindings) > 0 {
			return "AI审核通过（存在提醒事项）：" + joinStrings(aiFindings, "；")
		}
		return "AI审核通过（存在提醒事项）。"
	default:
		return "AI审核通过。"
	}
}

// completeTask updates the AI task with the decision result
func (s *reviewServiceImpl) completeTask(ctx context.Context, task *entity.ApprovalTask, decision, comment string, findings []string) {
	now := time.Now()
	task.Status = entity.TaskStatusCompleted
	task.Decision = decision
	task.EndTime = now.Format(time.RFC3339)
	task.UpdatedAt = now
	task.CompletedBy = "AI"

	if len(findings) > 0 {
		violationsJSON, _ := json.Marshal(findings)
		task.Violations = string(violationsJSON)
	}

	if err := s.taskRepo.Update(ctx, task); err != nil {
		s.logger.Error("Failed to update AI task with decision",
			"error", err, "task_id", task.ID)
	}
}

// buildInvoiceByItemMap builds a map of item ID -> InvoiceExtractionResult for AI audit
func (s *reviewServiceImpl) buildInvoiceByItemMap(
	invoices []*entity.InvoiceV2,
	items []*entity.ReimbursementItem,
	ctx context.Context,
	instanceID int64,
) map[int64]*port.InvoiceExtractionResult {
	result := make(map[int64]*port.InvoiceExtractionResult)

	// Build invoice by attachment map
	invoiceByAttachment := make(map[int64]*entity.InvoiceV2)
	for _, inv := range invoices {
		invoiceByAttachment[inv.AttachmentID] = inv
	}

	// Get processed attachments
	attachments, err := s.attachmentRepo.GetByInstanceIDAndStatus(ctx, instanceID, entity.AttachmentStatusProcessed)
	if err != nil {
		s.logger.Error("Failed to get attachments for invoice mapping", "error", err)
		return result
	}

	for _, att := range attachments {
		if att.ItemID == 0 {
			continue
		}
		inv := invoiceByAttachment[att.ID]
		if inv == nil {
			continue
		}

		invoiceDateStr := ""
		if inv.InvoiceDate != nil {
			invoiceDateStr = inv.InvoiceDate.Format("2006-01-02")
		}

		result[att.ItemID] = &port.InvoiceExtractionResult{
			Success:       true,
			InvoiceCode:   inv.InvoiceCode,
			InvoiceNumber: inv.InvoiceNumber,
			TotalAmount:   float64(inv.InvoiceAmountCents) / 100.0,
			InvoiceDate:   invoiceDateStr,
			SellerName:    inv.SellerName,
			SellerTaxID:   inv.SellerTaxID,
			BuyerName:     inv.BuyerName,
			BuyerTaxID:    inv.BuyerTaxID,
			Confidence:    1.0,
		}
	}

	return result
}

// joinStrings joins a slice of strings with a separator
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}
