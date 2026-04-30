CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE auth.users (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username                 VARCHAR(50)  NOT NULL UNIQUE,
    email                    VARCHAR(255) NOT NULL UNIQUE,
    password_hash            VARCHAR(255) NOT NULL,
    role                     VARCHAR(10)  NOT NULL DEFAULT 'user'
                             CHECK (role IN ('user', 'admin')),
    currency                 VARCHAR(3)   NOT NULL DEFAULT 'USD',
    has_completed_onboarding BOOLEAN      NOT NULL DEFAULT false,
    tokens_revoked_at        TIMESTAMPTZ,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON auth.users (email);
CREATE INDEX idx_users_username ON auth.users (username);
