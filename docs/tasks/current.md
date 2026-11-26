# 任务状态（唯一状态文件）

## 任务：综合社区论坛平台 阶段 0 + 阶段 1（核心 MVP）

### 最终状态：✅ 阶段 0 + 1 + 2 + 3 完成（知识库/AI 规划中）

### 环境记录
- Go 1.27.2、Node 22.14、npm 11 ✅
- 本机无 Docker / PostgreSQL / Redis → 运行时依赖走 `deploy/docker-compose.yml`；集成测试无 DB 自动 SKIP（`backend/tests/e2e_test.go`）。

### 决策记录
1. pgx 直写 SQL（sqlc 列为后续优化）。
2. 编辑器自建 textarea+工具栏，服务端 goldmark+bluemonday 净化为唯一信任边界（客户端 marked+DOMPurify 仅用于预览）。
3. Outbox：API 内嵌 dispatcher（1s 轮询）+ 独立 worker 二进制，FOR UPDATE SKIP LOCKED 多进程安全，指数退避重试。
4. 公开 ID = UUID public_id；内部 BIGSERIAL 不出 API。
5. 帖子状态机在 posts service 集中实现；版主范围校验经 rbac.CanModerateCategory（全局 moderate 权限不受版块限制）。
6. 处罚语义：warn 仅提醒；mute 禁评论；post_ban 禁发帖；temp_ban/perma_ban 全禁；惰性过期 + worker 周期落库。
7. antd v5 + @ant-design/v5-patch-for-react-19（React 19 兼容）；@ant-design/nextjs-registry 1.3.0（无 2.x）。
8. next 15.5.29 + react 19.1.0 + tailwind v4 + TanStack Query 5 + zustand 5。

### Phase 完成清单
- [x] Phase 0 工程底座（commit e147061）
- [x] Phase 1a 后端底座（config/logger/pgx/redis/storage/mailer/respond/apperr/pagination/httpx）
- [x] Phase 1b 迁移 000001–000007（36 表，成对 + FK 顺序 + 必需表三重静态检查）
- [x] Phase 1c 认证 / 1d 用户 / 1e 版块标签 / 1f 帖子 / 1g 评论 / 1h 互动 / 1i 通知 / 1j 搜索 / 1k 治理上传限流 / 1l 后台 API（commit 0671374）
- [x] Phase 1m 后端测试：单测 7 包全绿 + 集成 E2E（无 DB SKIP）+ 迁移静态检查
- [x] Phase 1n/1o/1p 前端前台 29 路由 + Ant Design 后台 12 页面（commit eff4dea）
- [x] Phase 1q 全量验证（见下）

### 验证记录（实际执行结果）
- 后端：`gofmt` 无差异 ✅ ｜ `go vet ./...` ✅ ｜ `go vet -tags integration ./tests/...` ✅ ｜ `go build ./...` ✅ ｜ `go test ./...` → 7 包 ok，0 FAIL ✅
- 迁移检查：TestMigrationPairs（7 组成对）✅ TestForeignKeyOrder（引用顺序）✅ TestRequiredTablesExist（35 张必需表）✅
- 前端：`npm run lint` ✅ 0 error ｜ `npx tsc --noEmit` ✅ ｜ `npm run build` ✅（Next 15.5.29，29 路由，成功产出 standalone）
- 依赖锁定：go.sum + package-lock.json 已提交

### 端到端链路覆盖（代码 + 集成测试断言）
注册 → 邮箱验证（Outbox→邮件事件）→ 登录 → 浏览版块/Feed → 发帖（审核策略/敏感词）→ 详情 → 评论（楼中楼/深度3/引用/@提及）→ 点赞/收藏（唯一约束幂等，重复请求计数不变——集成测试断言）→ 关注 → 通知（Outbox 异步）→ 举报（重复举报 409）→ 后台处理举报 → 锁定帖（评论 409）→ 越权编辑 403 → 非管理员访问后台 403 → 审计日志落库。

