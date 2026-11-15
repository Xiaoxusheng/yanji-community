# API — 接口规范（/api/v1）

## 1. 通用约定

- Base：`/api/v1`；REST + JSON；时间 ISO 8601 UTC。
- 认证：会话 Cookie `yanji_session`（HttpOnly/SameSite=Lax，生产 Secure）。`/auth/me` 等接口对游客返回 200 + `authenticated:false`（除标注需登录外）。
- 统一响应：`{"code":"OK","message":"success","data":{...},"request_id":"req_xxx"}`；错误时 `data:null`。
- 分页：`page`(默认1) + `page_size`(默认20, 最大100)；列表 data：`{items, page, page_size, total, has_more}`。
- 错误码：`OK / VALIDATION_ERROR(400) / UNAUTHENTICATED(401) / FORBIDDEN(403) / NOT_FOUND(404) / CONFLICT(409) / EMAIL_NOT_VERIFIED(403) / POST_LOCKED(409) / CONTENT_PENDING_REVIEW(202) / RATE_LIMITED(429) / ACCOUNT_SUSPENDED(403) / SANCTIONED(403) / INTERNAL_ERROR(500)`。
- 幂等：`POST /posts`、`PATCH /posts/{id}` 支持 `Idempotency-Key` 头（Redis 8h）。

## 2. 认证 /api/v1/auth

| Method | Path | 说明 | 备注 |
|---|---|---|---|
| POST | /auth/register | {username,email,password} → 需邮箱验证 | 防枚举统一提示；限流 |
| POST | /auth/login | {account,password} → set-cookie | 失败计数限流；写 login_logs |
| POST | /auth/logout | 撤销当前会话 | |
| POST | /auth/logout-all | 撤销全部会话 | |
| GET | /auth/me | 当前用户（游客→authenticated:false） | |
| POST | /auth/email/verify | {token} 验证邮箱 | |
| POST | /auth/email/resend | 重发验证邮件 | 已验证返回 OK 不泄露 |
| POST | /auth/password/forgot | {email} 发起重置 | 防枚举；限流 |
| POST | /auth/password/reset | {token,new_password} | 成功撤销全部会话 |
| GET | /auth/sessions | 会话设备列表 | 登录 |
| DELETE | /auth/sessions/{id} | 撤销指定会话 | 登录，仅本人 |

## 3. 用户 /api/v1

| Method | Path | 说明 |
|---|---|---|
| GET | /users/{username} | 公开资料 + 计数 + is_following（隐私设置控制） |
| GET | /users/{username}/posts | 公开帖子（published） |
| GET | /users/{username}/followers · /following | 遵循 show_followers |
| PATCH | /me/profile | display_name/bio/avatar_key/website/location |
| PATCH | /me/settings | profile_visibility/show_followers/notify_* |
| GET | /me/posts | 自己全部帖子 ?status= |
| GET | /me/overview | 个人中心聚合（帖子/评论/收藏/关注计数） |
| GET | /me/following?type=user\|category\|tag | 我的关注 |
| GET | /me/followers | 我的粉丝 |
| POST | /users/{publicId}/follow · DELETE | 关注/取关（幂等） |
| POST | /users/{publicId}/block · DELETE | 拉黑/取消 |
| GET | /me/bookmarks · /me/bookmark-folders | 收藏与收藏夹 |
| POST/PATCH/DELETE | /me/bookmark-folders[/{id}] | 收藏夹 CRUD |
| POST | /me/bookmarks/{postId} | 收藏 {folder_id?} |
| DELETE | /me/bookmarks/{postId} | 取消收藏 |

## 4. 版块 / 标签

| Method | Path | 说明 |
|---|---|---|
| GET | /categories | 版块树（含帖子数） |
| GET | /categories/{slug} | 版块详情 + is_following + is_moderator |
| GET | /categories/{slug}/posts | 版块帖子列表 ?sort=latest\|hot |
| POST | /categories/{slug}/follow · DELETE | 关注版块 |
| GET | /tags · /tags/{slug} | 标签列表/详情 |
| GET | /tags/{slug}/posts | 标签帖子 |
| POST | /tags/{slug}/follow · DELETE | 关注标签 |

## 5. 帖子 / 评论 / 互动

| Method | Path | 说明 |
|---|---|---|
| GET | /feeds?tab=latest\|hot\|featured\|following\|recommended | 首页信息流（following 需登录） |
| GET | /posts/{publicId} | 详情（浏览量去重计数；作者本人可见草稿） |
| POST | /posts | 创建 {title,body_markdown,category_id,post_type,tag_ids,is_draft}；审核策略生效 |
| PATCH | /posts/{publicId} | 编辑（作者；保留修订） |
| DELETE | /posts/{publicId} | 作者软删除 |
| POST | /posts/{publicId}/publish | 草稿发布 |
| GET | /posts/{publicId}/revisions | 修订历史（作者/版主/管理员） |
| POST | /posts/{publicId}/view | 浏览计数（去重） |
| GET | /posts/{publicId}/comments | 评论树（顶层分页 + 楼中楼展开，quote 附带） |
| POST | /posts/{publicId}/comments | {body_markdown,parent_id?,quote_comment_id?} 深度3限制 |
| PATCH | /comments/{publicId} | 作者编辑 |
| DELETE | /comments/{publicId} | 作者/版主/管理员 |
| POST/DELETE | /posts/{publicId}/like · /comments/{publicId}/like | 点赞幂等 |
| POST/DELETE | /posts/{publicId}/bookmark | 收藏/取消 |
| POST | /reports | 举报 {target_type,target_public_id,reason_code,description} |
| POST | /uploads | multipart 上传图片（校验后返回 key/url） |

