package entity

import "time"

// OAuthToken represents stored OAuth access and refresh tokens for Lark Mail API
type OAuthToken struct {
	ID                    int64     `json:"id"`
	UserID                string    `json:"user_id"`
	AccessToken           string    `json:"access_token"`
	RefreshToken          string    `json:"refresh_token"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
	Scope                 string    `json:"scope"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// IsAccessTokenExpired checks if the access token is expired or will expire within buffer duration
func (t *OAuthToken) IsAccessTokenExpired(buffer time.Duration) bool {
	return time.Now().Add(buffer).After(t.AccessTokenExpiresAt)
}

// IsRefreshTokenExpired checks if the refresh token is expired
func (t *OAuthToken) IsRefreshTokenExpired() bool {
	return time.Now().After(t.RefreshTokenExpiresAt)
}
