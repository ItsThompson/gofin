CREATE TABLE auth.refresh_token_blacklist (
    jti        VARCHAR(36)  PRIMARY KEY,
    user_id    UUID         NOT NULL REFERENCES auth.users(id),
    expires_at TIMESTAMPTZ  NOT NULL,
    revoked_at TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_blacklist_expires ON auth.refresh_token_blacklist (expires_at);
