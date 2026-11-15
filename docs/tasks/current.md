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

### 下一步
1. Docker 环境真实运行联调（迁移 000001–000009 + 演示链路）。
2. 知识库/AI 扩展（规划中，按需立项）。
3. 可选优化：WebSocket 传输层替换 SSE、消息表情回应、积分规则配置化、sqlc、S3 驱动。