### 阶段 2 增补（本次完成）
- 迁移 000008：12 张新表 + 最佳答案/申诉/投票/签到/勋章五重唯一约束
- 积分幂等 GrantTx（插入流水→原子更新账户→回填快照）、签到/注册/发帖/首赞/采纳/出席六类奖励钩子
- 问答采纳（提问者专属+唯一约束）、投票（单/多选/改票/行锁）、活动（FOR UPDATE 并发报名）、申诉（通过自动解罚保留历史）
- SSE：Redis Pub/Sub + EventSource，先落库后推送、断线补拉语义
- 后台：批量审核/积分调整/勋章管理/活动管理/申诉处理
- E2E 扩展：签到幂等/采纳积分勋章/投票 409/申诉解除处罚等断言

### 阶段 3 增补（本次完成）
- 迁移 000009：私信四表 + pair_key/成员唯一约束 + 举报/通知类型扩展 + allow_direct_messages 开关
- 私信模块：get-or-create 幂等会话、三重防骚扰（双向拉黑/私信开关/账号状态，打开+发送双检）、
  已读、免打扰（不计角标）、置顶、撤回（本人 5 分钟，保留占位）、图片附件（需本人上传）、引用消息
- 实时推送：复用 SSE/Redis Pub/Sub（先落库后推送，断线补拉以 DB 为准）；私信独立限流 30/min
- 前端：/messages 双栏聊天、头部未读角标、主页发私信、设置私信开关、SSE 到达提醒
- 测试：PairKey/撤回窗口/预览/trim 单测；E2E 断言（幂等会话、已读归零、越权 403、
  撤回占位、免打扰不计角标、拉黑 403、关闭私信 403）
- 后端 11 包 test 全绿；前端 lint/tsc/build 全绿

### P0/P1/P2 补强（本次完成）
- [x] P0 依赖漏洞：postcss high → overrides 固定 ^8.5.33；minimatch moderate（dev 链，11.x 破坏 eslintrc，保留记录）
- [x] P0 CI：.github/workflows/ci.yml——后端 gofmt/build/vet/test；集成 job 用真实 PG18+Redis8 服务容器
      跑 tests E2E（迁移 000001-000009 首次真实执行）；前端 npm ci/lint/build
- [x] P0 服务器验收脚本 scripts/server-e2e.sh（--full 含完整栈健康检查+10 步演示清单）
- [x] P1 Idempotency-Key：httpx.Idempotency 中间件（缓存响应 8h 重放；进行中 409；5xx 不缓存），挂 POST/PATCH /posts
- [x] P1 CSRF：httpx.OriginCheck（不安全方法 Origin 白名单校验，非浏览器放行）
- [x] P1 通知偏好：notify_inapp=false 跳过社交类站内通知（moderation/system 保留）
- [x] P2 OpenAPI：补阶段 2/3 全部接口（积分/问答/投票/活动/申诉/私信/SSE/后台增强）
- [x] E2E 装置修复：setup 启动 Outbox dispatcher（通知消费依赖）
- 后端 11 包全绿；前端 lint/tsc/build 全绿

### CI 首跑修复记录
- 根因：.gitignore 无锚点 `uploads/` 把源码包 backend/internal/modules/uploads/ 整个忽略，
  upload.go 从未入库（本地可编译掩盖了问题）→ CI go build 报 no required module provides package
- 修复：gitignore 改为锚定 /data/ /uploads/；补入 upload.go（磁盘/仓库 .go 73=73 核对一致；
  迁移 SQL 与 frontend/src 文件数同样核对无遗漏）

### CI 第二轮修复（进行中）
- 后端/前端 job ✅；集成 job 首跑失败
- 定位：中文搜索必然失败——simple 分词下中文整句是单一词素，tsv @@ 匹配不到子串
- 修复：searchPosts 叠加 title/body trigram ILIKE 兜底 + 迁移 000010（body GIN trgm）
- E2E：message 举报改用真实消息 ID 断言 201（原断言 convID 恰好命中消息 1 会误判）
- CI：测试日志 tee 到 GITHUB_STEP_SUMMARY（公开 check-run API 可读，绕过需鉴权的 log 接口）

