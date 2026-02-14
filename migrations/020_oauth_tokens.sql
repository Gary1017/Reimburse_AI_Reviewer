-- Migration: Add OAuth tokens table for storing Lark Mail API access tokens
-- Purpose: Enable automated email sending via Lark Mail API with OAuth 2.0
-- Phase: Voucher Generation (Phase 4)

CREATE TABLE IF NOT EXISTS oauth_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL UNIQUE,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    access_token_expires_at DATETIME NOT NULL,
    refresh_token_expires_at DATETIME NOT NULL,
    scope TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_user_id ON oauth_tokens(user_id);
