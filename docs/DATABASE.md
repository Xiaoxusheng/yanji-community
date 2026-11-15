# DATABASE — 数据库设计（PostgreSQL 18）

## 1. 设计原则

1. PostgreSQL 唯一事实源；Redis 仅缓存/限流/幂等。
2. 主表 `BIGSERIAL id` 内部主键 + 对外暴露 `public_id UUID`（API 一律使用 public_id 路由，防枚举）。
3. 时间列 `timestamptz`，统一 UTC（应用层 `now_utc()`）。
4. 软删除业务表带 `deleted_at timestamptz`；查询统一过滤。
5. 外键、唯一约束、CHECK 约束、索引全部在迁移文件中落实。
6. 互动表（点赞/收藏/关注）以 **复合唯一约束** 防重，不依赖应用层。
7. 计数列冗余但可校准（`POST /admin/ops/recalibrate` 规划中 / `internal/worker`）。
8. 迁移文件版本化（golang-migrate 编号），生产禁止自动 AutoMigrate。

## 2. 表清单（阶段 1 迁移 000001–000007）

### 迁移 000001 基础与扩展
- 启用扩展：`pg_trgm`（用户/标签模糊搜索）、`unaccent`（可选）。
- `now_utc()` 函数：`timezone('utc', now())`。

### 迁移 000002 用户与认证
| 表 | 关键字段 | 约束/索引 |
|---|---|---|
| users | id, public_id, username, email_norm, password_hash, status(active/pending/suspended/banned/deleted), email_verified_at, last_login_at, last_active_at, deleted_at | username 唯一(CITEXT 不用，统一 lower 存 email)、email_norm 唯一、public_id 唯一、status CHECK |
| user_profiles | user_id FK UNIQUE, display_name, avatar_key, bio, website, location, profile_visibility(public/followers/private), show_followers bool, notify_email bool, notify_inapp bool | CHECK website 前缀 http |
| user_sessions | id, user_id FK, token_hash UNIQUE, device_name, user_agent, ip_address, expires_at, last_seen_at, revoked_at | idx(user_id), idx(expires_at) |
| email_verification_tokens | user_id FK, token_hash UNIQUE, expires_at, used_at | |
| password_reset_tokens | user_id FK, token_hash UNIQUE, expires_at, used_at | |
| login_logs | user_id, success bool, ip_address, user_agent, created_at | idx(user_id, created_at) |

### 迁移 000003 RBAC
| 表 | 说明 |
|---|---|
| roles | code 唯一（member/moderator/admin/superadmin），is_system |
| permissions | code 唯一（resource.action 形式） |
| role_permissions | role_id+permission_id 唯一 |
| user_roles | user_id+role_id+scope_type(NULL/global/category)+scope_id 唯一；scope_id = categories.id，CHECK 与 scope_type 匹配 |

### 迁移 000004 版块与标签
| 表 | 约束要点 |
|---|---|
| categories | parent_id 自引用 FK，slug 唯一，visibility(public/private/hidden)，posting_policy(all/verified/staff_only)，review_policy(none/new_users/all)，status(active/archived)，deleted_at；CHECK parent_id != id |
| category_moderators | category_id+user_id 唯一 |
| category_follows | category_id+user_id 唯一 |
| tags | slug 唯一，status(active/hidden/disabled)，post_count 冗余 |
| tag_follows | tag_id+user_id 唯一 |

### 迁移 000005 帖子与评论
| 表 | 约束要点 |
|---|---|
| posts | public_id 唯一；author_id FK；category_id FK；post_type CHECK(discussion/article/question/poll)；status CHECK(draft/pending_review/published/rejected/hidden/locked/deleted)；title CHECK char_length BETWEEN 5 AND 200；body_rendered 净化后 HTML；is_pinned/is_featured；view_count/like_count/comment_count/bookmark_count DEFAULT 0 CHECK >= 0；published_at/last_activity_at/edited_at；deleted_at |
| 索引 | (status, published_at DESC) WHERE status='published'；(category_id, status, last_activity_at DESC)；(author_id, created_at DESC)；(is_featured, published_at DESC) WHERE status='published'；title+body 全文 GIN：`tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(title,'')||' '||coalesce(body_markdown,''))) STORED` |
| post_revisions | post_id FK, editor_id FK, title, body_markdown, edit_reason, created_at |
| post_tags | post_id+tag_id 唯一 |
| post_attachments | post_id FK, uploader_id, storage_key UNIQUE, original_name, mime_type, size_bytes CHECK>0, status(ready/deleted) |
| comments | public_id 唯一；post_id FK；author_id FK；parent_id 自引用；root_id；depth CHECK 0..3；body_markdown/body_rendered；status(published/pending_review/hidden/deleted)；like_count/reply_count；quote_comment_id FK（引用回复）；deleted_at；idx(post_id, created_at)、(root_id, created_at)、(parent_id) |

