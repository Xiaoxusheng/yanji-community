package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type stubPermChecker struct {
	anyPerms    map[string]bool
	globalPerms map[string]bool
}

func (s stubPermChecker) HasPermission(_ context.Context, _ int64, code string) (bool, error) {
	return s.anyPerms[code], nil
}

func (s stubPermChecker) HasGlobalPermission(_ context.Context, _ int64, code string) (bool, error) {
	return s.globalPerms[code], nil
}

func TestAuthUserSanctionAggregation(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	past := time.Now().UTC().Add(-time.Hour)

	// 多处罚叠加：后发 warn 不得覆盖正在生效的 mute 与 post_ban
	multi := &AuthUser{
		ID:              1,
		Status:          "active",
		EmailVerified:   true,
		SanctionType:    "warn",
		ActiveSanctions: []string{"warn", "mute", "post_ban"},
	}
	if multi.IsBanned() {
		t.Fatal("mute+post_ban 不应被误判为封禁")
	}
	if multi.CanComment() {
		t.Fatal("存在生效 mute 时必须禁止评论/私信")
	}
	if multi.CanPost() {
		t.Fatal("存在生效 post_ban 时必须禁止发帖")
	}

	// temp_ban 必须视为封禁，阻断全部敏感操作
	tempBan := &AuthUser{
		ID:              2,
		Status:          "active",
		EmailVerified:   true,
		SanctionType:    "temp_ban",
		SanctionEnds:    &future,
		ActiveSanctions: []string{"temp_ban"},
	}
	if !tempBan.IsBanned() || tempBan.CanPost() || tempBan.CanComment() {
		t.Fatal("temp_ban 生效期间必须视为封禁并禁止所有互动")
	}

	// 已过期处罚不得误拦正常用户
	expired := &AuthUser{
		ID:            3,
		Status:        "active",
		EmailVerified: true,
		SanctionType:  "temp_ban",
		SanctionEnds:  &past,
	}
	if expired.IsBanned() || !expired.CanPost() || !expired.CanComment() {
		t.Fatal("已过期 temp_ban 不应继续限制用户")
	}
}

func TestMiddlewareSanctionAndGlobalPermEnforcement(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)

	// 1. temp_ban 用户即使带有效会话也被 RequireAuth / RequireVerified / RequirePermission 拒绝
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(CtxAuthUser, &AuthUser{
			ID:              10,
			Status:          "active",
			EmailVerified:   true,
			SanctionType:    "temp_ban",
			SanctionEnds:    &future,
			ActiveSanctions: []string{"temp_ban"},
		})
		c.Next()
	})
	pc := stubPermChecker{
		anyPerms:    map[string]bool{"post.moderate": true, "user.sanction": true},
		globalPerms: map[string]bool{"user.sanction": true},
	}
	r.GET("/auth", RequireAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/verified", RequireVerified(), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/perm", RequirePermission(pc, "post.moderate"), func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/gperm", RequireGlobalPermission(pc, "user.sanction"), func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, path := range []string{"/auth", "/verified", "/perm", "/gperm"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("temp_ban 访问 %s 期望 403，实际 %d (%s)", path, w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "ACCOUNT_SUSPENDED") {
			t.Fatalf("temp_ban 访问 %s 期望错误码 ACCOUNT_SUSPENDED，实际 %s", path, w.Body.String())
		}
	}

	// 2. 单版块版主拥有 scoped 权限但无 global 权限，必须被 RequireGlobalPermission 阻断
	rMod := gin.New()
	rMod.Use(func(c *gin.Context) {
		c.Set(CtxAuthUser, &AuthUser{ID: 20, Status: "active", EmailVerified: true})
		c.Next()
	})
	modPC := stubPermChecker{
		anyPerms:    map[string]bool{"report.handle": true, "post.moderate": true},
		globalPerms: map[string]bool{},
	}
	rMod.GET("/scoped", RequirePermission(modPC, "report.handle"), func(c *gin.Context) { c.Status(http.StatusOK) })
	rMod.GET("/global", RequireGlobalPermission(modPC, "report.handle"), func(c *gin.Context) { c.Status(http.StatusOK) })

	wScoped := httptest.NewRecorder()
	rMod.ServeHTTP(wScoped, httptest.NewRequest(http.MethodGet, "/scoped", nil))
	if wScoped.Code != http.StatusOK {
		t.Fatalf("版主访问 scoped 接口期望 200，实际 %d", wScoped.Code)
	}

	wGlobal := httptest.NewRecorder()
	rMod.ServeHTTP(wGlobal, httptest.NewRequest(http.MethodGet, "/global", nil))
	if wGlobal.Code != http.StatusForbidden {
		t.Fatalf("单版块版主越权访问 global 接口期望 403，实际 %d", wGlobal.Code)
	}
}

