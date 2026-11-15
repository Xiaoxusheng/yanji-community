-- 000005：帖子、修订、标签关联、附件与评论。

CREATE TABLE posts (
    id             BIGSERIAL PRIMARY KEY,
    public_id      UUID NOT NULL DEFAULT gen_random_uuid(),
    author_id      BIGINT NOT NULL REFERENCES users(id),
    category_id    BIGINT NOT NULL REFERENCES categories(id),
    post_type      TEXT NOT NULL DEFAULT 'discussion'
                   CHECK (post_type IN ('discussion','article','question','poll')),
    title          TEXT NOT NULL CHECK (char_length(title) BETWEEN 5 AND 200),
    body_markdown  TEXT NOT NULL CHECK (char_length(body_markdown) <= 100000),
    body_rendered  TEXT NOT NULL DEFAULT '',          -- 净化后的 HTML 缓存
    excerpt        TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft','pending_review','published','rejected','hidden','locked','deleted')),
    reject_reason  TEXT NOT NULL DEFAULT '',
    is_pinned      BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured    BOOLEAN NOT NULL DEFAULT FALSE,
    view_count     BIGINT NOT NULL DEFAULT 0 CHECK (view_count >= 0),
    like_count     BIGINT NOT NULL DEFAULT 0 CHECK (like_count >= 0),
    comment_count  BIGINT NOT NULL DEFAULT 0 CHECK (comment_count >= 0),
    bookmark_count BIGINT NOT NULL DEFAULT 0 CHECK (bookmark_count >= 0),
    published_at   TIMESTAMPTZ,
    edited_at      TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    deleted_at     TIMESTAMPTZ,
    CONSTRAINT posts_public_id_unique UNIQUE (public_id)
);

-- 公开流主索引
CREATE INDEX idx_posts_feed ON posts (status, published_at DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX idx_posts_category ON posts (category_id, status, last_activity_at DESC);
CREATE INDEX idx_posts_author ON posts (author_id, created_at DESC);
CREATE INDEX idx_posts_featured ON posts (is_featured, published_at DESC) WHERE status = 'published' AND deleted_at IS NULL;

-- 全文检索（标题+正文，simple 配置避免分词歧义；中文按字切分 + ILIKE 兜底见搜索模块）
ALTER TABLE posts ADD COLUMN tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(body_markdown, ''))) STORED;
CREATE INDEX idx_posts_tsv ON posts USING GIN (tsv) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX idx_posts_title_trgm ON posts USING GIN (title gin_trgm_ops);

CREATE TABLE post_revisions (
    id            BIGSERIAL PRIMARY KEY,
    post_id       BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    editor_id     BIGINT NOT NULL REFERENCES users(id),
    title         TEXT NOT NULL,
    body_markdown TEXT NOT NULL,
    edit_reason   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_post_revisions_post ON post_revisions (post_id, created_at DESC);

CREATE TABLE post_tags (
    post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id  BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (post_id, tag_id)
);
CREATE INDEX idx_post_tags_tag ON post_tags (tag_id);

CREATE TABLE post_attachments (
    id            BIGSERIAL PRIMARY KEY,
    post_id       BIGINT REFERENCES posts(id) ON DELETE CASCADE,
    uploader_id   BIGINT NOT NULL REFERENCES users(id),
    storage_key   TEXT NOT NULL UNIQUE,
    original_name TEXT NOT NULL DEFAULT '',
    mime_type     TEXT NOT NULL DEFAULT '',
    size_bytes    BIGINT NOT NULL CHECK (size_bytes > 0),
    status        TEXT NOT NULL DEFAULT 'ready' CHECK (status IN ('ready','deleted')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);
CREATE INDEX idx_post_attachments_post ON post_attachments (post_id);

CREATE TABLE comments (
    id               BIGSERIAL PRIMARY KEY,
    public_id        UUID NOT NULL DEFAULT gen_random_uuid(),
    post_id          BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id        BIGINT NOT NULL REFERENCES users(id),
    parent_id        BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    root_id          BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    depth            INT NOT NULL DEFAULT 0 CHECK (depth BETWEEN 0 AND 2), -- 楼层：0/1/2 共 3 层
    body_markdown    TEXT NOT NULL CHECK (char_length(body_markdown) BETWEEN 1 AND 10000),
    body_rendered    TEXT NOT NULL DEFAULT '',
    quote_comment_id BIGINT REFERENCES comments(id) ON DELETE SET NULL,   -- 引用回复
    status           TEXT NOT NULL DEFAULT 'published'
                     CHECK (status IN ('published','pending_review','hidden','deleted')),
    like_count       BIGINT NOT NULL DEFAULT 0 CHECK (like_count >= 0),
    reply_count      BIGINT NOT NULL DEFAULT 0 CHECK (reply_count >= 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    edited_at        TIMESTAMPTZ,
    deleted_at       TIMESTAMPTZ,
    CONSTRAINT comments_public_id_unique UNIQUE (public_id)
);
CREATE INDEX idx_comments_post ON comments (post_id, status, created_at);
CREATE INDEX idx_comments_parent ON comments (parent_id, created_at);
CREATE INDEX idx_comments_root ON comments (root_id, created_at) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_comments_author ON comments (author_id, created_at DESC);
