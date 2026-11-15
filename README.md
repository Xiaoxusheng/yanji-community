# 综合社区论坛平台（yanji-community）

开放注册的综合社区论坛：版块、帖子（Markdown）、评论楼中楼、点赞/收藏/关注、搜索、通知、举报审核、RBAC 管理后台。

## 技术栈

| 端 | 技术 |
|---|---|
| 后端 | Go 1.27 + Gin + PostgreSQL 18 (pgx v5) + Redis 8 + golang-migrate |
| 前端 | Next.js (App Router) + React + TypeScript + Tailwind Design Tokens |
| 后台 | Ant Design 5 |
| 数据请求 | TanStack Query ｜ 客户端状态：Zustand |
| 本地运行 | Docker Compose（postgres + redis + api + web） |

## 快速启动（Docker Compose）

```bash
cp .env.example .env
docker compose -f deploy/docker-compose.yml --env-file .env up -d --build
# 前台: http://localhost:3000   API: http://localhost:8080/api/v1  健康检查: /healthz
```

首次启动自动执行数据库迁移并写入种子数据（角色/权限/默认版块）。管理员账号取自 `.env` 的 `SEED_ADMIN_EMAIL` / `SEED_ADMIN_PASSWORD`。

## 本地开发（不用 Docker 起 Go/Node）

```bash
# 1) 只用 Docker 启动依赖
docker compose -f deploy/docker-compose.yml up -d postgres redis

# 2) 后端
cd backend
cp ../.env.example .env
go mod download
go run ./cmd/api          # 迁移自动执行, 监听 :8080

# 3) 前端
cd ../frontend
npm install
npm run dev               # http://localhost:3000
```

## 常用命令

```bash
# 后端
cd backend && go build ./... && go vet ./... && go test ./...
# 前端
cd frontend && npm run lint && npm run build
```

集成测试（需要 PostgreSQL）：

```bash
cd backend && TEST_DATABASE_URL="postgres://yanji:yanji_dev_password@127.0.0.1:5432/yanji_community_test?sslmode=disable" go test ./tests/... -tags integration -v
```

## 文档

- 产品需求：[docs/PRD.md](docs/PRD.md)
- 架构：[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- 数据库：[docs/DATABASE.md](docs/DATABASE.md)
- API：[docs/API.md](docs/API.md) ＋ OpenAPI：[docs/openapi.yaml](docs/openapi.yaml)
- 权限：[docs/PERMISSIONS.md](docs/PERMISSIONS.md)
- UI 规范：[docs/UI_SPEC.md](docs/UI_SPEC.md)
- 开发计划与进度：[docs/DEVELOPMENT_PLAN.md](docs/DEVELOPMENT_PLAN.md)、[docs/tasks/current.md](docs/tasks/current.md)

## 阶段状态

- ✅ 阶段 0：工程底座、文档、迁移、CI 级检查
- ✅ 阶段 1：核心 MVP（账号/内容/讨论/互动/搜索/通知/治理/后台）
- ⬜ 阶段 2（规划中）：积分等级勋章、问答投票、活动、SSE、申诉
- ⬜ 阶段 3（规划中）：私信、实时推送、知识库
- ⬜ 阶段 4（规划中）：商业化与规模化

> 未实现功能在界面上均明确标记「规划中」，不以假数据冒充。