## 6. 搜索 / 通知 / 公告

| Method | Path | 说明 |
|---|---|---|
| GET | /search?q=&type=post\|user\|category\|tag&sort=relevance\|latest\|hot&category_id=&tag_slug=&author=&from=&to= | 统一搜索，结果含高亮片段 |
| GET | /me/notifications?unread=&type= | 通知列表 |
| GET | /me/notifications/unread-count | 未读数 |
| POST | /me/notifications/{id}/read · /read-all | 已读 |
| DELETE | /me/notifications/{id} | 删除 |
| GET | /announcements | 有效公告 |
| GET | /notifications/preferences · PATCH | 通知偏好 |

## 7. 管理 /api/v1/admin（全部要求 RBAC 权限点）

| Method | Path | 权限点 |
|---|---|---|
| GET | /admin/dashboard | dashboard.read |
| GET | /admin/users · /admin/users/{id} | user.read |
| POST | /admin/users/{id}/sanctions · DELETE .../sanctions/{sid} | user.sanction |
| POST | /admin/users/{id}/unban | user.sanction |
| GET | /admin/posts?status= · /admin/comments | post.read |
| POST | /admin/posts/{id}/moderate | post.moderate（版主校验版块范围） |
| POST | /admin/comments/{id}/moderate | comment.moderate |
| GET/POST/PATCH/DELETE | /admin/categories[/{id}] | category.manage |
| POST | /admin/categories/{id}/moderators · DELETE | category.manage |
| GET/POST/PATCH/DELETE | /admin/tags[/{id}] | tag.manage |
| GET/POST/PATCH/DELETE | /admin/announcements[/{id}] | announcement.manage |
| POST | /admin/posts/{id}/feature · /unfeature · /pin | post.feature / post.pin |
| GET | /admin/reports · POST /admin/reports/{id}/resolve | report.handle |
| GET | /admin/moderation/queue | post.moderate |
| GET/POST/PATCH/DELETE | /admin/sensitive-words[/{id}] | moderation.manage |
| GET | /admin/roles · /admin/permissions | role.read |
| POST | /admin/users/{id}/roles · DELETE | role.assign |
| GET | /admin/operation-logs | audit.read |
| GET/PATCH | /admin/settings | settings.manage（公开配置） |
| GET | /admin/sanctions | user.read |

## 8. 阶段 2 接口（积分/问答/投票/活动/申诉/SSE）

| Method | Path | 说明 |
|---|---|---|
| GET | /me/points · /me/points/transactions | 积分账户（余额/等级/签到状态）与流水 |
| POST | /me/check-in | 每日签到（幂等，+5 分） |
| GET | /badges · /users/{username}/badges | 勋章列表/用户勋章 |
| POST/DELETE | /comments/{id}/accept | 采纳/取消最佳回答（提问者；每帖唯一） |
| GET | /posts/{id}/poll · POST /posts/{id}/poll/vote | 投票结果/投票（唯一约束防重，支持改票规则） |
| GET | /activities · /activities/{id} | 活动列表/详情 |
| POST/DELETE | /activities/{id}/join | 报名/取消（行锁+容量检查） |
| POST | /appeals · GET /me/appeals · GET /me/sanctions | 申诉提交/我的申诉/我的处罚（申诉入口） |
| GET | /me/notifications/stream | SSE 实时通知（Redis Pub/Sub，先落库后推送） |
| POST | /admin/posts/batch-moderate | 批量审核（≤50，结果明细+汇总审计） |
| POST | /admin/users/{id}/points-adjust | 积分调整（备注必填，写流水+审计） |
| POST | /admin/users/{id}/badges · DELETE | 授予/收回勋章 |
| POST/PATCH/DELETE | /admin/activities[/{id}] · POST .../attend | 活动管理/标记出席（发奖励） |
| GET | /admin/appeals · POST /admin/appeals/{id}/handle | 申诉处理（通过自动解罚） |
| GET/POST | /admin/badges | 勋章管理 |

## 9. 其他

- `GET /healthz`（liveness）、`GET /readyz`（DB+Redis 就绪）、`GET /files/{key}`（本地存储静态服务）。
- OpenAPI 规范：`docs/openapi.yaml`（阶段 1 覆盖核心资源；持续维护）。
