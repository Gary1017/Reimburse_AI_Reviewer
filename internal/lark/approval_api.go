package lark

import (
	"context"
	"fmt"

	larkApproval "github.com/larksuite/oapi-sdk-go/v3/service/approval/v4"
	"go.uber.org/zap"
)

// ApprovalAPI handles Lark approval-related operations
type ApprovalAPI struct {
	client *Client
	logger *zap.Logger
}

// NewApprovalAPI creates a new approval API handler
func NewApprovalAPI(client *Client, logger *zap.Logger) *ApprovalAPI {
	return &ApprovalAPI{
		client: client,
		logger: logger,
	}
}

// GetInstanceDetail retrieves detailed information about an approval instance
func (a *ApprovalAPI) GetInstanceDetail(ctx context.Context, instanceID string) (*larkApproval.GetInstanceRespData, error) {
	req := larkApproval.NewGetInstanceReqBuilder().
		InstanceId(instanceID).
		Build()

	resp, err := a.client.client.Approval.Instance.Get(ctx, req)
	if err != nil {
		a.logger.Error("Failed to get instance detail",
			zap.String("instance_id", instanceID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}

	if !resp.Success() {
		a.logger.Error("API returned failure",
			zap.String("instance_id", instanceID),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return nil, fmt.Errorf("API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	return resp.Data, nil
}

// DownloadFile downloads a file attachment from Lark
func (a *ApprovalAPI) DownloadFile(ctx context.Context, fileKey string) ([]byte, error) {
	// Use Lark drive API to download file
	// TODO: Implement file download using drive API
	a.logger.Warn("File download not yet implemented", zap.String("file_key", fileKey))
	return nil, fmt.Errorf("file download not implemented")
}

// SubscribeApprovalEvent subscribes to approval events for a specific approval code
// This must be called once before the app can receive approval events via webhook
// API: POST /open-apis/approval/v4/approvals/:approval_code/subscribe
// According to Lark docs: After subscribing events in developer console,
// you MUST call this API to enable receiving events for this approval_code
func (a *ApprovalAPI) SubscribeApprovalEvent(ctx context.Context, approvalCode string) error {
	if approvalCode == "" {
		return fmt.Errorf("approval code cannot be empty")
	}

	// Use SDK if available, otherwise fall back to direct HTTP call
	// Try SDK method first
	req := larkApproval.NewSubscribeApprovalReqBuilder().
		ApprovalCode(approvalCode).
		Build()

	resp, err := a.client.client.Approval.Approval.Subscribe(ctx, req)
	if err != nil {
		a.logger.Error("Failed to subscribe to approval events (SDK call failed)",
			zap.String("approval_code", approvalCode),
			zap.Error(err))
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	if !resp.Success() {
		// Check if already subscribed (error code 1390007)
		if resp.Code == 1390007 {
			a.logger.Info("Approval already subscribed (this is OK)",
				zap.String("approval_code", approvalCode),
				zap.String("message", resp.Msg))
			return nil // Not an error, already subscribed
		}

		a.logger.Error("API returned failure when subscribing",
			zap.String("approval_code", approvalCode),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return fmt.Errorf("subscription failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	a.logger.Info("Successfully subscribed to approval events",
		zap.String("approval_code", approvalCode))
	return nil
}

// UnsubscribeApprovalEvent unsubscribes from approval events
// API: POST /open-apis/approval/v4/approvals/:approval_code/unsubscribe
func (a *ApprovalAPI) UnsubscribeApprovalEvent(ctx context.Context, approvalCode string) error {
	if approvalCode == "" {
		return fmt.Errorf("approval code cannot be empty")
	}

	req := larkApproval.NewUnsubscribeApprovalReqBuilder().
		ApprovalCode(approvalCode).
		Build()

	resp, err := a.client.client.Approval.Approval.Unsubscribe(ctx, req)
	if err != nil {
		a.logger.Error("Failed to unsubscribe from approval events",
			zap.String("approval_code", approvalCode),
			zap.Error(err))
		return fmt.Errorf("failed to unsubscribe: %w", err)
	}

	if !resp.Success() {
		a.logger.Error("API returned failure when unsubscribing",
			zap.String("approval_code", approvalCode),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return fmt.Errorf("unsubscription failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	a.logger.Info("Successfully unsubscribed from approval events",
		zap.String("approval_code", approvalCode))
	return nil
}

// GetUserInfo retrieves user information
func (a *ApprovalAPI) GetUserInfo(ctx context.Context, userID string) (map[string]interface{}, error) {
	// TODO: Implement user info retrieval using contact API
	a.logger.Debug("Getting user info", zap.String("user_id", userID))

	// For now, return minimal info
	return map[string]interface{}{
		"user_id": userID,
		"name":    "User " + userID,
	}, nil
}
