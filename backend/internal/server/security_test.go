package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/modules/uploads"
	"github.com/yanji-community/backend/internal/platform/config"
	"github.com/yanji-community/backend/internal/platform/storage"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/rbac"
)

func buildTestEngine(t *testing.T, uploadDir string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ls, err := storage.NewLocalStorage(uploadDir, "/files")
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Env:              "test",
		CORSAllowOrigins: []string{"http://localhost:3000"},
		RequestIDHeader:  "X-Request-Id",
		UploadMaxBytes:   2 << 20,
		SessionTTL:       time.Hour,
		RateLimitAuth:    5,
		RateLimitWrite:   30,
		RateLimitRead:    60,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return Build(Deps{
		Cfg:    cfg,
		Logger: logger,
		RBAC:   rbac.New(nil),
		Store:  ls,
	})
}

func TestServerSecurityHeadersAndCSRF(t *testing.T) {
	r := buildTestEngine(t, t.TempDir())

	// 1. 健康检查带全套安全响应头，且恶意 X-Request-Id 被清洗
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "evil\r\nInjected: 1")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/healthz 期望 200，实际 %d", w.Code)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("安全响应头缺失: %+v", w.Header())
	}
	if rid := w.Header().Get("X-Request-Id"); !strings.HasPrefix(rid, "req_") {
		t.Fatalf("恶意 X-Request-Id 未被清洗: %q", rid)
	}

	// 2. 跨站 POST 攻击（非法 Origin / Sec-Fetch-Site: cross-site）在进入业务前即被 403 拦截
	for _, path := range []string{"/api/v1/auth/login", "/api/v1/posts", "/api/v1/admin/users/1/roles"} {
		w1 := httptest.NewRecorder()
		req1 := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req1.Header.Set("Origin", "https://attacker.example")
		r.ServeHTTP(w1, req1)
		if w1.Code != http.StatusForbidden {
			t.Fatalf("跨站 Origin POST %s 期望 403，实际 %d", path, w1.Code)
		}

		w2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req2.Header.Set("Sec-Fetch-Site", "cross-site")
		r.ServeHTTP(w2, req2)
		if w2.Code != http.StatusForbidden {
			t.Fatalf("Sec-Fetch-Site=cross-site POST %s 期望 403，实际 %d", path, w2.Code)
		}
	}
}

func TestServerUnauthenticatedAccessBlocked(t *testing.T) {
	r := buildTestEngine(t, t.TempDir())

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/me/profile"},
		{http.MethodGet, "/api/v1/me/notifications"},
		{http.MethodGet, "/api/v1/me/notifications/stream"},
		{http.MethodGet, "/api/v1/me/conversations"},
		{http.MethodPost, "/api/v1/conversations"},
		{http.MethodPost, "/api/v1/posts"},
		{http.MethodPost, "/api/v1/uploads"},
		{http.MethodGet, "/api/v1/admin/dashboard"},
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodGet, "/api/v1/admin/appeals"},
		{http.MethodPost, "/api/v1/admin/users/1/roles"},
		{http.MethodPost, "/api/v1/admin/users/1/sanctions"},
		{http.MethodGet, "/api/v1/admin/operation-logs"},
	}

	for _, tc := range protectedRoutes {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("未认证访问 %s %s 期望 401，实际 %d (%s)", tc.method, tc.path, w.Code, w.Body.String())
		}
	}

	// /api/v1/auth/me 游客态虽返 200，但必须禁止缓存
	wMe := httptest.NewRecorder()
	r.ServeHTTP(wMe, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if !strings.Contains(wMe.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("/api/v1/auth/me 必须返回 Cache-Control: no-store，实际 %q", wMe.Header().Get("Cache-Control"))
	}
}

func TestServerLocalFilesSecurity(t *testing.T) {
	dir := t.TempDir()
	ls, err := storage.NewLocalStorage(dir, "/files")
	if err != nil {
		t.Fatal(err)
	}
	// 写入一张合法 PNG
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	if err := ls.Put(context.Background(), "uploads/2026/01/safe.png", bytes.NewReader(pngBytes)); err != nil {
		t.Fatal(err)
	}

	r := buildTestEngine(t, dir)

	// 1. 合法图片带严格 sandbox CSP + nosniff
	wOk := httptest.NewRecorder()
	r.ServeHTTP(wOk, httptest.NewRequest(http.MethodGet, "/files/uploads/2026/01/safe.png", nil))
	if wOk.Code != http.StatusOK {
		t.Fatalf("合法静态图片期望 200，实际 %d", wOk.Code)
	}
	if !strings.Contains(wOk.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("/files/ 响应缺少 sandbox CSP: %q", wOk.Header().Get("Content-Security-Policy"))
	}
	if wOk.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("Content-Type 期望 image/png，实际 %q", wOk.Header().Get("Content-Type"))
	}

	// 2. 路径穿越与非白名单扩展名全部拒绝（404）
	for _, badPath := range []string{
		"/files/../../etc/passwd",
		"/files/uploads/../../secret.png",
		"/files/uploads/2026/01/evil.svg",
		"/files/uploads/2026/01/evil.html",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, badPath, nil))
		if w.Code != http.StatusNotFound && w.Code != http.StatusMovedPermanently {
			t.Fatalf("危险文件路径 %s 不应放行，实际状态码 %d", badPath, w.Code)
		}
	}
}

func TestServerAuthRateLimitEnforcedWithoutRedis(t *testing.T) {
	r := buildTestEngine(t, t.TempDir())
	// RateLimitAuth=5；前 5 次后第 6 次必须返回 429
	for i := 1; i <= 6; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.RemoteAddr = "203.0.113.77:5555"
		r.ServeHTTP(w, req)
		if i <= 5 && w.Code != http.StatusOK {
			t.Fatalf("第 %d 次 auth 请求期望 200，实际 %d", i, w.Code)
		}
		if i == 6 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("第 6 次超限 auth 请求期望 429，实际 %d", w.Code)
		}
	}
}

func TestUploadMaxBytesReaderBlocksOversizeDoS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ls, err := storage.NewLocalStorage(t.TempDir(), "/files")
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// 限制最大 1KB，MaxBytesReader 允许 1KB + 1MB multipart 头，构造 2MB 载荷应立即截断
	svc := uploads.New(ls, nil, logger, 1024)
	h := uploads.NewHandler(svc)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(httpx.CtxAuthUser, &httpx.AuthUser{ID: 1, Status: "active", EmailVerified: true})
		c.Next()
	})
	h.Register(r.Group("/api/v1"))

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "big.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	_, _ = part.Write(bytes.Repeat([]byte{'A'}, 2<<20))
	_ = mw.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("超大上传请求期望 400 被 MaxBytesReader 拦截，实际 %d (%s)", w.Code, w.Body.String())
	}
}
