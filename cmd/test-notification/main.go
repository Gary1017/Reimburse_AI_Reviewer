package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/config"
)

// Isolated test for Lark IM message sending
// This tests the notification functionality independently without full system integration

type accessTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

type imMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
}

type imMessageResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		MessageID string `json:"message_id"`
	} `json:"data"`
}

func main() {
	fmt.Println("=== Lark IM Notification Test ===")
	fmt.Println("This tool tests the ability to send messages via Lark IM API")
	fmt.Println()

	// Check for --retry flag to retry failed notifications
	if len(os.Args) > 1 && os.Args[1] == "--retry" {
		retryFailedNotifications()
		return
	}

	// Load config
	configPath := "configs/config.yaml"
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("App ID: %s...%s\n", cfg.Lark.AppID[:4], cfg.Lark.AppID[len(cfg.Lark.AppID)-4:])
	fmt.Printf("Approval Code: %s\n", cfg.Lark.ApprovalCode)

	ctx := context.Background()

	// Step 1: Get tenant access token
	fmt.Println("\n[Step 1] Getting tenant access token...")
	token, err := getTenantAccessToken(ctx, cfg.Lark.AppID, cfg.Lark.AppSecret)
	if err != nil {
		log.Fatalf("Failed to get access token: %v", err)
	}
	fmt.Printf("✓ Got access token: %s...%s\n", token[:10], token[len(token)-10:])

	// Step 2: Get a test open_id
	var openID string
	if len(os.Args) > 1 {
		arg := os.Args[1]
		// If arg starts with "ou_", it's an open_id
		if len(arg) > 3 && arg[:3] == "ou_" {
			openID = arg
			fmt.Printf("\n[Step 2] Using open_id from command line: %s\n", openID)
		} else {
			// Otherwise, treat it as an instance_id and fetch open_id from Lark
			fmt.Printf("\n[Step 2] Fetching open_id from instance: %s\n", arg)
			openID, err = getOpenIDFromInstance(ctx, token, arg)
			if err != nil {
				log.Fatalf("Failed to get open_id from instance: %v", err)
			}
			fmt.Printf("✓ Got open_id: %s\n", openID)
		}
	} else {
		// Try to get from latest instance in database
		fmt.Println("\n[Step 2] No argument provided. Trying to fetch from a recent instance...")
		fmt.Println("Usage: ./bin/test-notification <open_id or instance_id>")
		fmt.Println()
		fmt.Print("Enter open_id or instance_id: ")
		var input string
		fmt.Scanln(&input)
		if len(input) > 3 && input[:3] == "ou_" {
			openID = input
		} else if input != "" {
			openID, err = getOpenIDFromInstance(ctx, token, input)
			if err != nil {
				log.Fatalf("Failed to get open_id: %v", err)
			}
		}
	}

	if openID == "" {
		log.Fatal("No open_id provided. Cannot send test message.")
	}

	// Step 3: Send a simple text message
	fmt.Println("\n[Step 3] Sending simple text message...")
	msgID, err := sendTextMessage(ctx, token, openID, "测试消息：来自AI报销系统的通知测试")
	if err != nil {
		fmt.Printf("✗ Failed to send text message: %v\n", err)
	} else {
		fmt.Printf("✓ Text message sent! message_id: %s\n", msgID)
	}

	// Step 4: Try sending an interactive card message (like audit notification)
	fmt.Println("\n[Step 4] Sending interactive card message...")
	msgID, err = sendInteractiveCard(ctx, token, openID)
	if err != nil {
		fmt.Printf("✗ Failed to send card message: %v\n", err)
	} else {
		fmt.Printf("✓ Interactive card sent! message_id: %s\n", msgID)
	}

	fmt.Println("\n=== Test Complete ===")
}

