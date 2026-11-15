# 任务状态（唯一状态文件）

## 任务：综合社区论坛平台 阶段 0 + 阶段 1（核心 MVP）

### 目标
空仓库交付可运行的论坛平台：账号/版块/标签/帖子/评论/互动/搜索/通知/治理/上传/RBAC/审计 + 前台与 Ant Design 后台，端到端业务闭环。

### 环境
- Go 1.27.2、Node 22.14、npm 11 ✅
- Docker / PostgreSQL / Redis 本机缺失 → 集成测试无 DB 自动 SKIP；运行验证走 docker compose。

### 决策记录
1. pgx 直写 SQL（sqlc 列为后续优化）。
2. 编辑器自建 textarea + 工具栏，服务端 goldmark+bluemonday 净化为唯一信任边界。
3. Outbox：API 内嵌 dispatcher（1s 轮询）+ 独立 worker 二进制。
4. 公开 ID = UUID public_id；内部 BIGSERIAL 不出 API。
5. 集成测试 `//go:build integration`，无 TEST_DATABASE_URL 时 SKIP。

### Phase 清单（随进度更新）
- [x] Phase 0 工程底座：git/骨架/gitignore/.env.example/compose/Dockerfile/AGENTS/README/docs 七文档
- [ ] Phase 1a 后端底座（go.mod/config/logger/db/redis/respond/apperr/middleware/health）
- [ ] Phase 1b 迁移 000001–000007
- [ ] Phase 1c 认证模块
- [ ] Phase 1d 用户模块
- [ ] Phase 1e 版块/标签
- [ ] Phase 1f 帖子
- [ ] Phase 1g 评论
- [ ] Phase 1h 互动
- [ ] Phase 1i 通知
- [ ] Phase 1j 搜索
- [ ] Phase 1k 治理/上传/限流
- [ ] Phase 1l 后台 API
- [ ] Phase 1m 后端测试 + 验证
- [ ] Phase 1n 前端底座
- [ ] Phase 1o 前台页面
- [ ] Phase 1p 后台页面
- [ ] Phase 1q 全量验证（build/vet/test/lint/build/迁移检查）

### 验证记录
（待各 Phase 完成后填写实际命令与结果）

### 下一步
- 需要 PostgreSQL/Docker 的端到端联调（compose up 后手工验证演示链路）。
- 阶段 2：积分/问答/投票/活动/SSE（规划中）。
