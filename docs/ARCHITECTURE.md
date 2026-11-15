# ARCHITECTURE — 技术架构

## 1. 总体形态

**模块化单体**：一个 Go API 服务（含内嵌 Worker goroutine 处理 Outbox 事件）+ 一个 Next.js 前端。PostgreSQL 唯一事实数据源；Redis 负责限流、幂等键、短期缓存。

```
浏览器 ──HTTP──▶ Next.js (SSR/CSR, :3000)
                    │  fetch /api/v1
                    ▼
                 Go API (Gin, :8080)
                    ├── PostgreSQL 18 (pgx v5 连接池)
                    ├── Redis 8 (限流/幂等/缓存)
                    └── Storage (local / S3 兼容)
```

## 2. 技术栈（锁定版本）

| 层 | 技术 | 版本 |
|---|---|---|
| 语言 | Go | 1.27.x（本机 1.27.2） |
| HTTP | gin-gonic/gin | v1.11 |
| DB 驱动 | jackc/pgx v5（池 + 事务） | v5.7 |
| 迁移 | golang-migrate（库内嵌，启动时自动执行 dev/prod 显式命令） | v4.18 |
| 缓存 | redis/go-redis v9 | v9.16 |
| 密码 | golang.org/x/crypto（bcrypt） | latest stable |
| Markdown | goldmark（渲染）+ microcosmos-cc/bluemonday（净化） | v1.7 / v1.0 |
| 校验 | go-playground/validator | v10 |
| UUID | google/uuid | v1.6 |
| 前端 | Next.js（App Router）+ React + TypeScript | 15.x / 19.x / 5.x |
| UI | 前台自建 Design Token + Tailwind CSS；后台 Ant Design | 5.x |
| 数据请求 | TanStack Query | 5.x |
| 客户端状态 | Zustand（仅认证态/编辑器草稿等跨页状态） | 5.x |

> 决策记录：DB 访问采用 **pgx v5 直接编写 SQL + 薄 Repository 层**（类型安全、无反射开销）。sqlc 代码生成作为后续优化项，不阻塞交付。ORM 不引入。

## 3. 目录结构

```
backend/
├── cmd/api/main.go          # API 入口：装配依赖、路由、优雅退出
├── cmd/worker/main.go       # 独立 Worker 进程（同构 outbox 消费，生产可拆分）
├── migrations/              # 版本化 SQL 迁移（000001_xxx.up.sql / .down.sql）
└── internal/
    ├── platform/            # 基础设施：config / logger / database / redis / storage / mailer / migrate / seed
    ├── shared/              # 横切：apperr(错误码) / respond(统一响应) / pagination / validator
    └── modules/
        ├── auth/            # 注册/登录/会话/邮箱验证/密码重置（handler+service+repo）
        ├── users/           # 资料/设置/公开主页/会话设备
        ├── follows/         # 关注用户/版块/标签、拉黑
        ├── categories/      # 版块、版主、版块关注
        ├── tags/            # 标签、标签关注
        ├── posts/           # 帖子状态机/修订/审核/热度/Feed/附件
        ├── comments/        # 评论树/深度3/引用/@提及
        ├── interactions/    # 点赞/收藏夹
        ├── notifications/   # 通知 + outbox 消费者
        ├── reports/         # 举报
        ├── moderation/      # 敏感词/审核队列/处罚/审计日志
        ├── search/          # PG 全文检索
        ├── uploads/         # 上传（本地/S3 抽象）
        └── admin/           # 后台聚合接口
frontend/
└── src/
    ├── app/                # App Router：(site) 前台 / (admin) 后台 / (auth) 登录注册
    ├── components/         # ui(设计系统) / post / comment / editor / admin / layout
    ├── features/           # auth / feed / posts / comments / search / notifications / profile / admin
    ├── lib/                # api client(带 credentials) / query client / error map / markdown / utils
    ├── stores/             # zustand：auth store、editor draft store
    └── styles/             # design tokens.css、globals.css
```

## 4. 后端分层与请求流

```
Gin Handler（参数绑定/取认证上下文/调用 service/统一响应）
   → Service（事务边界/状态机/对象级权限/领域规则/Outbox 写入）
     → Repository（pgx SQL，模块私有表访问）
```

- 跨模块读取通过在 service 层显式传参/组合，不引入 service 间循环依赖；写操作在单个事务内完成主数据 + outbox_events 写入。
- 认证中间件 → RBAC 中间件 → 限流中间件（Redis 滑动窗口）→ Handler。

## 5. 关键机制

### 5.1 统一响应与错误
`{code, message, data, request_id}`；错误码见 `docs/API.md`。`apperr.AppError{HTTP 状态码、Code、message}` 由 service 抛出，handler 统一转换。

### 5.2 Outbox 可靠异步
`outbox_events(event_type, aggregate_id, payload, status, attempts, next_retry_at)`。业务事务内与主数据同事务写入；API 进程内嵌 dispatcher（1s 轮询 claimed 批量处理）+ 独立 worker 进程同构实现；指数退避重试，超过上限标记 `failed`。事件类型：`notification.created`、`email.verification`、`email.password_reset`、`email.welcome`。

### 5.3 计数与热度
- 点赞/评论/收藏/关注计数：原子 SQL 增减，DB 唯一约束兜底防重。
- 热度分（服务端唯一实现 `posts/hot.go`）：`score = (likes*3 + comments*2 + bookmarks*4 + views*0.1) / (hours_since_publish + 2)^1.2`，权重可由 system_configs 覆盖。

### 5.4 浏览量去重
Redis `SETNX post:view:{postID}:{userOrVisitorHash}` TTL 1h，去重成功才 `view_count + 1`。

### 5.5 上传安全
扩展名白名单 → multipart 头 MIME 白名单 → 实际文件头 magic bytes 校验（jpeg/png/gif/webp）→ 大小限制 → 随机 UUID 存储名 → 存入 Storage（local: 目录 + GET /files/{key} 静态服务；s3: 预留实现接口）。

### 5.6 限流
Redis INCR+EXPIRE 固定窗口（分钟级），按 `bucket:ip` 与 `bucket:user` 双维度：auth 20/min、write 60/min、read 300/min，超限返回 `RATE_LIMITED`。

### 5.7 审计
`operation_logs(actor_id, action, target_type, target_id, request_id, detail, ip, created_at)`；服务层通过 `moderation.AuditLogger` 记录；不记录密码/Token/Cookie。

## 6. 认证与安全

- 会话：128-bit 随机 token → Cookie `yanji_session`；DB 存 SHA-256 摘要；滑动续期；撤销即失效。
- CORS：仅放行 `APP_PUBLIC_URL`；凭据模式允许。
- Cookie `SameSite=Lax` + 同源部署下天然缓解 CSRF；写接口额外校验 `Origin` 头（生产）。
- 安全响应头：`X-Content-Type-Options`、`X-Frame-Options: DENY`、`Referrer-Policy`。
- 日志：slog 结构化 JSON，禁止打印密码/Token。

## 7. 部署

`deploy/docker-compose.yml`：postgres:18-alpine + redis:8-alpine + api（多阶段构建）+ web（standalone）。迁移由 API 启动时自动执行（`MIGRATE_ON_START=true`，生产可关闭改为 CI 执行）。时间统一 UTC。

## 8. 规划中（不在本架构第一期实现）

独立搜索引擎（Meilisearch）、消息队列（Redis Stream 替代 outbox 轮询）、WebSocket/SSE、多级缓存、Prometheus 指标导出。
