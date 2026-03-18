-- Generic user tokens table for email verification and password reset.
-- Tokens are single-use and expire after a configurable duration.
-- Creating a new token of the same type invalidates prior tokens for that user.

CREATE TABLE user_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(255) NOT NULL,
    token_type  VARCHAR(20) NOT NULL CHECK (token_type IN ('email_verify', 'password_reset')),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_tokens_hash ON user_tokens(token_hash) WHERE used_at IS NULL;
CREATE INDEX idx_user_tokens_user ON user_tokens(user_id, token_type);
