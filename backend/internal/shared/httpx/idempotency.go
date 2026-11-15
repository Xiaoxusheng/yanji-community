package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// ---- Idempotency-Key 请求去重（PRD：创建/编辑帖子防重复提交） ----
// 首次请求：执行并缓存 JSON 响应（8h）；进行中的重复请求：409；已完成的重复请求：重放缓存响应。

type storedResponse struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// idempotencyBodyWriter 捕获响应体以缓存重放。
type idempotencyBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *idempotencyBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Idempotency 对带 Idempotency-Key 头的已认证写请求做去重与响应重放。
// 必须挂在 SessionAuth 之后（依赖认证用户）。
func Idempotency(rdb *redis.Client, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" || len(key) > 128 || rdb == nil || c.Request.Method == http.MethodGet {
			c.Next()
			return
		}
		u := CurrentUser(c)
		if u == nil {
			c.Next()
			return
		}
		ctx := c.Request.Context()
		cacheKey := fmt.Sprintf("idem:%d:%s", u.ID, key)
		lockKey := cacheKey + ":lock"

		// 已完成 → 重放
		if cached, err := rdb.Get(ctx, cacheKey).Bytes(); err == nil {
			var stored storedResponse
			if json.Unmarshal(cached, &stored) == nil && stored.Status > 0 {
				c.Header("Idempotency-Replayed", "true")
				c.Data(stored.Status, "application/json; charset=utf-8", []byte(stored.Body))
				c.Abort()
				return
			}
		}
		// 进行中 → 冲突
		acquired, err := rdb.SetNX(ctx, lockKey, "1", 30*time.Second).Result()
		if err == nil && !acquired {
			apperr.RespondError(c, apperr.Conflict("CONFLICT", "相同请求正在处理或刚提交，请勿重复点击"))
			return
		}
		bw := &idempotencyBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = bw
		c.Next()
		// 5xx 不缓存（允许客户端重试）；4xx/2xx 缓存重放
		if bw.Status() > 0 && bw.Status() < 500 {
			if b, err := json.Marshal(storedResponse{Status: bw.Status(), Body: bw.body.String()}); err == nil {
				_ = rdb.Set(ctx, cacheKey, b, ttl).Err()
			}
		}
		_ = rdb.Del(context.Background(), lockKey).Err()
	}
}
