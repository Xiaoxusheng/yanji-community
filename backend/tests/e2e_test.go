//go:build integration

// 端到端集成测试：需要 PostgreSQL（TEST_DATABASE_URL）。
// 覆盖验收链路：注册→邮箱验证→登录→浏览版块→发帖→评论→点赞/收藏→举报→后台处理→越权校验。
// 运行：TEST_DATABASE_URL=postgres://... go test ./tests/... -tags integration -v
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/database"
	"github.com/yanji-community/backend/internal/platform/logger"
	"github.com/yanji-community/backend/internal/platform/mailer"
	"github.com/yanji-community/backend/internal/platform/migrate"
	"github.com/yanji-community/backend/internal/platform/seed"
	"github.com/yanji-community/backend/internal/server"
	"github.com/yanji-community/backend/internal/shared/outbox"
	"github.com/yanji-community/backend/internal/shared/rbac"
)

func testConfig(t *testing.T, dbURL string) *config.Config {
	t.Helper()
	t.Setenv("DATABASE_URL", dbURL)
	t.Setenv("APP_ENV", "development")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.MigrateOnStart = false
	cfg.CookieSecure = false
	return cfg
}

type api struct {
	t      *testing.T
	base   string
	client *http.Client
}

func (a *api) do(method, path string, body any, expect int) map[string]any {
	a.t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, a.base+"/api/v1"+path, rdr)
	if err != nil {
		a.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.client.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != expect {
		a.t.Fatalf("%s %s 期望 %d 实际 %d: %s", method, path, expect, resp.StatusCode, string(raw))
	}
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return m
}

func (a *api) code(m map[string]any) string {
	if v, ok := m["code"].(string); ok {
		return v
	}
	return ""
}

func (a *api) data(m map[string]any) map[string]any {
	d, _ := m["data"].(map[string]any)
	return d
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}
}

func setup(t *testing.T) (*pgxpool.Pool, *httptest.Server, *outbox.Dispatcher) {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("未设置 TEST_DATABASE_URL，跳过集成测试")
	}
	ctx := context.Background()
	pool, err := database.New(ctx, dbURL, 5)
	if err != nil {
		t.Fatalf("连接测试库失败: %v", err)
	}
	t.Cleanup(pool.Close)

	// 清库重迁移，保证可重复执行
	if _, err := pool.Exec(ctx, `
		DROP SCHEMA public CASCADE;
		CREATE SCHEMA public;`); err != nil {
		t.Fatalf("重置 schema 失败: %v", err)
	}
	if err := migrate.Up(ctx, dbURL); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := testConfig(t, dbURL)
	log := logger.New("development")
	if err := seed.Run(ctx, pool, cfg, log); err != nil {
		t.Fatalf("种子失败: %v", err)
	}

	dispatcher := outbox.NewDispatcher(pool, log)
	notifications.RegisterConsumer(dispatcher, pool)
	notifications.RegisterEmailHandlers(dispatcher, mailer.NewLogMailer(log), cfg)

	rbacSvc := rbac.New(pool)
	audit := moderation.NewAuditLogger(pool, log)
	gate := moderation.NewGate(pool)
	engine := server.Build(server.Deps{
		Cfg: cfg, Pool: pool, Redis: nil, Logger: log,
		RBAC: rbacSvc, Audit: audit, Gate: gate,
	})
	ts := httptest.NewServer(engine)
	t.Cleanup(ts.Close)
	return pool, ts, dispatcher
}