func TestOriginCheckCSRFDefense(t *testing.T) {
	r := gin.New()
	r.Use(OriginCheck([]string{"https://community.example.com"}))
	r.GET("/read", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/write", func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := []struct {
		name           string
		method         string
		path           string
		host           string
		origin         string
		secFetchSite   string
		forwardedProto string
		wantStatus     int
	}{
		{"跨站恶意 Origin 被拦截", http.MethodPost, "/write", "community.example.com", "https://evil.example", "", "", http.StatusForbidden},
		{"Origin null 被拦截", http.MethodPost, "/write", "community.example.com", "null", "", "", http.StatusForbidden},
		{"无 Origin 但 Sec-Fetch-Site cross-site 被拦截", http.MethodPost, "/write", "community.example.com", "", "cross-site", "", http.StatusForbidden},
		{"非法协议伪造同 Host 被拦截", http.MethodPost, "/write", "community.example.com", "javascript://community.example.com", "", "", http.StatusForbidden},
		{"X-Forwarded-Proto 与 Origin 协议不一致被拦截", http.MethodPost, "/write", "10.0.0.5:8080", "http://10.0.0.5:8080", "", "https", http.StatusForbidden},
		{"白名单 Origin 正常放行", http.MethodPost, "/write", "api.example.com", "https://community.example.com", "same-site", "https", http.StatusOK},
		{"同源反代且协议一致正常放行", http.MethodPost, "/write", "10.0.0.5:8080", "https://10.0.0.5:8080", "same-origin", "https", http.StatusOK},
		{"安全方法 GET 不受 OriginCheck 阻断", http.MethodGet, "/read", "community.example.com", "https://evil.example", "cross-site", "", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.secFetchSite != "" {
				req.Header.Set("Sec-Fetch-Site", tc.secFetchSite)
			}
			if tc.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}
			r.ServeHTTP(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("期望状态码 %d，实际 %d (%s)", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestRequestIDHeaderInjectionDefense(t *testing.T) {
	r := gin.New()
	r.Use(RequestID("X-Request-Id"))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(CtxRequestID))
	})

	// 恶意带换行/空格/特殊字符的请求头必须被拒绝并替换
	badInputs := []string{
		"bad\r\nX-Injected: yes",
		"bad value with spaces",
		"<script>alert(1)</script>",
		strings.Repeat("a", 80),
	}
	for _, bad := range badInputs {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("X-Request-Id", bad)
		r.ServeHTTP(w, req)
		got := w.Header().Get("X-Request-Id")
		if got == bad || !strings.HasPrefix(got, "req_") {
			t.Fatalf("恶意 Request-ID 未被清洗替换: input=%q got=%q", bad, got)
		}
	}

	// 合法 Request-ID 正常透传
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-Id", "trace-abc_123.ok")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("X-Request-Id"); got != "trace-abc_123.ok" {
		t.Fatalf("合法 Request-ID 应透传，实际 %q", got)
	}
}

func TestRateLimitFallbackWithoutRedis(t *testing.T) {
	r := gin.New()
	r.Use(RateLimit(nil, "test_bucket", 3))
	r.GET("/limited", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 1; i <= 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "198.51.100.10:12345"
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次请求应放行，实际 %d", i, w.Code)
		}
	}

	// 第 4 次必须被内存兜底限流拦截（杜绝 Redis 缺失时 fail-open）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("第 4 次超限请求期望 429，实际 %d", w.Code)
	}

	// 另一 IP 独立计数不受影响
	wOther := httptest.NewRecorder()
	reqOther := httptest.NewRequest(http.MethodGet, "/limited", nil)
	reqOther.RemoteAddr = "198.51.100.11:54321"
	r.ServeHTTP(wOther, reqOther)
	if wOther.Code != http.StatusOK {
		t.Fatalf("其他 IP 期望 200，实际 %d", wOther.Code)
	}
}

func TestIdempotencyDigestIsolation(t *testing.T) {
	d1 := idempotencyDigest(1, http.MethodPost, "/api/v1/posts", "same-key")
	d2 := idempotencyDigest(2, http.MethodPost, "/api/v1/posts", "same-key")
	d3 := idempotencyDigest(1, http.MethodPatch, "/api/v1/posts", "same-key")
	d4 := idempotencyDigest(1, http.MethodPost, "/api/v1/posts/abc/publish", "same-key")
	d5 := idempotencyDigest(1, http.MethodPost, "/api/v1/posts", "same-key:lock")

	seen := map[string]string{
		d1: "u1-post",
		d2: "u2-post",
		d3: "u1-patch",
		d4: "u1-publish",
		d5: "u1-lock-suffix",
	}
	if len(seen) != 5 {
		t.Fatalf("幂等键摘要发生跨用户/跨方法/跨路径碰撞: %v", seen)
	}
}

func TestSecureHeadersAndNoStore(t *testing.T) {
	r := gin.New()
	r.Use(SecureHeaders(), NoStore())
	r.GET("/secret", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/secret", nil))

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
		"Pragma":                 "no-cache",
	}
	for k, v := range expected {
		if got := w.Header().Get(k); got != v {
			t.Fatalf("响应头 %s 期望 %q，实际 %q", k, v, got)
		}
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("期望 Cache-Control 包含 no-store，实际 %q", w.Header().Get("Cache-Control"))
	}
}
