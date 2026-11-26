-- 000002：用户、资料、会话与认证令牌。

CREATE TABLE users (
    id                BIGSERIAL PRIMARY KEY,
    public_id         UUID NOT NULL DEFAULT gen_random_uuid(),
    username          TEXT NOT NULL,
    email_norm        TEXT NOT NULL,                 -- 统一小写存储
    password_hash     TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('active','pending','suspended','banned','deleted')),
    email_verified_at TIMESTAMPTZ,
    last_login_at     TIMESTAMPTZ,
    last_active_at    TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT users_username_unique UNIQUE (username),
    CONSTRAINT users_email_unique UNIQUE (email_norm),
    CONSTRAINT users_public_id_unique UNIQUE (public_id)
);
CREATE INDEX idx_users_status_created ON users (status, created_at DESC);

CREATE TABLE user_profiles (
    user_id            BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name       TEXT NOT NULL DEFAULT '',
    avatar_key         TEXT NOT NULL DEFAULT '',
    cover_key          TEXT NOT NULL DEFAULT '',
    bio                TEXT NOT NULL DEFAULT '' CHECK (char_length(bio) <= 500),
    website            TEXT NOT NULL DEFAULT '',
    location           TEXT NOT NULL DEFAULT '',
    profile_visibility TEXT NOT NULL DEFAULT 'public'
                       CHECK (profile_visibility IN ('public','followers','private')),
    show_followers     BOOLEAN NOT NULL DEFAULT TRUE,
    notify_email       BOOLEAN NOT NULL DEFAULT TRUE,
    notify_inapp       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE user_sessions (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL,                       -- SHA-256 摘要，不存原始 token
    device_name  TEXT NOT NULL DEFAULT '',
    user_agent   TEXT NOT NULL DEFAULT '',
    ip_address   TEXT NOT NULL DEFAULT '',
    expires_at   TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    revoked_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT user_sessions_token_unique UNIQUE (token_hash)
);
CREATE INDEX idx_user_sessions_user ON user_sessions (user_id, created_at DESC);
CREATE INDEX idx_user_sessions_expires ON user_sessions (expires_at) WHERE revoked_at IS NULL;

CREATE TABLE email_verification_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_email_verify_user ON email_verification_tokens (user_id);

CREATE TABLE password_reset_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_password_reset_user ON password_reset_tokens (user_id);

CREATE TABLE login_logs (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    success    BOOLEAN NOT NULL,
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_login_logs_user ON login_logs (user_id, created_at DESC);