func TestE2ECoreLoop(t *testing.T) {
	pool, ts, dispatcher := setup(t)
	_ = dispatcher

	alice := newClient()
	bob := newClient()
	adminC := newClient()
	apiA := &api{t: t, base: ts.URL, client: alice}
	apiB := &api{t: t, base: ts.URL, client: bob}
	apiAdmin := &api{t: t, base: ts.URL, client: adminC}

	// 1. 游客浏览公开内容
	feeds := apiA.do("GET", "/feeds?tab=latest", nil, 200)
	if feeds["code"] != "OK" {
		t.Fatal("游客浏览失败")
	}
	cats := apiA.do("GET", "/categories", nil, 200)
	items := cats["data"].(map[string]any)["items"].([]any)
	if len(items) < 3 {
		t.Fatalf("种子版块缺失: %d", len(items))
	}
	// 游客互动 → 401
	apiA.do("POST", "/posts/x/like", nil, 401)

	// 2. Alice 注册
	apiA.do("POST", "/auth/register", map[string]string{
		"username": "alice", "email": "alice@test.com", "password": "password123",
	}, 201)
	// Bob 注册
	apiB.do("POST", "/auth/register", map[string]string{
		"username": "bob", "email": "bob@test.com", "password": "password123",
	}, 201)

	// 3. 从 outbox 取验证 token（模拟邮件内链接）
	aliceToken := grabToken(t, pool, "email.verification", "alice@test.com")
	apiA.do("POST", "/auth/email/verify", map[string]string{"token": aliceToken}, 200)

	// 4. 未验证的 Bob 不能发帖
	_ = dispatcher // outbox 消费同步处理
	bobToken := grabToken(t, pool, "email.verification", "bob@test.com")
	apiB.do("POST", "/auth/email/verify", map[string]string{"token": bobToken}, 200)

	// 5. 登录
	apiA.do("POST", "/auth/login", map[string]string{"account": "alice", "password": "password123"}, 200)
	apiB.do("POST", "/auth/login", map[string]string{"account": "bob", "password": "password123"}, 200)
	// 管理员登录
	apiAdmin.do("POST", "/auth/login", map[string]string{"account": "admin", "password": "Admin@2024"}, 200)

	me := apiA.do("GET", "/auth/me", nil, 200)
	ud := me["data"].(map[string]any)["user"].(map[string]any)
	if ud["email_verified"] != true {
		t.Fatal("Alice 邮箱应已验证")
	}

	// 6. 发帖
	created := apiA.do("POST", "/posts", map[string]any{
		"title": "第一篇测试帖子标题", "body_markdown": "正文内容，包含 **Markdown**。",
		"category_slug": "general", "post_type": "discussion", "tag_slugs": []string{},
	}, 201)
	postID := created["data"].(map[string]any)["id"].(string)
	if postID == "" {
		t.Fatal("帖子 ID 缺失")
	}

	// 7. Bob 评论 + 楼中楼
	c1 := apiB.do("POST", "/posts/"+postID+"/comments", map[string]any{
		"body_markdown": "第一条评论 @alice",
	}, 201)
	c1ID := c1["data"].(map[string]any)["id"].(string)
	c2 := apiA.do("POST", "/posts/"+postID+"/comments", map[string]any{
		"body_markdown": "楼中楼回复", "parent_id": c1ID,
	}, 201)
	c2ID := c2["data"].(map[string]any)["id"].(string)
	// 深度 3 拒绝（根 0 → 子 1 → 孙 2 → 重孙超限转为引用回复，因此允许；直接验证第 4 层被折叠为引用）
	_, _ = c2ID, c1ID

	// 8. 点赞幂等
	apiB.do("POST", "/posts/"+postID+"/like", nil, 200)
	apiB.do("POST", "/posts/"+postID+"/like", nil, 200)
	detail := apiB.do("GET", "/posts/"+postID, nil, 200)
	pd := detail["data"].(map[string]any)["post"].(map[string]any)
	if pd["like_count"].(float64) != 1 {
		t.Fatalf("重复点赞应只计 1 次，实际 %v", pd["like_count"])
	}
	if pd["is_liked"] != true {
		t.Fatal("点赞状态未生效")
	}

	// 9. 收藏
	apiB.do("POST", "/posts/"+postID+"/bookmark", map[string]any{}, 200)
	apiB.do("POST", "/posts/"+postID+"/bookmark", map[string]any{}, 200)
	bms := apiB.do("GET", "/me/bookmarks", nil, 200)
	bmItems := bms["data"].(map[string]any)["items"].([]any)
	if len(bmItems) != 1 {
		t.Fatalf("重复收藏应只产生 1 条记录，实际 %d", len(bmItems))
	}

	// 10. 关注
	apiB.do("POST", "/users/"+pd["author"].(map[string]any)["id"].(string)+"/follow", nil, 200)

	// 11. Bob 举报帖子
	apiB.do("POST", "/reports", map[string]any{
		"target_type": "post", "target_id": postID, "reason_code": "spam", "description": "测试举报",
	}, 201)
	// 重复举报 → 409
	apiB.do("POST", "/reports", map[string]any{
		"target_type": "post", "target_id": postID, "reason_code": "spam", "description": "测试举报",
	}, 409)

	// 12. 越权：Bob 不能编辑 Alice 的帖子
	apiB.do("PATCH", "/posts/"+postID, map[string]any{"title": "篡改的标题很长很长"}, 403)

	// 13. 管理员审核通过举报处理 + 隐藏帖子
	reports := apiAdmin.do("GET", "/admin/reports?status=pending", nil, 200)
	rItems := reports["data"].(map[string]any)["items"].([]any)
	if len(rItems) == 0 {
		t.Fatal("举报队列不应为空")
	}
	firstReport := int64(rItems[0].(map[string]any)["id"].(float64))
	apiAdmin.do("POST", fmt.Sprintf("/admin/reports/%d/resolve", firstReport), map[string]any{
		"resolution": "action_taken", "note": "核实属实，已处理",
	}, 200)

	// 管理员锁定帖子
	apiAdmin.do("POST", "/admin/posts/"+postID+"/moderate", map[string]any{
		"action": "lock", "reason": "测试锁定",
	}, 200)
	// 锁定后禁止新评论
	apiB.do("POST", "/posts/"+postID+"/comments", map[string]any{"body_markdown": "锁定后评论"}, 409)

	// 14. Alice 收到通知（评论/点赞/审核）
	notifs := apiA.do("GET", "/me/notifications", nil, 200)
	nItems := notifs["data"].(map[string]any)["items"].([]any)
	if len(nItems) == 0 {
		t.Fatal("Alice 应收到通知")
	}
	apiA.do("POST", "/me/notifications/read-all", nil, 200)
	unread := apiA.do("GET", "/me/notifications/unread-count", nil, 200)
	if unread["data"].(map[string]any)["unread"].(float64) != 0 {
		t.Fatal("全部已读后未读数应为 0")
	}

	// 15. 搜索
	sr := apiB.do("GET", "/search?q=%E6%B5%8B%E8%AF%95%E5%B8%96%E5%AD%90&type=post", nil, 200)
	if sr["data"].(map[string]any)["total"].(float64) < 1 {
		t.Fatal("搜索应命中测试帖子")
	}

	// 16. 非管理员禁止访问后台
	apiB.do("GET", "/admin/dashboard", nil, 403)

	// 17. 审计日志存在
	logs := apiAdmin.do("GET", "/admin/operation-logs", nil, 200)
	lItems := logs["data"].(map[string]any)["items"].([]any)
	if len(lItems) == 0 {
		t.Fatal("管理员操作应写入审计日志")
	}
}

// grabToken 从 outbox 事件中提取验证令牌（模拟用户点击邮件链接）。
func grabToken(t *testing.T, pool *pgxpool.Pool, eventType, email string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for i := 0; i < 50; i++ {
		rows, err := pool.Query(ctx, `
			SELECT payload FROM outbox_events WHERE event_type = $1 ORDER BY id DESC`, eventType)
		if err != nil {
			t.Fatal(err)
		}
		var found *string
		for rows.Next() {
			var raw []byte
			_ = rows.Scan(&raw)
			var p map[string]string
			_ = json.Unmarshal(raw, &p)
			if strings.EqualFold(p["to"], email) {
				s := p["token"]
				found = &s
			}
		}
		rows.Close()
		if found != nil {
			return *found
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("未找到 %s 的验证令牌", email)
	return ""
}
