package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSanctionChecks(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour)
	past := time.Now().UTC().Add(-time.Hour)

	cases := []struct {
		name        string
		u           *AuthUser
		wantMuted   bool
		wantCanPost bool
		wantCanCmt  bool
	}{
		{"无处罚", &AuthUser{}, false, true, true},
		{"警告", &AuthUser{SanctionType: "warn"}, false, true, true},
		{"禁言", &AuthUser{SanctionType: "mute"}, true, true, false},
		{"禁发帖", &AuthUser{SanctionType: "post_ban"}, true, false, true},
		{"临时封禁生效中", &AuthUser{SanctionType: "temp_ban", SanctionEnds: &future}, true, false, false},
		{"临时封禁已过期", &AuthUser{SanctionType: "temp_ban", SanctionEnds: &past}, false, true, true},
		{"永久封禁", &AuthUser{SanctionType: "perma_ban"}, true, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.u.Muted(); got != c.wantMuted {
				t.Fatalf("Muted() = %v, want %v", got, c.wantMuted)
			}
			if got := c.u.CanPost(); got != c.wantCanPost {
				t.Fatalf("CanPost() = %v, want %v", got, c.wantCanPost)
			}
			if got := c.u.CanComment(); got != c.wantCanCmt {
				t.Fatalf("CanComment() = %v, want %v", got, c.wantCanCmt)
			}
		})
	}
}

func TestVerifiedNilSafe(t *testing.T) {
	var u *AuthUser
	if u.Verified() {
		t.Fatal("nil 用户不应为已验证")
	}
}

func TestOriginCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginCheck([]string{"http://localhost:3000/"}))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	cases := []struct {
		name   string
		method string
		host   string
		origin string
		want   int
	}{
		{"GET 无 Origin 放行", http.MethodGet, "localhost:8080", "", http.StatusOK},
		{"POST 非浏览器无 Origin 放行", http.MethodPost, "localhost:8080", "", http.StatusOK},
		{"POST 白名单 Origin 放行", http.MethodPost, "localhost:8080", "http://localhost:3000", http.StatusOK},
		{"POST 同 Host 反代放行", http.MethodPost, "192.168.1.10:3000", "http://192.168.1.10:3000", http.StatusOK},
		{"POST 同 Host 大小写不敏感放行", http.MethodPost, "Forum.Example.COM:8443", "https://forum.example.com:8443", http.StatusOK},
		{"POST 不同端口 Origin 拒绝", http.MethodPost, "192.168.1.10:3000", "http://192.168.1.10:8080", http.StatusForbidden},
		{"POST 外部非法 Origin 拒绝", http.MethodPost, "localhost:8080", "https://evil.example.com", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/ok", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