### 迁移 000006 互动
| 表 | 约束 |
|---|---|
| post_likes | user_id+post_id 唯一；FK post ON DELETE CASCADE |
| comment_likes | user_id+comment_id 唯一；CASCADE |
| bookmark_folders | user_id, name, sort_order；CHECK name 非空 |
| bookmarks | user_id+post_id 唯一；folder_id FK ON DELETE SET NULL（默认夹 NULL） |
| user_follows | follower_id+following_id 唯一；CHECK follower_id != following_id |
| user_blocks | blocker_id+blocked_id 唯一；CHECK blocker != blocked |

### 迁移 000007 通知/治理/运营/系统
| 表 | 要点 |
|---|---|
| notifications | recipient_id FK, actor_id FK, type, resource_type, resource_id(内部 BIGINT), title, body, read_at, dedupe_key UNIQUE NULLS DISTINCT；idx(recipient_id, created_at DESC)、(recipient_id) WHERE read_at IS NULL |
| reports | reporter_id FK, target_type(post/comment/user), target_id, reason_code CHECK 枚举, description, status(pending/processing/resolved/dismissed), assignee_id, resolution_note, resolved_at；防重：idx(reporter, target_type, target_id) 部分唯一 WHERE status IN ('pending','processing') |
| moderation_actions | target_type/target_id, target_user_id, action_type(hide/restore/lock/pin/unpin/feature/unfeature/reject/approve/move_category), reason, operator_id, created_at |
| user_sanctions | user_id FK, sanction_type(warn/mute/post_ban/temp_ban/perma_ban), starts_at, ends_at, reason, status(active/lifted/expired), operator_id, source_report_id；idx(user_id, status) |
| sensitive_words | word UNIQUE, match_type(exact/contains), action(block/review), is_enabled |
| announcements | title, body, status(draft/published/archived), published_at, starts_at, ends_at, author_id |
| featured_items | item_type(post), item_id, position, sort_order, operator_id, UNIQUE(item_type, item_id) |
| operation_logs | actor_id, action, target_type, target_id, request_id, detail JSONB, ip_address, created_at；idx(created_at DESC)、(actor_id) |
| system_configs | key UNIQUE, value TEXT, value_type, description, is_public bool, updated_by |
| outbox_events | event_type, aggregate_id, payload JSONB, status(pending/processing/done/failed), attempts, next_retry_at, last_error；idx(status, next_retry_at) |

### 第二阶段（仅文档规划，不建表）
point_accounts、point_transactions、levels、badges、user_badges、polls、poll_options、poll_votes、activities、activity_participants、appeals。
### 第三阶段（仅文档规划）
conversations、conversation_members、messages、message_attachments。

## 3. 状态枚举（CHECK 约束强制）

- users.status：`active | pending | suspended | banned | deleted`
- posts.status：`draft | pending_review | published | rejected | hidden | locked | deleted`
- comments.status：`published | pending_review | hidden | deleted`
- categories：visibility `public|private|hidden`；posting_policy `all|verified|staff_only`；review_policy `none|new_users|all`；status `active|archived`
- reports.status：`pending | processing | resolved | dismissed`
- user_sanctions：`warn | mute | post_ban | temp_ban | perma_ban`

## 4. 迁移策略

- 目录：`backend/migrations/`，命名 `NNNNNN_desc.up.sql` / `NNNNNN_desc.down.sql`。
- 开发/生产启动自动执行（可 `MIGRATE_ON_START=false` 关闭）；CI 中以 `migrate` 检查。
- 种子数据（`platform/seed`）：角色/权限点、默认版块（综合讨论/技术分享/生活/问答）、示例敏感词关闭态、超级管理员账号（凭据来自环境变量，严禁硬编码）。