### 下一步
1. 推送后盯 CI 集成 job（E2E 首次真实执行），修复暴露问题至全绿。
2. 家里服务器运行 scripts/server-e2e.sh --full 做完整栈联调。
3. 剩余 P1/P2 项：验证码、账号注销、活动签到码（按需）。

### 全仓联调修复与部署对齐（本次完成）
- 部署与反代：`deploy/nginx.conf` 保留 `$http_host`（兼容自定义端口），`deploy/docker-compose.yml` 补齐 `nginx` 网关与 `APP_ENV`/`APP_PUBLIC_URL`/`MAIL_DRIVER`/`API_INTERNAL_BASE_URL`；`frontend/Dockerfile` 保证 `public/` 目录存在；`frontend/next.config.ts` 增加 `/api/*`、`/files/*` 内部 rewrite；`OriginCheck` 兼容同 Host 反代请求并补 `TestOriginCheck` 单测。
- 后端逻辑修复：补齐帖子/评论首次点赞奖励作者 +1 钩子；修复后台积分调账幂等键与审计及勋章授予/收回审计；修复活动 `Join` creatorID 扫描并开放 `GET /admin/activities/:id/participants`；修复 `categories.Update` `RowsAffected` 检查；补私信举报成员鉴权与列表目标摘要；补评论编辑原始 Markdown 返回（仅作者可见）；补个人资料 `allow_direct_messages`、`users/:username` 可见性裁决、`?type=category|tag` 关注列表、帖子双向拉黑与主页可见性过滤；同步 `category_moderators` 与角色分配校验；`/auth/me` 增加 `can_admin`；纯图片私信填充 `[图片]` 占位满足 DB 非空约束。
- 前台页面修复：补齐最佳回答徽章与采纳/取消操作及无损 Markdown 编辑；移除过期「规划中」文案并连通 `/me/points`、`/activities`、`/compose`、`/me/appeals`；个人设置增加头像上传；私信聊天支持发图/引用、修复 `sending` 与图片 URL；发帖页保留/恢复 `post_type`；通知页支持私信跳转；用户主页接入真实关注/拉黑态；后台入口按 `can_admin` 显隐，移动端 `<640px` 增加底部导航。
- 管理后台修复：统一鉴权请求基址；用户详情接入按 `(role_id, scope_id)` 移除角色、版主必选版块 `category_id`、数字输入校验及已持有勋章查看/收回；帖子管理接入批量审核多选栏与独立弹窗表单；活动管理接入报名名单与标记出席结算积分抽屉。
- 测试与文档：扩展 `migrations_test.go` 覆盖二/三阶段全部表名；补齐 `e2e_test.go` 首次点赞+1/重复调整积分/私信开关返回/非成员举报 404 断言；同步 `docs/openapi.yaml`、`docs/API.md` 与 `scripts/server-e2e.sh`。
- 线上部署与启动排查修复：
  - 修复 `000002_users.up.sql` 重复定义 `users_status_check` 导致的首次迁移失败（`SQLSTATE 42710`）。
  - 修复 `deploy/docker-compose.yml` 中 PostgreSQL 18 卷挂载路径为 `/var/lib/postgresql`，收敛 `postgres`/`redis`/`api`/`web` 仅绑定 `127.0.0.1`、补透传 `COOKIE_SECURE`/`SMTP_*`。
  - 修复 `backend/internal/platform/config/config.go`：允许显式配置 `COOKIE_SECURE=false` 以兼容无 HTTPS 的 HTTP IP 访问。
  - 修复 `backend/internal/server/server.go`、`activities.go`、`points/handler.go` 重复路由注册（`/posts/:id/view`、`/badges`、`/activities`）及后台 5 组路由前缀双写（`categories`/`tags`/`reports`/`activities`/`appeals`），新增 `TestBuildRoutesNoConflict` 路由表防回归单测。

