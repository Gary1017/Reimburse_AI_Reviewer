package lark

import (
	"net/http"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"go.uber.org/zap"
)

// SDKClient wraps the Lark SDK client
type SDKClient struct {
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

// NewSDKClient creates a new Lark SDK client.
// An optional httpClient can be provided (e.g. go-vcr recorder) to override
// the default HTTP transport used by the Lark SDK.
func NewSDKClient(cfg Config, logger *zap.Logger, httpClient ...*http.Client) *SDKClient {
	opts := []lark.ClientOptionFunc{
		lark.WithLogLevel(larkcore.LogLevelInfo),
		lark.WithEnableTokenCache(true),
	}
	if len(httpClient) > 0 && httpClient[0] != nil {
		opts = append(opts, lark.WithHttpClient(httpClient[0]))
	}

	client := lark.NewClient(cfg.AppID, cfg.AppSecret, opts...)

	return &SDKClient{
		client:       client,
		approvalCode: cfg.ApprovalCode,
		appID:        cfg.AppID,
		appSecret:    cfg.AppSecret,
		logger:       logger,
	}
}

// GetClient returns the underlying Lark SDK client
func (c *SDKClient) GetClient() *lark.Client {
	return c.client
}

// GetApprovalCode returns the approval definition code
func (c *SDKClient) GetApprovalCode() string {
	return c.approvalCode
}

// GetAppID returns the app ID
func (c *SDKClient) GetAppID() string {
	return c.appID
}

// GetAppSecret returns the app secret
func (c *SDKClient) GetAppSecret() string {
	return c.appSecret
}