func getTenantAccessToken(ctx context.Context, appID, appSecret string) (string, error) {
	reqBody := map[string]string{
		"app_id":     appID,
		"app_secret": appSecret,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Token response: %s\n", string(respBody))

	var tokenResp accessTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Code != 0 {
		return "", fmt.Errorf("token request failed: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.TenantAccessToken, nil
}

func sendTextMessage(ctx context.Context, token, openID, text string) (string, error) {
	content := map[string]string{"text": text}
	contentBytes, _ := json.Marshal(content)

	payload := imMessageRequest{
		ReceiveID: openID,
		MsgType:   "text",
		Content:   string(contentBytes),
	}
	body, _ := json.Marshal(payload)

	fmt.Printf("  Request body: %s\n", string(body))

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Response: %s\n", string(respBody))

	var msgResp imMessageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return "", err
	}

	if msgResp.Code != 0 {
		return "", fmt.Errorf("send message failed: code=%d, msg=%s", msgResp.Code, msgResp.Msg)
	}

	return msgResp.Data.MessageID, nil
}

func sendInteractiveCard(ctx context.Context, token, openID string) (string, error) {
	// Build an interactive card similar to the audit notification
	card := map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"template": "green",
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": "✅ AI审核通过 - 测试消息",
			},
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"fields": []map[string]interface{}{
					{
						"is_short": true,
						"text": map[string]interface{}{
							"tag":     "lark_md",
							"content": "**报销金额**\n¥1,234.56",
						},
					},
					{
						"is_short": true,
						"text": map[string]interface{}{
							"tag":     "lark_md",
							"content": "**置信度**\n95%",
						},
					},
				},
			},
			map[string]interface{}{
				"tag": "hr",
			},
			map[string]interface{}{
				"tag": "note",
				"elements": []map[string]interface{}{
					{
						"tag":     "plain_text",
						"content": fmt.Sprintf("测试时间: %s", time.Now().Format("2006-01-02 15:04:05")),
					},
				},
			},
		},
	}

	cardBytes, _ := json.Marshal(card)

	payload := imMessageRequest{
		ReceiveID: openID,
		MsgType:   "interactive",
		Content:   string(cardBytes),
	}
	body, _ := json.Marshal(payload)

	fmt.Printf("  Request body (card): %s\n", string(body)[:200]+"...")

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Response: %s\n", string(respBody))

	var msgResp imMessageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return "", err
	}

	if msgResp.Code != 0 {
		return "", fmt.Errorf("send card failed: code=%d, msg=%s", msgResp.Code, msgResp.Msg)
	}

	return msgResp.Data.MessageID, nil
}

