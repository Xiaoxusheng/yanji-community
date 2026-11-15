# 任务状态（唯一状态文件）

## 任务：综合社区论坛平台 阶段 0 + 阶段 1（核心 MVP）

### 最终状态：✅ 阶段 0 + 阶段 1 + 阶段 2 完成

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

### 下一步
1. Docker 环境真实运行联调（迁移 000001–000008 + 演示链路）。
2. 阶段 3（规划中）：私信、WebSocket、知识库。
3. 可选优化：积分规则后台可配置、sqlc、OpenAPI 全量、S3 驱动。
