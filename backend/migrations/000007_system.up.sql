-- 000007：通知、治理、运营与系统表。

CREATE TABLE notifications (
    id            BIGSERIAL PRIMARY KEY,
    recipient_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    type          TEXT NOT NULL CHECK (type IN
                  ('comment','reply','mention','like_post','like_comment','follow','moderation','system')),
    resource_type TEXT NOT NULL DEFAULT '' CHECK (resource_type IN ('','post','comment','user')),
    resource_id   BIGINT NOT NULL DEFAULT 0,
    title         TEXT NOT NULL,
    body          TEXT NOT NULL DEFAULT '',
    read_at       TIMESTAMPTZ,
    dedupe_key    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT notifications_dedupe_unique UNIQUE NULLS NOT DISTINCT (dedupe_key)
);
CREATE INDEX idx_notifications_recipient ON notifications (recipient_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications (recipient_id) WHERE read_at IS NULL;

CREATE TABLE reports (
    id              BIGSERIAL PRIMARY KEY,
    reporter_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type     TEXT NOT NULL CHECK (target_type IN ('post','comment','user')),
    target_id       BIGINT NOT NULL,
    reason_code     TEXT NOT NULL CHECK (reason_code IN
                    ('spam','harassment','porn_violence','illegal','infringement','misinformation','flame','other')),
    description     TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    status          TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','processing','resolved','dismissed')),
    assignee_id     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    resolution_note TEXT NOT NULL DEFAULT '',
    resolved_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
-- 同一用户对同一对象在处理中不可重复提交
CREATE UNIQUE INDEX idx_reports_open_unique ON reports (reporter_id, target_type, target_id)
    WHERE status IN ('pending','processing');
CREATE INDEX idx_reports_status ON reports (status, created_at DESC);

CREATE TABLE moderation_actions (
    id             BIGSERIAL PRIMARY KEY,
    target_type    TEXT NOT NULL CHECK (target_type IN ('post','comment','user','category','tag')),
    target_id      BIGINT NOT NULL,
    target_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action_type    TEXT NOT NULL CHECK (action_type IN
                   ('approve','reject','hide','restore','lock','unlock','pin','unpin','feature','unfeature',
                    'move_category','resolve_report','dismiss_report','warn','mute','ban','unban')),
    reason         TEXT NOT NULL DEFAULT '',
    operator_id    BIGINT NOT NULL REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_moderation_actions_target ON moderation_actions (target_type, target_id, created_at DESC);

CREATE TABLE user_sanctions (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sanction_type    TEXT NOT NULL CHECK (sanction_type IN ('warn','mute','post_ban','temp_ban','perma_ban')),
    reason           TEXT NOT NULL DEFAULT '',
    starts_at        TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    ends_at          TIMESTAMPTZ,                       -- NULL = 永久
    status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','lifted','expired')),
    source_report_id BIGINT REFERENCES reports(id) ON DELETE SET NULL,
    operator_id      BIGINT NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    lifted_at        TIMESTAMPTZ
);
CREATE INDEX idx_user_sanctions_user ON user_sanctions (user_id, status);

CREATE TABLE sensitive_words (
    id         BIGSERIAL PRIMARY KEY,
    word       TEXT NOT NULL UNIQUE CHECK (char_length(word) BETWEEN 1 AND 100),
    match_type TEXT NOT NULL DEFAULT 'contains' CHECK (match_type IN ('exact','contains')),
    action     TEXT NOT NULL DEFAULT 'block' CHECK (action IN ('block','review')),
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE announcements (
    id           BIGSERIAL PRIMARY KEY,
    title        TEXT NOT NULL CHECK (char_length(title) BETWEEN 2 AND 200),
    body         TEXT NOT NULL DEFAULT '' CHECK (char_length(body) <= 10000),
    status       TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','archived')),
    starts_at    TIMESTAMPTZ,
    ends_at      TIMESTAMPTZ,
    author_id    BIGINT NOT NULL REFERENCES users(id),
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT announcements_time_check CHECK (ends_at IS NULL OR starts_at IS NULL OR ends_at > starts_at)
);
CREATE INDEX idx_announcements_active ON announcements (published_at DESC)
    WHERE status = 'published';

CREATE TABLE featured_items (
    id          BIGSERIAL PRIMARY KEY,
    item_type   TEXT NOT NULL DEFAULT 'post' CHECK (item_type IN ('post')),
    item_id     BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    position    TEXT NOT NULL DEFAULT 'home' CHECK (position IN ('home','sidebar')),
    sort_order  INT NOT NULL DEFAULT 0,
    operator_id BIGINT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT featured_items_unique UNIQUE (item_type, item_id, position)
);

CREATE TABLE operation_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action      TEXT NOT NULL,
    target_type TEXT NOT NULL DEFAULT '',
    target_id   BIGINT NOT NULL DEFAULT 0,
    request_id  TEXT NOT NULL DEFAULT '',
    detail      JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address  TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_operation_logs_created ON operation_logs (created_at DESC);
CREATE INDEX idx_operation_logs_actor ON operation_logs (actor_id, created_at DESC);

CREATE TABLE system_configs (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    value_type  TEXT NOT NULL DEFAULT 'string' CHECK (value_type IN ('string','int','bool','json')),
    description TEXT NOT NULL DEFAULT '',
    is_public   BOOLEAN NOT NULL DEFAULT FALSE,     -- true 时可通过公开接口读取
    updated_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE outbox_events (
    id            BIGSERIAL PRIMARY KEY,
    event_type    TEXT NOT NULL CHECK (event_type IN
                  ('notification.created','email.verification','email.password_reset','email.welcome')),
    aggregate_id  BIGINT NOT NULL DEFAULT 0,
    payload       JSONB NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','done','failed')),
    attempts      INT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    last_error    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    processed_at  TIMESTAMPTZ
);
CREATE INDEX idx_outbox_pending ON outbox_events (next_retry_at) WHERE status IN ('pending','processing');
