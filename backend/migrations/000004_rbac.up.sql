-- 000003：RBAC（角色/权限点/用户角色含版块范围）。

CREATE TABLE roles (
    id          BIGSERIAL PRIMARY KEY,
    code        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_system   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE permissions (
    id         BIGSERIAL PRIMARY KEY,
    code       TEXT NOT NULL UNIQUE,           -- 形如 post.moderate
    name       TEXT NOT NULL,
    resource   TEXT NOT NULL,
    action     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc()
);

CREATE TABLE role_permissions (
    role_id       BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id    BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL DEFAULT 'global' CHECK (scope_type IN ('global','category')),
    scope_id   BIGINT REFERENCES categories(id) ON DELETE CASCADE, -- category 范围时必填
    created_at TIMESTAMPTZ NOT NULL DEFAULT now_utc(),
    CONSTRAINT user_roles_unique UNIQUE (user_id, role_id, scope_type, scope_id),
    CONSTRAINT user_roles_scope_check CHECK (
        (scope_type = 'global' AND scope_id IS NULL) OR
        (scope_type = 'category' AND scope_id IS NOT NULL)
    )
);
CREATE INDEX idx_user_roles_user ON user_roles (user_id);
CREATE INDEX idx_user_roles_scope ON user_roles (scope_type, scope_id);
