package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yanji-community/backend/internal/platform/config"
)

// TestBuildRoutesNoConflict 验证全部模块路由装配无 Gin 冲突或 panic，且关键前台与后台路由路径准确。
func TestBuildRoutesNoConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := Build(Deps{
		Cfg: &config.Config{
			Env:             "test",
			RequestIDHeader: "X-Request-Id",
			SessionTTL:      24 * time.Hour,
			PublicURL:       "http://localhost:3000",
			UploadMaxBytes:  10 << 20,
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	routes := engine.Routes()
	if len(routes) == 0 {
		t.Fatal("路由表不应为空")
	}
	want := map[string]bool{
		"GET /api/v1/auth/captcha":                      false,
		"POST /api/v1/auth/password/change":             false,
		"POST /api/v1/posts/:id/view":                   false,
		"GET /api/v1/badges":                            false,
		"GET /api/v1/users/:username/badges":            false,
		"GET /api/v1/activities":                        false,
		"POST /api/v1/activities/:id/join":              false,
		"GET /api/v1/me/notifications/stream":           false,
		"POST /api/v1/admin/posts/batch-moderate":       false,
		"GET /api/v1/admin/categories":                  false,
		"GET /api/v1/admin/tags":                        false,
		"GET /api/v1/admin/reports":                     false,
		"GET /api/v1/admin/appeals":                     false,
		"GET /api/v1/admin/activities":                  false,
		"GET /api/v1/admin/activities/:id/participants": false,
	}
	for _, r := range routes {
		key := r.Method + " " + r.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for k, found := range want {
		if !found {
			t.Fatalf("缺少期望路由: %s", k)
		}
	}
}

// TestUnauthenticatedGuards 验证需登录路由与验证码端点不会触发 nil 指针 500 崩溃且安全响应头完整。
func TestUnauthenticatedGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := Build(Deps{
		Cfg: &config.Config{
			Env:             "test",
			RequestIDHeader: "X-Request-Id",
			SessionTTL:      24 * time.Hour,
			PublicURL:       "http://localhost:3000",
			UploadMaxBytes:  10 << 20,
		},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	for _, tc := range []struct {
		method, path string
		wantStatus   int
	}{
		{http.MethodGet, "/api/v1/auth/captcha", http.StatusOK},
		{http.MethodGet, "/api/v1/me/profile", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/me/following", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/posts/nonexistent/revisions", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/auth/password/change", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/reports", http.StatusUnauthorized},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != tc.wantStatus {
			t.Fatalf("%s %s 状态码 = %d，期望 %d", tc.method, tc.path, w.Code, tc.wantStatus)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s %s 缺少 X-Content-Type-Options", tc.method, tc.path)
		}
	}
}
