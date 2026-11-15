-- 000003：版块与标签。

CREATE TABLE categories (
    id             BIGSERIAL PRIMARY KEY,
    parent_id      BIGINT REFERENCES categories(id) ON DELETE SET NULL,
    name           TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    slug           TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$'),
    description    TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 1000),
    icon           TEXT NOT NULL DEFAULT '',
    sort_order     INT NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    visibility     TEXT NOT NULL DEFAULT 'public'
                   CHECK (visibility IN ('public','private','hidden')),
    posting_policy TEXT NOT NULL DEFAULT 'all'
                   CHECK (posting_policy IN ('all','verified','staff_only')),
    review_policy  TEXT NOT NULL DEFAULT 'none'
                   CHECK (review_policy IN ('none','new_users','all')),
    post_count     BIGINT NOT NULL DEFAULT 0 CHECK (post_count >= 0),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    deleted_at     TIMESTAMPTZ,
    CONSTRAINT categories_no_self_parent CHECK (parent_id IS NULL OR parent_id <> id)
);
CREATE INDEX idx_categories_parent ON categories (parent_id, sort_order);

CREATE TABLE category_moderators (
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (category_id, user_id)
);

CREATE TABLE category_follows (
    category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (category_id, user_id)
);

CREATE TABLE tags (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 30),
    slug       TEXT NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,48}$'),
    description TEXT NOT NULL DEFAULT '' CHECK (char_length(description) <= 500),
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','hidden','disabled')),
    post_count BIGINT NOT NULL DEFAULT 0 CHECK (post_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE tag_follows (
    tag_id     BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    PRIMARY KEY (tag_id, user_id)
);
