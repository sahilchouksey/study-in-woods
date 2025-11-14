-- Phase 3: Authentication & Authorization - Database Migrations
-- Add token_version to users table and create password_reset_tokens table

-- Add token_version column to users table (if not exists)
ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version INTEGER DEFAULT 0 NOT NULL;

-- Create password_reset_tokens table
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(100) NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create indexes for password_reset_tokens
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires_at ON password_reset_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_deleted_at ON password_reset_tokens(deleted_at);

-- Create indexes for jwt_token_blacklist (if not exists from Phase 0.5)
CREATE INDEX IF NOT EXISTS idx_jwt_token_blacklist_user_id ON jwt_token_blacklist(user_id);
CREATE INDEX IF NOT EXISTS idx_jwt_token_blacklist_expires_at ON jwt_token_blacklist(expires_at);

COMMENT ON TABLE password_reset_tokens IS 'Stores password reset tokens with expiration';
COMMENT ON COLUMN users.token_version IS 'Incremented to invalidate all user tokens';
