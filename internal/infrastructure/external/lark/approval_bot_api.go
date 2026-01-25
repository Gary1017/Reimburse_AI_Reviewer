package lark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// ApprovalBotAPI handles Lark approval bot message operations
// ARCH-012: AI Audit Result Notification via Lark Approval Bot
type ApprovalBotAPI struct {
	client *SDKClient
	logger *zap.Logger
}

// NewApprovalBotAPI creates a new approval bot API handler
func NewApprovalBotAPI(client *SDKClient, logger *zap.Logger) *ApprovalBotAPI {
	return &ApprovalBotAPI{
		client: client,
		logger: logger,
	}
}

// imMessageRequest represents the request body for IM message API
type imMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
}

// imMessageResponse represents the response from IM message API
type imMessageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		MessageID string `json:"message_id"`
	} `json:"data"`
}

// SendAuditResultMessage sends an audit result notification to an approver
// Uses Lark IM API: POST /open-apis/im/v1/messages?receive_id_type=open_id
func (a *ApprovalBotAPI) SendAuditResultMessage(ctx context.Context, req *entity.AuditNotificationRequest) (*entity.AuditNotificationResponse, error) {
	if req.OpenID == "" {
		return nil, fmt.Errorf("open_id is required")
	}

	// Build interactive card message
	cardContent := a.buildInteractiveCard(req)
	contentBytes, err := json.Marshal(cardContent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal card content: %w", err)
	}

	// Build IM message request
	payload := imMessageRequest{
		ReceiveID: req.OpenID,
		MsgType:   "interactive",
		Content:   string(contentBytes),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	a.logger.Debug("Sending IM bot message",
		zap.String("instance_code", req.InstanceCode),
		zap.String("open_id", req.OpenID),
		zap.String("decision", req.AuditResult.Decision))

	// Get access token
	token, err := a.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	// Send HTTP request to IM API
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id",
		bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		a.logger.Error("Failed to send message",
			zap.String("instance_code", req.InstanceCode),
			zap.Error(err))
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var msgResp imMessageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check for errors
	if msgResp.Code != 0 {
		a.logger.Error("IM API returned error",
			zap.Int("code", msgResp.Code),
			zap.String("msg", msgResp.Msg),
			zap.String("response", string(respBody)))
		return &entity.AuditNotificationResponse{
			Success:      false,
			ErrorCode:    msgResp.Code,
			ErrorMessage: msgResp.Msg,
		}, nil
	}

	a.logger.Info("Audit notification sent successfully",
		zap.String("instance_code", req.InstanceCode),
		zap.String("message_id", msgResp.Data.MessageID))

	return &entity.AuditNotificationResponse{
		Success:   true,
		MessageID: msgResp.Data.MessageID,
	}, nil
}

// buildInteractiveCard builds a Lark interactive card for audit notification
func (a *ApprovalBotAPI) buildInteractiveCard(req *entity.AuditNotificationRequest) map[string]interface{} {
	result := req.AuditResult

	// Determine header color based on decision
	var headerTemplate string
	var headerTitle string
	switch result.Decision {
	case entity.AuditDecisionPass:
		headerTemplate = "green"
		headerTitle = "AI审核通过"
	case entity.AuditDecisionNeedsReview:
		headerTemplate = "orange"
		headerTitle = "AI审核：需人工复核"
	case entity.AuditDecisionFail:
		headerTemplate = "red"
		headerTitle = "AI审核不通过"
	default:
		headerTemplate = "blue"
		headerTitle = "AI审核结果"
	}

	// Build violations text
	violationsText := ""
	if len(result.Violations) > 0 {
		violationsText = "**发现问题:**\n"
		for _, v := range result.Violations {
			violationsText += fmt.Sprintf("• %s\n", v)
		}
	}

	// Build card elements
	elements := []interface{}{
		// Amount and confidence row
		map[string]interface{}{
			"tag": "div",
			"fields": []map[string]interface{}{
				{
					"is_short": true,
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**报销金额**\n%.2f", result.TotalAmount),
					},
				},
				{
					"is_short": true,
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**置信度**\n%d%%", int(result.Confidence*100)),
					},
				},
			},
		},
		// Divider
		map[string]interface{}{
			"tag": "hr",
		},
	}

	// Add violations if any
	if violationsText != "" {
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": violationsText,
			},
		})
	}

	// Add instance info
	elements = append(elements, map[string]interface{}{
		"tag": "note",
		"elements": []map[string]interface{}{
			{
				"tag":     "plain_text",
				"content": fmt.Sprintf("审批单号: %s", req.InstanceCode),
			},
		},
	})

	// Build complete card
	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": headerTemplate,
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": headerTitle,
			},
		},
		"elements": elements,
	}
}

// getAccessToken retrieves the tenant access token from Lark
func (a *ApprovalBotAPI) getAccessToken(ctx context.Context) (string, error) {
	// Build request for internal tenant access token endpoint
	reqBody := map[string]string{
		"app_id":     a.client.appID,
		"app_secret": a.client.appSecret,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	var tokenResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal token response: %w", err)
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("token request failed: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.TenantAccessToken, nil
}