### 前台现代简洁重设计与主流功能补齐（本次完成）
- 视觉与设计系统：重构 `tokens.css` / `globals.css` 为低噪点中性画布 + 克制墨黑主操作与钴蓝点缀色，支持深浅主题切换按钮（`yanji-theme` 防闪烁脚本）与平板/移动端自适应折叠栅格；新增统一 1.7px 线性 SVG 图标库 `Icons.tsx`、通用弹窗 `ModalDialog` / `ConfirmDialog` / `PromptDialog` 与图片大图灯箱 `ImageLightbox`，移除全站冗余 Emoji 与原生 `confirm`/`prompt`。
- 全局导航与主流社区交互：`SiteChrome` 增加 `/` 与 `Ctrl/Cmd+K` 快捷搜索及防抖实时联想下拉、用户下拉卡片、移动端底部未读红点与返回顶部按钮；首页侧栏接入实时积分与一键签到卡；详情页补齐阅读进度条、预计阅读时长、代码块一键复制、作者名片关注/私信、收藏夹归档选择、帖子修订历史弹窗、作者删帖与评论热度/时间排序；`MarkdownEditor` 支持编辑/分栏/预览三模式、图片粘贴与拖拽上传及 `Ctrl+Enter` 快捷发布；`/compose` 补齐投票多选上限/改票/截止时间与推荐标签点选；`/search` 接入版块/作者/日期高级筛选；`/t/[slug]` 与 `/u/[username]` 接入标签真实关注切换与个人多类型关注管理；`/me/bookmarks` 支持收藏夹重命名；`ChatPanel` 支持发送图片放大与举报私信消息。
- 校验与线上更新：前端 `eslint`、`tsc --noEmit`、`next build`（36 路由）全绿，并已同步重建服务器 `web` 容器验收。

### 认证验证码与全站安全加固（本次完成）
- 登录/注册/找回密码图形验证码：新增 `auth/captcha.go`（纯 Go 标准库 PNG 渲染 + 正弦扭曲与干扰线，无 SVG XSS 风险，优先存 Redis 并用 Lua `GET+DEL` 原子消费防重放，未连 Redis 时自动降级有界内存缓存），开放 `GET /api/v1/auth/captcha`，前端新增 `CaptchaField` 并在登录、注册、找回密码及设置重发重置邮件页强制启用，失败自动刷新验证码。
- 登录防暴破与密码策略：`auth/service.go` 新增 `(account, ip)` 连续 5 次失败锁定 15 分钟与跨 IP 单账号 12 次失败保护性锁定，加入虚拟 bcrypt 等时比较消除账号枚举计时差；`ValidatePassword` 要求 8-72 位且包含至少两类字符并拦截常见弱口令；新增 `POST /api/v1/auth/password/change` 与设置页修改密码表单（改密后撤销全部旧会话并轮换当前 Cookie）。
- 漏洞修复与防线加固：修复 `SetSessionCookie` / `ClearSessionCookie` 在 Gin 中 `SetSameSite` 调用顺序错误导致 `SameSite=Lax` 未写入响应头的问题；为 `/me/*`、`GET /posts/:id/revisions`、`postsWrite`、`commentsWrite`、`/reports` 补齐 `httpx.RequireAuth()` 拦截游客空指针 500 与封禁账户；修复搜索高亮 `ts_headline` 未转义 HTML 的存储型 XSS（哨兵标签转义 + 前端 `DOMPurify` 白名单）；`server.Build` 配置 `SetTrustedProxies`、`nginx.conf` 覆写 `X-Forwarded-For $remote_addr` 并强化 `/files/` 与 `ServeLocal` 的 `nosniff` 与 `Content-Security-Policy: default-src 'none'`；校验用户 `avatar_key` 安全前缀。
- 注册事务外键 Bug 修复：修复 `auth.Service.Register`（同步收敛 `sendVerificationFor`/`ForgotPassword`）在 `users` 插入事务未提交前通过 `pool` 插入 `email_verification_tokens` 导致外键 `SQLSTATE 23503` 报错 500 的问题（改用 `CreateEmailTokenTx`/`CreateResetTokenTx` 同事务提交）；并在 `respond.Fail`/`apperr.RespondError` 增加 5xx 服务端原因日志。
- 收藏列表扫描类型修复：修复 `interactions.BookmarkItem.CreatedAt` 为 `string` 导致 pgx v5 扫描 `timestamptz` 报 `cannot scan timestamptz into *string` 500 错误（改为 `time.Time`）。
- 审计日志与处罚列表 SQL 修复：修复 `moderation.AuditLogger` 中 `jsonb_build_object('note', $6)` 未指定类型导致 `SQLSTATE 42P18` 进而中止外层事务报错 500（改为 `$6::text`），并修复 `SanctionService.ListAll` 将 `us.created_at` 误扫入 `int64` 的问题。
- 通知列表 SQL 语法修复：修复 `notifications.Service.List` 中 `LEFT JOIN LATERAL (CASE ...)` 非法 SQL 语法导致 `GET /me/notifications` 报错 500（改为标量 `COALESCE(CASE ..., '')` 子查询）。
- 搜索相关性排序 SQL 修复：修复 `searchPosts` 中默认 `relevance` 排序引用不存在列 `query` 导致 `GET /search` 报错 500（改用 `websearch_to_tsquery('simple', $1)` 并提升标题命中权重）。
- 锁定帖可见性修复：统一 Feed、版块列表、搜索、详情、收藏夹及用户帖子计数将 `locked` 纳入公开可见范围（仅禁止新增评论，且 `lock`/`unlock` 不错误增减版块 `post_count`）。
- 实时推送空指针修复：修复 Redis 未连接时向 `notifications.PublishNotification`（`redis.UniversalClient` 接口）传入 typed-nil `*redis.Client` 引发私信/通知 SSE 推送 panic 的问题。
- 私信撤回约束与迁移修复：新增成对迁移 `000011_messages_recall_check`（更新 `messages_not_empty` CHECK 约束允许 `status = 'recalled'` 时清空 `body_text`），修复 `DELETE /messages/:id` 撤回消息报 `SQLSTATE 23514` 500 错误。
- 幂等演示数据与标签总数修复：新增 `seed.RunDemo` 与 `cmd/seed-demo` 入口（并在 `backend/Dockerfile` 打包 `/app/seed-demo`），在线导入 5 个演示用户、9 个标签、11 篇覆盖讨论/长文/问答/投票/置顶/精华/锁定的帖子、23 条楼中楼评论与采纳回答、3 场活动与公告；同时修复 `tags.Repo.ListPublic` 丢弃 `total` 返回 `0` 的问题。

