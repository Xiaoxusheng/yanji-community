# PERMISSIONS — 角色权限设计（RBAC）

## 1. 模型

```
users ─< user_roles >─ roles ─< role_permissions >─ permissions
                │
                └─ scope: (scope_type, scope_id)
                   scope_type ∈ {global, category}
                   scope_id   ∈ {NULL, categories.id}
```

- **全局角色**：scope_type='global'（如 admin）。
- **版块范围角色**：scope_type='category' 且 scope_id=版块 ID（版主）。版主权限仅在该版块生效，服务端逐次校验。
- 权限点格式 `resource.action`；前端仅做展示性隐藏。

## 2. 内置角色（is_system=true）

| code | 名称 | 授予方式 |
|---|---|---|
| member | 注册用户 | 注册时自动授予（global） |
| moderator | 版主 | 管理员在后台按版块授予 |
| admin | 管理员 | 超管授予 |
| superadmin | 超级管理员 | 种子账号唯一持有 |

## 3. 权限点清单（阶段 1）

| 权限点 | 说明 | member | moderator | admin | superadmin |
|---|---|---|---|---|---|
| post.create | 发帖 | ✅ | ✅ | ✅ | ✅ |
| post.comment | 评论 | ✅ | ✅ | ✅ | ✅ |
| post.interact | 点赞/收藏/关注 | ✅ | ✅ | ✅ | ✅ |
| post.moderate | 审核帖子（版主限授权版块） | ❌ | ✅(scope) | ✅ | ✅ |
| comment.moderate | 审核评论（同上范围限制） | ❌ | ✅(scope) | ✅ | ✅ |
| post.pin | 置顶（范围限制） | ❌ | ✅(scope) | ✅ | ✅ |
| post.feature | 加精（范围限制） | ❌ | ✅(scope) | ✅ | ✅ |
| report.create | 举报 | ✅ | ✅ | ✅ | ✅ |
| report.handle | 处理举报 | ❌ | ✅(scope) | ✅ | ✅ |
| dashboard.read | 后台仪表盘 | ❌ | ❌ | ✅ | ✅ |
| user.read | 用户管理查看 | ❌ | ❌ | ✅ | ✅ |
| user.sanction | 处罚用户 | ❌ | ❌ | ✅ | ✅ |
| category.manage | 版块管理 | ❌ | ❌ | ✅ | ✅ |
| tag.manage | 标签管理 | ❌ | ❌ | ✅ | ✅ |
| announcement.manage | 公告管理 | ❌ | ❌ | ✅ | ✅ |
| moderation.manage | 敏感词/审核配置 | ❌ | ❌ | ✅ | ✅ |
| role.read | 角色查看 | ❌ | ❌ | ✅ | ✅ |
| role.assign | 授予/移除角色 | ❌ | ❌ | ❌ | ✅ |
| settings.manage | 系统配置 | ❌ | ❌ | ✅ | ✅ |
| audit.read | 审计日志 | ❌ | ❌ | ✅ | ✅ |

## 4. 服务端校验流程

1. 认证中间件解析会话 → 注入 `UserID`、`EmailVerified`、`Status`。
2. `RequireAuth` → 未登录 401；账号 `suspended/banned` → 403 `ACCOUNT_SUSPENDED`；活跃处罚（mute/post_ban/temp_ban/perma_ban）→ 403 `SANCTIONED`。
3. `RequirePermission(code)` → 查 user_roles+role_permissions（带 5 分钟内存缓存）。
4. **范围校验**：`moderator` 的 `post.moderate` 等操作在 service 层再校验目标帖子的 category_id ∈ 授权版块集合；admin/superadmin 无范围限制。
5. **对象级校验**：编辑/删除帖子评论必须 `author_id == current user`（或具备对应 moderate 权限）。
6. 版主处理举报仅限目标内容所在授权版块。

## 5. 处罚效果（user_sanctions，查询时判定 + 惰性过期）

| 类型 | 效果 |
|---|---|
| warn | 仅通知 |
| mute | 禁止评论/私信 |
| post_ban | 禁止发帖 |
| temp_ban | 登录受限（ends_at 前不可互动与登录） |
| perma_ban | 永久禁言 |

处罚创建/解除写 operation_logs；被处罚用户收到 moderation 通知。
