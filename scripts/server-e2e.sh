#!/usr/bin/env bash
# ============================================================
# 服务器一键验收脚本（在有 Docker 的机器上运行）
# 用途：真实执行迁移 000001-000010 并跑通三阶段端到端验收链路（支持 :80 Nginx 网关及 :3000/:8080）。
# 用法：./scripts/server-e2e.sh [--full]
#   --full  额外启动完整服务栈并做健康检查与冒烟测试
# ============================================================
set -euo pipefail
cd "$(dirname "$0")/.."
ROOT="$(pwd)"

echo "==> 1/5 启动依赖（PostgreSQL 18 + Redis 8）"
docker compose -f deploy/docker-compose.yml up -d postgres redis

echo "==> 2/5 等待健康检查"
for i in $(seq 1 30); do
  if docker compose -f deploy/docker-compose.yml exec -T postgres pg_isready -U yanji -d yanji_community >/dev/null 2>&1; then
    echo "    PostgreSQL 就绪"; break
  fi
  [ "$i" = 30 ] && { echo "PostgreSQL 启动超时"; exit 1; }
  sleep 2
done

echo "==> 3/5 准备独立验收数据库 yanji_community_test"
docker compose -f deploy/docker-compose.yml exec -T postgres psql -U yanji -d yanji_community -c \
  "SELECT 'CREATE DATABASE yanji_community_test' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname='yanji_community_test')\gexec" >/dev/null

export TEST_DATABASE_URL="postgres://yanji:yanji_dev_password@127.0.0.1:5432/yanji_community_test?sslmode=disable"

echo "==> 4/5 运行端到端集成测试（真实迁移 + 全业务链路断言）"
cd "$ROOT/backend"
if [ ! -d "$HOME/go/pkg/mod" ] && [ ! -f go.sum ]; then echo "缺少依赖缓存"; fi
go test ./tests/... -tags integration -v -count=1 -timeout 10m

echo "==> 5/5 结果"
echo "✅ 端到端验收全部通过：迁移真实执行 + 三阶段业务闭环断言通过"

if [ "${1:-}" = "--full" ]; then
  echo ""
  echo "==> 启动完整服务栈（api + web）"
  cd "$ROOT"
  docker compose -f deploy/docker-compose.yml up -d --build
  echo "    等待 API 就绪..."
  for i in $(seq 1 60); do
    if curl -sf http://127.0.0.1:8080/readyz >/dev/null 2>&1; then break; fi
    [ "$i" = 60 ] && { echo "API 启动超时"; docker compose -f deploy/docker-compose.yml logs api | tail -30; exit 1; }
    sleep 2
  done
  echo "✅ /healthz:  $(curl -s http://127.0.0.1:8080/healthz)"
  echo "✅ /readyz:   $(curl -s http://127.0.0.1:8080/readyz)"
  echo "✅ 前台:      http://$(hostname -I 2>/dev/null | awk '{print $1}'):3000 （或本机 http://localhost:3000）"
  echo "✅ API:       http://localhost:8080/api/v1"
  echo ""
  echo "---------- 手工演示清单（PRD 第九部分）----------"
  echo " 1. 游客浏览首页最新/热门/精华 Tab 与帖子详情"
  echo " 2. 注册 admin@example.com 之外的新账号 → 查收验证邮件（默认 log 驱动，见 api 容器日志）→ 验证登录"
  echo " 3. 发帖（Markdown/标签/图片）→ 另一账号评论楼中楼、点赞、收藏"
  echo " 4. 作者通知中心查看并标记已读"
  echo " 5. 举报评论 → 管理员后台处理举报"
  echo " 6. 版主授权版块置顶/锁定/隐藏；跨版块操作应 403"
  echo " 7. 管理员查看审计日志 /operation-logs"
  echo " 8. 重复点赞/关注/收藏不重复计数；锁定帖评论 409；编辑他人内容 403"
  echo " 9. 积分中心签到、问答帖采纳最佳回答、投票帖防重复"
  echo "10. 双账号互发私信 → 已读/撤回/免打扰；重启 api 容器后数据仍在"
fi
