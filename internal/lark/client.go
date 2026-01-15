package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"go.uber.org/zap"
)

// Client wraps the Lark SDK client
type Client struct {
	client       *lark.Client
	approvalCode string // Approval definition code for subscription
	logger       *zap.Logger
}

// Config holds Lark client configuration
type Config struct {
	AppID        string
	AppSecret    string
	ApprovalCode string // Unique approval definition code
}

// NewClient creates a new Lark client
func NewClient(cfg Config, logger *zap.Logger) *Client {
	client := lark.NewClient(cfg.AppID, cfg.AppSecret,
		lark.WithLogLevel(larkcore.LogLevelInfo),
		lark.WithEnableTokenCache(true),
	)

	return &Client{
		client:       client,
		approvalCode: cfg.ApprovalCode,
		logger:       logger,
	}
}

// GetClient returns the underlying Lark SDK client
func (c *Client) GetClient() *lark.Client {
	return c.client
}

// GetApprovalCode returns the approval definition code
func (c *Client) GetApprovalCode() string {
	return c.approvalCode
}

// GetAccessToken retrieves the access token
func (c *Client) GetAccessToken(ctx context.Context) (string, error) {
	// The SDK handles token caching internally
	token, err := c.client.Auth.GetTenantAccessToken(ctx)
	if err != nil {
		c.logger.Error("Failed to get access token", zap.Error(err))
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	return token, nil
}