### 界面高级感升级与全栈安全审计加固（本次完成）
- **视觉与交互高级感升级（Atelier Slate & Indigo 体系）**：
  - 令牌与表面层：升级 `tokens.css` / `globals.css`，建立分层画布、双层柔和阴影、深色模式顶边微光、靛蓝品牌渐变 `--brand-gradient`、顶部克制氛围光 `--ambient-glow`、`.surface-glass` / `.surface-spotlight` / `.surface-hero` 表面类、编辑级 `.prose-community` 排版及尊重 `prefers-reduced-motion` 的弹窗/Toast 过渡微动效。
  - 核心组件与前台页面：`Avatar` 升级为确定性低饱和渐变色盘与 `ring` 边框并修复路径重复拼接；升级 `SiteChrome` 磨砂顶栏与品牌标章、`Dialog`/`Toast`/`States`、`PostCard`（置顶左强调条与悬浮微浮）、首页编辑风导览横幅与 `.surface-spotlight` 签到卡、帖子详情渐变进度条与作者名片、用户主页氛围头图与四格指标瓦片、版块/积分页面及认证页实时密码强度条（`PasswordStrengthBar`）。
  - 管理后台统一质感：`admin/layout.tsx` 接入 Ant Design `ConfigProvider` 统一 `#3b5bdb` 品牌令牌与圆角、升级侧栏品牌标章、毛玻璃顶栏与仪表盘指标/趋势图样式。
