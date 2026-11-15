-- 000009：阶段 3——一对一私信：会话、成员、消息与附件。
-- 红线：消息先可靠落库后推送；1:1 会话用 pair_key 唯一约束防重复建会话；
-- 成员关系唯一；撤回窗口由服务端校验；举报/通知类型扩展支持私信。

CREATE TABLE conversations (
    id               BIGSERIAL PRIMARY KEY,
    pair_key         TEXT UNIQUE,                     -- 1:1 会话去重键："dm:{小ID}:{大ID}"
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    last_message_at  TIMESTAMPTZ
);
CREATE INDEX idx_conversations_last_msg ON conversations (last_message_at DESC);

CREATE TABLE conversation_members (
    id                  BIGSERIAL PRIMARY KEY,
    conversation_id     BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    muted               BOOLEAN NOT NULL DEFAULT FALSE,  -- 免打扰
    pinned              BOOLEAN NOT NULL DEFAULT FALSE,  -- 会话置顶
    joined_at           TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT conversation_members_unique UNIQUE (conversation_id, user_id)
);
CREATE INDEX idx_conv_members_user ON conversation_members (user_id);

CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    body_text       TEXT NOT NULL DEFAULT '' CHECK (char_length(body_text) <= 5000),
    reply_to_id     BIGINT REFERENCES messages(id) ON DELETE SET NULL,  -- 引用消息
    status          TEXT NOT NULL DEFAULT 'sent' CHECK (status IN ('sent','recalled')),
    recalled_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT messages_not_empty CHECK (char_length(body_text) > 0)
);
CREATE INDEX idx_messages_conv ON messages (conversation_id, created_at DESC);
CREATE INDEX idx_messages_sender ON messages (sender_id, created_at DESC);

CREATE TABLE message_attachments (
    id          BIGSERIAL PRIMARY KEY,
    message_id  BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    uploader_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL UNIQUE,
    mime_type   TEXT NOT NULL DEFAULT '' CHECK (mime_type IN ('image/jpeg','image/png','image/gif','image/webp')),
    size_bytes  BIGINT NOT NULL CHECK (size_bytes > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_message_attachments_msg ON message_attachments (message_id);

-- 私信开关（PRD 3.2：是否允许陌生人私信）
ALTER TABLE user_profiles ADD COLUMN allow_direct_messages BOOLEAN NOT NULL DEFAULT TRUE;

-- 举报支持私信目标
ALTER TABLE reports DROP CONSTRAINT reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check
    CHECK (target_type IN ('post','comment','user','message'));

-- 通知类型支持私信
ALTER TABLE notifications DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('comment','reply','mention','like_post','like_comment','follow','moderation','system','message'));
