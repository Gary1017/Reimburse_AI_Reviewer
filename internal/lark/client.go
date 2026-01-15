package lark

import (
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"go.uber.org/zap"
)

// Client wraps the Lark SDK client
type Client struct {
	client       *lark.Client
	approvalCode string // Approval definition code for subscription
	appID        string
	appSecret    string
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
		appID:        cfg.AppID,
		appSecret:    cfg.AppSecret,
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