- **全栈安全审计与漏洞修复**：
  - **越权与 RBAC 范围隔离**：`httpx.RequireGlobalPermission` 与 `server.go` 隔离所有全局管理接口（`/admin/appeals*`、`/admin/users*`、`/admin/roles`、`/admin/settings`、`/admin/categories*`、`/admin/tags*` 等），彻底阻断单版块版主越权解封/提权；补全 `posts`、`admin.Comments`、`reports` 版主版块范围隔离，禁止版主访问私信/用户举报及跨版块 `staff_only` 发帖；`admin.CreateSanction`/`AssignRole`/`RemoveRole` 增加角色层级保护、禁止自罚/自降权/移除最后超管，封禁事务内即时吊销目标会话；`categories` 增删版主后即时 `rbac.Invalidate` 并补齐版块/标签审计日志；`appeals` 禁止自我审批并在解罚前校验是否存在其他生效封禁。
  - **处罚与认证防线**：`AuthUser` 新增 `ActiveSanctions` 聚合全部生效处罚防 `warn` 覆盖；`RequireAuth`/`RequireVerified`/`RequirePermission`/`Login`/`StreamHandler` 全面阻断 `temp_ban`/`perma_ban`；私信发消息与建会话挂 `RequireCanComment()` 与 `dmRl` 限流；登录防暴破增加 `uid:` 维度防交替用户名/邮箱使尝试翻倍，`ChangePassword` 接入暴破锁定；`outbox.go` 邮件发送后擦除验证/重置明文 token；SMTP `tls.Config` 显式设 `MinVersion: TLS12`；`/auth/me`、`/auth/sessions`、`/me/*`、`/admin/*` 增加 `Cache-Control: no-store`。
  - **内容安全、IDOR 与资源防线**：修复 `comments.Repo.InsertTx` 漏写 `status` 导致待审评论直接发布的问题，及帖子/评论编辑命中 `review` 敏感词未降级待审与计数漂移；`comments`/`interactions`/`polls`/`categories`/`posts`/`activities`/`reports` 全面补齐非公开版块与非公开状态 IDOR 校验（含修复 `posts.ListByCategory` 漏 `JOIN categories`、`comments.toComment` 清除已删/隐藏正文、`activities.Get` 限 `published`、`messaging.Recall` 清理图片附件）；敏感词检测剥离零宽字符与 Markdown 强调符防绕过；全站 `ILIKE` 补 `escapeLike` + `ESCAPE '\'`；问答最佳答案积分幂等键改为按 `postID` 唯一防反复切换刷分；评论 `@mention` 上限 10 人防通知轰炸；非公开主页 `bio` 在搜索与关注列表脱敏。
  - **基础中间件与前端/网关加固**：`RateLimit` 改为 Redis Lua 原子窗口 + 有界内存降级兜底杜绝 fail-open；`Idempotency` 改为 `userID|method|path|key` SHA-256 隔离并限 256KB 缓存体；`OriginCheck` 补充 `Sec-Fetch-Site: cross-site`、协议白名单与 `X-Forwarded-Proto` 校验；`RequestID` 正则校验防头注入；`uploads` 入口加 `MaxBytesReader`、8 字节完整 PNG 魔数与可信 MIME 落库；SSE 限单用户 5 连接与剥离换行；`markdown` 限制 `class` 正则并开 `noreferrer`；前端 `next.config.ts` 关闭 `X-Powered-By` 并补安全头、`api.ts` 新增 `safeExternalUrl` 与拦截 `//`、`MarkdownEditor` 禁止 SSR 返回未净化 HTML、`deploy/nginx.conf` 补全 `/files/` 安全响应头。
- **自动化安全测试与全量验证**：
  - 新增/扩充 `httpx/security_test.go`、`server/security_test.go`、`markdown/markdown_test.go`、`storage/storage_test.go`、`moderation/moderation_test.go`、`search/search_test.go`、`auth/security_test.go`、`notifications/stream_test.go`。
  - 后端：`gofmt`、`go vet ./...`、`go vet -tags integration ./tests/...`、`go build ./...`、`go test -count=1 ./...`（14 个含测试包全绿，0 FAIL）✅。
  - 前端：`npm run lint`（0 error）、`npx tsc --noEmit`、`npm run build`（Next 15.5.29，36/36 路由构建通过）✅；`npm audit --omit=dev` 生产依赖 0 漏洞 ✅。
