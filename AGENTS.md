# AGENTS.md — 项目工程规范（必读）

> 任何 Agent/开发者在修改本仓库前必须先读本文件。与领域 Skill（go-dev-standard / frontend-dev-standard / antd-ui-expert 等）配合使用；冲突时以本文件更具体的约定优先。

## 项目概览

- **项目**：综合社区论坛平台（开放注册、版块/帖子/评论/互动/治理/后台）。
- **架构**：模块化单体。`backend/` Go API（Gin + pgx v5 + PostgreSQL 18 + Redis 8），`frontend/` Next.js App Router（React 19 + TS + Tailwind tokens，后台 Ant Design 5）。
- **当前阶段**：阶段 1 核心 MVP（见 `docs/DEVELOPMENT_PLAN.md`）；阶段 2/3 功能一律标记「规划中」，禁止做成假实现。

## 目录约定

```
backend/cmd/{api,worker}      入口
backend/internal/platform     config/logger/database/redis/storage/mailer/migrate/seed
backend/internal/shared       apperr/respond/pagination/validator
backend/internal/modules/*    auth/users/follows/categories/tags/posts/comments/
                              interactions/notifications/reports/moderation/search/uploads/admin
backend/migrations/           NNNNNN_name.up.sql / .down.sql（成对）
frontend/src/{app,components,features,lib,stores,styles}
docs/                         PRD/ARCHITECTURE/DATABASE/API/PERMISSIONS/UI_SPEC/DEVELOPMENT_PLAN + tasks/current.md
deploy/docker-compose.yml     postgres+redis+api+web
```

## 常用命令

| 场景 | 命令（在 backend/ 或 frontend/ 下执行） |
|---|---|
| 后端构建 | `go build ./...` |
| 后端检查 | `go vet ./...` |
| 后端格式化 | `gofmt -w .` |
| 后端测试 | `go test ./...`（集成测试无 `TEST_DATABASE_URL` 时自动 SKIP） |
| 集成测试 | `TEST_DATABASE_URL=postgres://... go test ./tests/... -tags integration` |
| 前端安装 | `npm install` |
| 前端构建 | `npm run build` |
| 前端 Lint | `npm run lint` |
| 本地全套启动 | 根目录 `docker compose -f deploy/docker-compose.yml --env-file .env up -d --build` |

## 编码规范

### Go
- 分层：handler（参数/响应）→ service（事务/权限/状态机/Outbox）→ repo（SQL）。业务逻辑禁止写进 handler。
- 错误：统一 `shared/apperr`（`apperr.New(httpStatus, code, msg)`），handler 层只转换不吞错；禁止暴露原始 DB 错误。
- 响应：统一 `shared/respond.OK / Fail`，带 request_id。
- SQL：参数化，禁止拼接字符串；模块私有表，跨模块经 service。
- 计数更新用 `UPDATE ... SET n = n + 1` 原子 SQL；互动表依赖 DB 唯一约束防重。
- 时间 UTC；公开 ID 一律 UUID（public_id），API 路由不暴露内部 BIGINT。
- 新增表必须落外键/唯一/CHECK/索引并同步更新 `docs/DATABASE.md` 与成对迁移。
- 单元测试优先覆盖纯逻辑（状态机/校验/热度/权限）；集成测试放 `backend/tests/`，无 DB 环境 `t.Skip`。

### 前端
- App Router；页面在 `src/app`；业务组件按域放 `components/{post,comment,editor,admin}` 与 `features/*`。
- 服务端数据一律 TanStack Query；跨页客户端状态才用 Zustand（auth、编辑器草稿）。
- API 调用统一走 `lib/api.ts`（`credentials:'include'`）；按错误码映射提示，不解析中文 message。
- 样式用 `styles/tokens.css` 变量 + Tailwind 工具类；后台统一 Ant Design。
- 每个数据页面必须有 loading/empty/error 三态；表单防重复提交。
- 禁止 localStorage 存长期凭证；SSR 组件不碰 window/localStorage。

### 安全红线（最高优先级）
1. 不提交任何真实密钥/密码/Token；环境变量进 `.env`（已 gitignore），样例进 `.env.example`。
2. 权限一律服务端校验（RBAC + 对象级 + 版主范围）。
3. Markdown 渲染必须净化（bluemonday）；上传校验扩展名+MIME+magic bytes+大小，随机存储名。
4. 审计日志不记录密码/Token/Cookie。
5. 重要状态变化（审核/处罚/版块管理）必须写 operation_logs。

## Git 提交规范

Conventional Commits（中文 subject）：`feat(posts): 帖子搜索与高亮`、`fix(auth): 修复会话续期`；一个完整切片一个 commit；提交前跑 `go build/vet/test` 与前端 `npm run build`。

## 任务状态文件

长期任务状态唯一来源：`docs/tasks/current.md`。完成阶段必须更新该文件，禁止新建第二份状态文档。
