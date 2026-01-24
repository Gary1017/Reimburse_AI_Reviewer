package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/garyjia/ai-reimbursement/internal/application/service"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

// Handlers contains all HTTP request handlers
type Handlers struct {
	approvalService service.ApprovalService
	auditService    service.AuditService
	voucherService  service.VoucherService
	logger          Logger
}

// NewHandlers creates a new Handlers instance
func NewHandlers(
	approvalService service.ApprovalService,
	auditService service.AuditService,
	voucherService service.VoucherService,
	logger Logger,
) *Handlers {
	return &Handlers{
		approvalService: approvalService,
		auditService:    auditService,
		voucherService:  voucherService,
		logger:          logger,
	}
}

// Response represents a standard JSON response
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

// InstanceResponse represents an approval instance in API responses
type InstanceResponse struct {
	ID              int64   `json:"id"`
	LarkInstanceID  string  `json:"lark_instance_id"`
	Status          string  `json:"status"`
	ApplicantUserID string  `json:"applicant_user_id,omitempty"`
	Department      string  `json:"department,omitempty"`
	SubmissionTime  string  `json:"submission_time"`
	ApprovalTime    *string `json:"approval_time,omitempty"`
	AIAuditResult   string  `json:"ai_audit_result,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// AuditResponse represents the audit result in API responses
type AuditResponse struct {
	InstanceID  int64   `json:"instance_id"`
	OverallPass bool    `json:"overall_pass"`
	Confidence  float64 `json:"confidence"`
	Reasoning   string  `json:"reasoning"`
}

// VoucherResponse represents the voucher generation result in API responses
type VoucherResponse struct {
	InstanceID      int64    `json:"instance_id"`
	Success         bool     `json:"success"`
	FolderPath      string   `json:"folder_path,omitempty"`
	VoucherFilePath string   `json:"voucher_file_path,omitempty"`
	AttachmentPaths []string `json:"attachment_paths,omitempty"`
}

// ListInstancesRequest represents query parameters for listing instances
type ListInstancesRequest struct {
	Limit  int `form:"limit"`
	Offset int `form:"offset"`
}

// HealthCheck handles GET /health
func (h *Handlers) HealthCheck(c *gin.Context) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   "1.0.0",
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    response,
	})
}

// ListInstances handles GET /api/instances
func (h *Handlers) ListInstances(c *gin.Context) {
	var req ListInstancesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		h.logger.Error("Invalid query parameters", "error", err)
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid query parameters",
		})
		return
	}

	// Set defaults
	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 20
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	instances, err := h.approvalService.ListInstances(c.Request.Context(), req.Limit, req.Offset)
	if err != nil {
		h.logger.Error("Failed to list instances", "error", err)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to retrieve instances",
		})
		return
	}

	// Convert to response format
	var responseInstances []InstanceResponse
	for _, instance := range instances {
		responseInstances = append(responseInstances, toInstanceResponse(instance))
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    responseInstances,
	})
}

// GetInstance handles GET /api/instances/:id
func (h *Handlers) GetInstance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid instance ID", "id", idStr, "error", err)
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid instance ID",
		})
		return
	}

	instance, err := h.approvalService.GetInstance(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get instance", "id", id, "error", err)
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "instance not found",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    toInstanceResponse(instance),
	})
}

// TriggerAudit handles POST /api/instances/:id/audit
func (h *Handlers) TriggerAudit(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid instance ID", "id", idStr, "error", err)
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid instance ID",
		})
		return
	}

	// Check if instance exists
	instance, err := h.approvalService.GetInstance(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Instance not found for audit", "id", id, "error", err)
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "instance not found",
		})
		return
	}

	h.logger.Info("Triggering audit", "instance_id", id, "lark_instance_id", instance.LarkInstanceID)

	// Perform audit
	result, err := h.auditService.AuditInstance(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Audit failed", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "audit failed: " + err.Error(),
		})
		return
	}

	response := AuditResponse{
		InstanceID:  id,
		OverallPass: result.OverallPass,
		Confidence:  result.Confidence,
		Reasoning:   result.Reasoning,
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    response,
	})
}

// GenerateVoucher handles POST /api/instances/:id/voucher
func (h *Handlers) GenerateVoucher(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid instance ID", "id", idStr, "error", err)
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid instance ID",
		})
		return
	}

	// Check if instance exists
	_, err = h.approvalService.GetInstance(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Instance not found for voucher", "id", id, "error", err)
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "instance not found",
		})
		return
	}

	h.logger.Info("Generating voucher", "instance_id", id)

	// Generate voucher
	result, err := h.voucherService.GenerateVoucher(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Voucher generation failed", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "voucher generation failed: " + err.Error(),
		})
		return
	}

	response := VoucherResponse{
		InstanceID:      id,
		Success:         result.Success,
		FolderPath:      result.FolderPath,
		VoucherFilePath: result.VoucherFilePath,
		AttachmentPaths: result.AttachmentPaths,
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    response,
	})
}

// toInstanceResponse converts domain entity to API response
func toInstanceResponse(instance *entity.ApprovalInstance) InstanceResponse {
	resp := InstanceResponse{
		ID:              instance.ID,
		LarkInstanceID:  instance.LarkInstanceID,
		Status:          instance.Status,
		ApplicantUserID: instance.ApplicantUserID,
		Department:      instance.Department,
		SubmissionTime:  instance.SubmissionTime.Format(time.RFC3339),
		AIAuditResult:   instance.AIAuditResult,
		CreatedAt:       instance.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       instance.UpdatedAt.Format(time.RFC3339),
	}

	if instance.ApprovalTime != nil {
		approvalTime := instance.ApprovalTime.Format(time.RFC3339)
		resp.ApprovalTime = &approvalTime
	}

	return resp
}
