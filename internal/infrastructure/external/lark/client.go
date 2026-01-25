package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"go.uber.org/zap"
)

// Client implements port.LarkClient interface
type Client struct {
	sdkClient   *SDKClient
	approvalAPI *ApprovalAPI
	logger      *zap.Logger
}

// NewClient creates a new Lark client adapter
func NewClient(sdkClient *SDKClient, logger *zap.Logger) *Client {
	return &Client{
		sdkClient:   sdkClient,
		approvalAPI: NewApprovalAPI(sdkClient, logger),
		logger:      logger,
	}
}

// GetInstanceDetail retrieves detailed information about an approval instance
// Implements port.LarkClient interface
func (c *Client) GetInstanceDetail(ctx context.Context, instanceID string) (*port.LarkInstanceDetail, error) {
	// Use existing ApprovalAPI to get instance detail
	detail, err := c.approvalAPI.GetInstanceDetail(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance detail: %w", err)
	}

	if detail == nil {
		return nil, fmt.Errorf("instance not found: %s", instanceID)
	}

	// Map Lark SDK response to port DTO
	result := &port.LarkInstanceDetail{
		InstanceCode: derefString(detail.InstanceCode),
		ApprovalCode: derefString(detail.ApprovalCode),
		Status:       derefString(detail.Status),
	}

	// Extract user IDs
	if detail.UserId != nil {
		result.UserID = *detail.UserId
	}
	if detail.OpenId != nil {
		result.OpenID = *detail.OpenId
	}

	// Extract timestamps (convert from string to int64)
	if detail.StartTime != nil {
		if startTime, err := strconv.ParseInt(*detail.StartTime, 10, 64); err == nil {
			result.StartTime = startTime
		}
	}
	if detail.EndTime != nil {
		if endTime, err := strconv.ParseInt(*detail.EndTime, 10, 64); err == nil {
			result.EndTime = endTime
		}
	}

	// Serialize form data to JSON string
	if detail.Form != nil {
		formData, err := serializeFormData(detail.Form)
		if err != nil {
			c.logger.Warn("Failed to serialize form data",
				zap.String("instance_id", instanceID),
				zap.Error(err))
		} else {
			result.FormData = formData
		}
	}

	return result, nil
}

// GetApprovers retrieves approver information for an instance
// Implements port.LarkClient interface
func (c *Client) GetApprovers(ctx context.Context, instanceID string) ([]port.ApproverInfo, error) {
	// Use existing ApprovalAPI to get approvers
	approvers, err := c.approvalAPI.GetApproversForInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get approvers: %w", err)
	}

	// Map to port DTOs
	result := make([]port.ApproverInfo, 0, len(approvers))
	for _, approver := range approvers {
		result = append(result, port.ApproverInfo{
			UserID: approver.UserID,
			OpenID: approver.OpenID,
			Name:   approver.Name,
		})
	}

	return result, nil
}

// serializeFormData converts form data to JSON string
func serializeFormData(form interface{}) (string, error) {
	data, err := json.Marshal(form)
	if err != nil {
		return "", fmt.Errorf("failed to marshal form data: %w", err)
	}
	return string(data), nil
}

// derefString safely dereferences a string pointer
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
