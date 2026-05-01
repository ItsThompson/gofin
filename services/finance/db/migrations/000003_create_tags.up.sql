CREATE TABLE finance.tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID         NOT NULL,
    name        VARCHAR(50)  NOT NULL,
    is_default  BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Tag names are unique per user (case-insensitive).
-- Uses a unique index instead of a table constraint because sqlc's parser
-- does not support expression-based UNIQUE constraints.
CREATE UNIQUE INDEX idx_tags_user_name ON finance.tags (user_id, lower(name));
CREATE INDEX idx_tags_user ON finance.tags (user_id);
