package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/garyjia/ai-reimbursement/internal/application/port"
	"github.com/garyjia/ai-reimbursement/internal/domain/entity"
	"go.uber.org/zap"
)

// OAuthTokenRepository implements port.OAuthTokenRepository
type OAuthTokenRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewOAuthTokenRepository creates a new OAuth token repository
func NewOAuthTokenRepository(db *sql.DB, logger *zap.Logger) port.OAuthTokenRepository {
	return &OAuthTokenRepository{
		db:     db,
		logger: logger,
	}
}

// Upsert creates or updates an OAuth token for a user
func (r *OAuthTokenRepository) Upsert(ctx context.Context, token *entity.OAuthToken) error {
	query := `
		INSERT INTO oauth_tokens (
			user_id, access_token, refresh_token,
			access_token_expires_at, refresh_token_expires_at, scope
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			access_token = excluded.access_token,
			refresh_token = excluded.refresh_token,
			access_token_expires_at = excluded.access_token_expires_at,
			refresh_token_expires_at = excluded.refresh_token_expires_at,
			scope = excluded.scope,
			updated_at = CURRENT_TIMESTAMP
	`

	result, err := r.getExecutor(ctx).ExecContext(ctx, query,
		token.UserID,
		token.AccessToken,
		token.RefreshToken,
		token.AccessTokenExpiresAt,
		token.RefreshTokenExpiresAt,
		token.Scope,
	)
	if err != nil {
		r.logger.Error("Failed to upsert OAuth token",
			zap.String("user_id", token.UserID),
			zap.Error(err))
		return fmt.Errorf("failed to upsert OAuth token: %w", err)
	}

	// If this was an insert, get the ID
	if token.ID == 0 {
		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert id: %w", err)
		}
		token.ID = id
		token.CreatedAt = time.Now()
	}
	token.UpdatedAt = time.Now()

	return nil
}

// GetByUserID retrieves an OAuth token by user ID
func (r *OAuthTokenRepository) GetByUserID(ctx context.Context, userID string) (*entity.OAuthToken, error) {
	query := `
		SELECT id, user_id, access_token, refresh_token,
			access_token_expires_at, refresh_token_expires_at, scope,
			created_at, updated_at
		FROM oauth_tokens
		WHERE user_id = ?
	`

	var token entity.OAuthToken

	err := r.getExecutor(ctx).QueryRowContext(ctx, query, userID).Scan(
		&token.ID,
		&token.UserID,
		&token.AccessToken,
		&token.RefreshToken,
		&token.AccessTokenExpiresAt,
		&token.RefreshTokenExpiresAt,
		&token.Scope,
		&token.CreatedAt,
		&token.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		r.logger.Error("Failed to get OAuth token by user ID",
			zap.String("user_id", userID),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get OAuth token: %w", err)
	}

	return &token, nil
}

// getExecutor returns appropriate executor based on context
func (r *OAuthTokenRepository) getExecutor(ctx context.Context) executor {
	if tx, ok := ctx.Value(contextKey("tx")).(*sql.Tx); ok {
		return tx
	}
	return r.db
}

// Verify interface compliance
var _ port.OAuthTokenRepository = (*OAuthTokenRepository)(nil)
