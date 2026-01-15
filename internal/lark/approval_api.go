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