// getOpenIDFromInstance fetches the instance detail from Lark and extracts an open_id
func getOpenIDFromInstance(ctx context.Context, token, instanceID string) (string, error) {
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/approval/v4/instances/%s", instanceID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("  Instance response: %s\n", string(respBody)[:minInt(500, len(respBody))]+"...")

	var instanceResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			UserID   string `json:"user_id"`
			OpenID   string `json:"open_id"`
			Timeline []struct {
				OpenID string `json:"open_id"`
				UserID string `json:"user_id"`
				Type   string `json:"type"`
			} `json:"timeline"`
			TaskList []struct {
				OpenID string `json:"open_id"`
				UserID string `json:"user_id"`
				Status string `json:"status"`
			} `json:"task_list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(respBody, &instanceResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal: %w", err)
	}

	if instanceResp.Code != 0 {
		return "", fmt.Errorf("API error: code=%d, msg=%s", instanceResp.Code, instanceResp.Msg)
	}

	// Try to get open_id from various sources
	// 1. From applicant
	if instanceResp.Data.OpenID != "" {
		fmt.Printf("  Found applicant open_id: %s\n", instanceResp.Data.OpenID)
		return instanceResp.Data.OpenID, nil
	}

	// 2. From timeline (approvers)
	for _, t := range instanceResp.Data.Timeline {
		if t.OpenID != "" {
			fmt.Printf("  Found timeline open_id: %s (type: %s)\n", t.OpenID, t.Type)
			return t.OpenID, nil
		}
	}

	// 3. From task_list (pending approvers)
	for _, task := range instanceResp.Data.TaskList {
		if task.OpenID != "" {
			fmt.Printf("  Found task open_id: %s (status: %s)\n", task.OpenID, task.Status)
			return task.OpenID, nil
		}
	}

	return "", fmt.Errorf("no open_id found in instance")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// retryFailedNotifications retries sending notifications for failed records
func retryFailedNotifications() {
	fmt.Println("=== Retry Failed Notifications ===")
	fmt.Println()

	// Load config
	configPath := "configs/config.yaml"
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()

	// Get token
	token, err := getTenantAccessToken(ctx, cfg.Lark.AppID, cfg.Lark.AppSecret)
	if err != nil {
		log.Fatalf("Failed to get access token: %v", err)
	}
	fmt.Printf("✓ Got access token\n\n")

	// Get a specific instance_id to retry (from the most recent failed notification)
	// Using instance 21 which was the most recent failed one
	instanceID := "C2E89722-D0D3-4770-929D-4010C2184FEC"
	if len(os.Args) > 2 {
		instanceID = os.Args[2]
	}

	fmt.Printf("Retrying notification for instance: %s\n", instanceID)

	// Get open_id from instance
	openID, err := getOpenIDFromInstance(ctx, token, instanceID)
	if err != nil {
		log.Fatalf("Failed to get open_id: %v", err)
	}
	fmt.Printf("✓ Got open_id: %s\n\n", openID)

	// Build and send audit result card (simulating what the real notifier sends)
	// This is a simplified version - the real one would fetch audit results from DB
	card := buildAuditResultCard("FAIL", 236.05, 0.9, []string{
		"发票类型不符：加油站发票用于住宿费报销",
		"重复发票：该发票已被使用",
	}, instanceID)

	cardBytes, _ := json.Marshal(card)

	payload := imMessageRequest{
		ReceiveID: openID,
		MsgType:   "interactive",
		Content:   string(cardBytes),
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=open_id",
		bytes.NewReader(body))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to send: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	fmt.Printf("Response: %s\n", string(respBody))

	var msgResp imMessageResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		log.Fatalf("Failed to parse response: %v", err)
	}

	if msgResp.Code != 0 {
		fmt.Printf("✗ Failed: code=%d, msg=%s\n", msgResp.Code, msgResp.Msg)
	} else {
		fmt.Printf("✓ Audit notification sent! message_id: %s\n", msgResp.Data.MessageID)
	}
}

// buildAuditResultCard builds a Lark interactive card for audit results
func buildAuditResultCard(decision string, amount float64, confidence float64, violations []string, instanceID string) map[string]interface{} {
	var headerTemplate, headerTitle string
	switch decision {
	case "PASS":
		headerTemplate = "green"
		headerTitle = "✅ AI审核通过"
	case "NEEDS_REVIEW":
		headerTemplate = "orange"
		headerTitle = "⚠️ AI审核：需人工复核"
	case "FAIL":
		headerTemplate = "red"
		headerTitle = "❌ AI审核不通过"
	default:
		headerTemplate = "blue"
		headerTitle = "AI审核结果"
	}

	elements := []interface{}{
		map[string]interface{}{
			"tag": "div",
			"fields": []map[string]interface{}{
				{
					"is_short": true,
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**报销金额**\n¥%.2f", amount),
					},
				},
				{
					"is_short": true,
					"text": map[string]interface{}{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**置信度**\n%d%%", int(confidence*100)),
					},
				},
			},
		},
		map[string]interface{}{
			"tag": "hr",
		},
	}

	// Add violations
	if len(violations) > 0 {
		violationsText := "**发现问题:**\n"
		for _, v := range violations {
			violationsText += fmt.Sprintf("• %s\n", v)
		}
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": violationsText,
			},
		})
	}

	elements = append(elements, map[string]interface{}{
		"tag": "note",
		"elements": []map[string]interface{}{
			{
				"tag":     "plain_text",
				"content": fmt.Sprintf("审批单号: %s", instanceID),
			},
		},
	})

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
