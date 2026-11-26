package notifications

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	httpx "github.com/yanji-community/backend/internal/shared/httpx"
)

func TestSSEConnLimitPerUser(t *testing.T) {
	const uid int64 = 999001
	for i := 0; i < maxSSEConnsPerUser; i++ {
		if !acquireSSEConn(uid) {
			t.Fatalf("第 %d 条 SSE 连接应允许建立", i+1)
		}
	}
	if acquireSSEConn(uid) {
		t.Fatalf("超过单用户最大并发 SSE 连接数 (%d) 必须拒绝", maxSSEConnsPerUser)
	}
	for i := 0; i < maxSSEConnsPerUser; i++ {
		releaseSSEConn(uid)
	}
	if !acquireSSEConn(uid) {
		t.Fatal("释放全部 SSE 连接后应允许重新建立")
	}
	releaseSSEConn(uid)
}

func TestStreamHandlerBlocksBannedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(httpx.CtxAuthUser, &httpx.AuthUser{
			ID:              888,
			Status:          "active",
			EmailVerified:   true,
			ActiveSanctions: []string{"temp_ban"},
		})
		c.Next()
	})
	r.GET("/stream", StreamHandler(nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stream", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("temp_ban 用户建立 SSE 应被 403 拦截，实际 %d", w.Code)
	}
}
