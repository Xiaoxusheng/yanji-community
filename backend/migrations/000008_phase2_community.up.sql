-- 000008：阶段 2——积分/等级/勋章/签到、投票、活动、申诉、问答最佳答案。
-- 幂等红线：积分流水幂等键唯一；签到 (user, date) 唯一；投票 (user, option) 唯一；
-- 活动 (activity, user) 唯一；开放申诉 (user, target) 唯一；最佳答案每帖唯一。

-- ---------- 积分 ----------
CREATE TABLE point_accounts (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    balance      BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    total_earned BIGINT NOT NULL DEFAULT 0 CHECK (total_earned >= 0),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE point_transactions (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount         BIGINT NOT NULL CHECK (amount <> 0),
    balance_after  BIGINT NOT NULL CHECK (balance_after >= 0),
    source_type    TEXT NOT NULL CHECK (source_type IN
                   ('registration','sign_in','post_created','post_liked','comment_liked',
                    'answer_accepted','activity_attended','badge','admin_adjust')),
    source_id      BIGINT NOT NULL DEFAULT 0,
    idempotency_key TEXT NOT NULL UNIQUE,          -- 幂等：同一业务事件只记一次账
    note           TEXT NOT NULL DEFAULT '',
    operator_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_point_tx_user ON point_transactions (user_id, created_at DESC);

CREATE TABLE sign_ins (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    check_date DATE NOT NULL,
    points     INT NOT NULL DEFAULT 5 CHECK (points > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (user_id, check_date)              -- 每日一次签到，数据库层防重
);

-- ---------- 等级与勋章 ----------
CREATE TABLE levels (
    level      INT PRIMARY KEY,
    name       TEXT NOT NULL,
    min_points BIGINT NOT NULL CHECK (min_points >= 0),
    color      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE badges (
    id             BIGSERIAL PRIMARY KEY,
    code           TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    icon           TEXT NOT NULL DEFAULT '',
    condition_type TEXT NOT NULL DEFAULT 'manual' CHECK (condition_type IN ('manual','auto')),
    is_enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE user_badges (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    badge_id   BIGINT NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
    awarded_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT user_badges_unique UNIQUE (user_id, badge_id)   -- 每枚勋章只授一次
);
CREATE INDEX idx_user_badges_user ON user_badges (user_id, created_at DESC);

-- ---------- 投票 ----------
CREATE TABLE polls (
    id              BIGSERIAL PRIMARY KEY,
    post_id         BIGINT NOT NULL UNIQUE REFERENCES posts(id) ON DELETE CASCADE,
    multiple_choice BOOLEAN NOT NULL DEFAULT FALSE,
    max_choices     INT NOT NULL DEFAULT 1 CHECK (max_choices >= 1),
    allow_revote    BOOLEAN NOT NULL DEFAULT FALSE,
    ends_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE poll_options (
    id         BIGSERIAL PRIMARY KEY,
    poll_id    BIGINT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    content    TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 200),
    sort_order INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_poll_options_poll ON poll_options (poll_id, sort_order);

CREATE TABLE poll_votes (
    id         BIGSERIAL PRIMARY KEY,
    poll_id    BIGINT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id  BIGINT NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT poll_votes_unique UNIQUE (user_id, option_id)   -- 投票防重：数据库唯一约束
);
CREATE INDEX idx_poll_votes_poll ON poll_votes (poll_id);

-- ---------- 活动 ----------
CREATE TABLE activities (
    id               BIGSERIAL PRIMARY KEY,
    title            TEXT NOT NULL CHECK (char_length(title) BETWEEN 2 AND 120),
    description      TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 10000),
    cover_key        TEXT NOT NULL DEFAULT '',
    location         TEXT NOT NULL DEFAULT '',
    starts_at        TIMESTAMPTZ NOT NULL,
    ends_at          TIMESTAMPTZ,
    signup_starts_at TIMESTAMPTZ,
    signup_ends_at   TIMESTAMPTZ,
    capacity         INT NOT NULL DEFAULT 0 CHECK (capacity >= 0),   -- 0 = 不限名额
    status           TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','cancelled','finished')),
    reward_points    INT NOT NULL DEFAULT 0 CHECK (reward_points >= 0),
    created_by       BIGINT NOT NULL REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT activities_time_check CHECK (ends_at IS NULL OR ends_at > starts_at)
);
CREATE INDEX idx_activities_status ON activities (status, starts_at DESC);

CREATE TABLE activity_participants (
    id          BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status      TEXT NOT NULL DEFAULT 'signed_up' CHECK (status IN ('signed_up','cancelled','attended')),
    signed_at   TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT activity_participants_unique UNIQUE (activity_id, user_id)  -- 并发报名由唯一约束 + 行锁保证
);
CREATE INDEX idx_activity_participants_user ON activity_participants (user_id);

-- ---------- 申诉 ----------
CREATE TABLE appeals (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL CHECK (target_type IN ('sanction','moderation_action')),
    target_id   BIGINT NOT NULL,
    reason      TEXT NOT NULL CHECK (char_length(reason) BETWEEN 5 AND 2000),
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','rejected')),
    handled_by  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    handle_note TEXT NOT NULL DEFAULT '',
    handled_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
-- 同一对象同一用户同时只能有一条待处理申诉
CREATE UNIQUE INDEX idx_appeals_open_unique ON appeals (user_id, target_type, target_id) WHERE status = 'pending';
CREATE INDEX idx_appeals_status ON appeals (status, created_at DESC);

-- ---------- 问答：最佳回答（每帖唯一） ----------
ALTER TABLE comments ADD COLUMN is_accepted_answer BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX idx_comments_accepted_unique ON comments (post_id) WHERE is_accepted_answer = TRUE;
