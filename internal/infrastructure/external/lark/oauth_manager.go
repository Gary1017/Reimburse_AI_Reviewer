package lark

import (
	"context"
	"fmt"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkauthen "github.com/larksuite/oapi-sdk-go/v3/service/authen/v1"
	"go.uber.org/zap"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
)

const (
	// tokenRefreshBuffer is the time buffer before token expiry to trigger refresh
	tokenRefreshBuffer = 5 * time.Minute
)

// OAuthManager manages OAuth tokens for Lark Mail API
type OAuthManager struct {
	client    *lark.Client
	tokenRepo port.OAuthTokenRepository
	logger    *zap.Logger
	mu        sync.Mutex
}

// NewOAuthManager creates a new OAuth token manager
func NewOAuthManager(client *lark.Client, tokenRepo port.OAuthTokenRepository, logger *zap.Logger) *OAuthManager {
	return &OAuthManager{
		client:    client,
		tokenRepo: tokenRepo,
		logger:    logger,
	}
}

// ExchangeCode exchanges an authorization code for access and refresh tokens
func (m *OAuthManager) ExchangeCode(ctx context.Context, code string) (*entity.OAuthToken, error) {
	codePrefix := code
	if len(codePrefix) > 10 {
		codePrefix = codePrefix[:10] + "..."
	}
	m.logger.Info("Exchanging authorization code for tokens",
		zap.String("code_prefix", codePrefix))

	// Build request to exchange code for tokens
	req := larkauthen.NewCreateAccessTokenReqBuilder().
		Body(larkauthen.NewCreateAccessTokenReqBodyBuilder().
			GrantType("authorization_code").
			Code(code).
			Build()).
		Build()

	// Call Lark API
	resp, err := m.client.Authen.AccessToken.Create(ctx, req)
	if err != nil {
		m.logger.Error("Failed to exchange authorization code",
			zap.Error(err))
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Check for API errors
	if !resp.Success() {
		m.logger.Error("Lark API returned error",
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return nil, fmt.Errorf("lark API error: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// Extract token data from response (all fields are pointers)
	if resp.Data == nil {
		return nil, fmt.Errorf("empty response data from Lark API")
	}

	accessToken := ""
	if resp.Data.AccessToken != nil {
		accessToken = *resp.Data.AccessToken
	}

	refreshToken := ""
	if resp.Data.RefreshToken != nil {
		refreshToken = *resp.Data.RefreshToken
	}

	userID := ""
	if resp.Data.UserId != nil {
		userID = *resp.Data.UserId
	}

	expiresIn := int64(0)
	if resp.Data.ExpiresIn != nil {
		expiresIn = int64(*resp.Data.ExpiresIn)
	}

	refreshExpiresIn := int64(0)
	if resp.Data.RefreshExpiresIn != nil {
		refreshExpiresIn = int64(*resp.Data.RefreshExpiresIn)
	}

	if accessToken == "" || refreshToken == "" || userID == "" {
		return nil, fmt.Errorf("incomplete token data from Lark API")
	}

	// Create token entity
	now := time.Now()
	token := &entity.OAuthToken{
		UserID:                userID,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  now.Add(time.Duration(expiresIn) * time.Second),
		RefreshTokenExpiresAt: now.Add(time.Duration(refreshExpiresIn) * time.Second),
		Scope:                 "", // Scope is optional
	}

	// Save to database
	if err := m.tokenRepo.Upsert(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	m.logger.Info("Successfully exchanged code and saved token",
		zap.String("user_id", userID),
		zap.Time("access_expires_at", token.AccessTokenExpiresAt),
		zap.Time("refresh_expires_at", token.RefreshTokenExpiresAt))

	return token, nil
}

// GetAccessToken returns a valid access token for the user (refreshing if needed)
func (m *OAuthManager) GetAccessToken(ctx context.Context, userID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Retrieve token from database
	token, err := m.tokenRepo.GetByUserID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get token from database: %w", err)
	}

	if token == nil {
		return "", fmt.Errorf("no token found for user %s", userID)
	}

	// Check if refresh token is expired
	if token.IsRefreshTokenExpired() {
		m.logger.Error("Refresh token expired, user needs to re-authorize",
			zap.String("user_id", userID),
			zap.Time("refresh_expired_at", token.RefreshTokenExpiresAt))
		return "", fmt.Errorf("refresh token expired, user must re-authorize")
	}

	// Check if access token needs refresh (with buffer)
	if token.IsAccessTokenExpired(tokenRefreshBuffer) {
		m.logger.Info("Access token expired or expiring soon, refreshing",
			zap.String("user_id", userID),
			zap.Time("access_expires_at", token.AccessTokenExpiresAt))

		newToken, err := m.refreshToken(ctx, token)
		if err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		token = newToken
	}

	return token.AccessToken, nil
}

// SaveToken saves or updates a token (implements OAuthTokenProvider.SaveToken)
func (m *OAuthManager) SaveToken(ctx context.Context, userID string, accessToken string, refreshToken string, accessExpiresIn int64, refreshExpiresIn int64) error {
	now := time.Now()
	token := &entity.OAuthToken{
		UserID:                userID,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  now.Add(time.Duration(accessExpiresIn) * time.Second),
		RefreshTokenExpiresAt: now.Add(time.Duration(refreshExpiresIn) * time.Second),
		Scope:                 "",
	}

	if err := m.tokenRepo.Upsert(ctx, token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	m.logger.Info("Token saved successfully",
		zap.String("user_id", userID),
		zap.Time("access_expires_at", token.AccessTokenExpiresAt))

	return nil
}

// refreshToken refreshes an expired access token using the refresh token
func (m *OAuthManager) refreshToken(ctx context.Context, token *entity.OAuthToken) (*entity.OAuthToken, error) {
	m.logger.Info("Refreshing access token",
		zap.String("user_id", token.UserID))

	// Build refresh request
	req := larkauthen.NewCreateRefreshAccessTokenReqBuilder().
		Body(larkauthen.NewCreateRefreshAccessTokenReqBodyBuilder().
			GrantType("refresh_token").
			RefreshToken(token.RefreshToken).
			Build()).
		Build()

	// Call Lark API
	resp, err := m.client.Authen.RefreshAccessToken.Create(ctx, req)
	if err != nil {
		m.logger.Error("Failed to refresh access token",
			zap.String("user_id", token.UserID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to refresh access token: %w", err)
	}

	// Check for API errors
	if !resp.Success() {
		m.logger.Error("Lark API returned error on token refresh",
			zap.String("user_id", token.UserID),
			zap.Int("code", resp.Code),
			zap.String("msg", resp.Msg))
		return nil, fmt.Errorf("lark API error on refresh: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	// Extract new token data (all fields are pointers)
	if resp.Data == nil {
		return nil, fmt.Errorf("empty response data from Lark API")
	}

	accessToken := ""
	if resp.Data.AccessToken != nil {
		accessToken = *resp.Data.AccessToken
	}

	refreshToken := ""
	if resp.Data.RefreshToken != nil {
		refreshToken = *resp.Data.RefreshToken
	}

	expiresIn := int64(0)
	if resp.Data.ExpiresIn != nil {
		expiresIn = int64(*resp.Data.ExpiresIn)
	}

	refreshExpiresIn := int64(0)
	if resp.Data.RefreshExpiresIn != nil {
		refreshExpiresIn = int64(*resp.Data.RefreshExpiresIn)
	}

	if accessToken == "" || refreshToken == "" {
		return nil, fmt.Errorf("incomplete token data from Lark API")
	}

	// Update token entity
	now := time.Now()
	token.AccessToken = accessToken
	token.RefreshToken = refreshToken
	token.AccessTokenExpiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	token.RefreshTokenExpiresAt = now.Add(time.Duration(refreshExpiresIn) * time.Second)

	// Save updated token to database
	if err := m.tokenRepo.Upsert(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to save refreshed token: %w", err)
	}

	m.logger.Info("Successfully refreshed and saved token",
		zap.String("user_id", token.UserID),
		zap.Time("access_expires_at", token.AccessTokenExpiresAt),
		zap.Time("refresh_expires_at", token.RefreshTokenExpiresAt))

	return token, nil
}

// Verify interface compliance
var _ port.OAuthTokenProvider = (*OAuthManager)(nil)
