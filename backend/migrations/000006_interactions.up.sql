-- 000006：互动——点赞/收藏/关注/拉黑（全部以数据库唯一约束防重）。

CREATE TABLE post_likes (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id    BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (user_id, post_id)
);
CREATE INDEX idx_post_likes_post ON post_likes (post_id);

CREATE TABLE comment_likes (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    comment_id BIGINT NOT NULL REFERENCES comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (user_id, comment_id)
);
CREATE INDEX idx_comment_likes_comment ON comment_likes (comment_id);

CREATE TABLE bookmark_folders (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_bookmark_folders_user ON bookmark_folders (user_id, sort_order);

CREATE TABLE bookmarks (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id    BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    folder_id  BIGINT REFERENCES bookmark_folders(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT bookmarks_user_post_unique UNIQUE (user_id, post_id)
);
CREATE INDEX idx_bookmarks_user ON bookmarks (user_id, created_at DESC);
CREATE INDEX idx_bookmarks_folder ON bookmarks (folder_id);

CREATE TABLE user_follows (
    follower_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT user_follows_no_self CHECK (follower_id <> following_id)
);
CREATE INDEX idx_user_follows_following ON user_follows (following_id);

CREATE TABLE user_blocks (
    blocker_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blocked_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (blocker_id, blocked_id),
    CONSTRAINT user_blocks_no_self CHECK (blocker_id <> blocked_id)
);
