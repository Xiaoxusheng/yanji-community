package httpx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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

const maxIdempotencyBodyBytes = 256 << 10 // 限制缓存响应体最大 256KB

// idempotencyBodyWriter 捕获响应体以缓存重放。
type idempotencyBodyWriter struct {
	gin.ResponseWriter
	body     *bytes.Buffer
	overflow bool
}

func (w *idempotencyBodyWriter) Write(b []byte) (int, error) {
	if !w.overflow {
		if w.body.Len()+len(b) <= maxIdempotencyBodyBytes {
			w.body.Write(b)
		} else {
			w.overflow = true
		}
	}
	return w.ResponseWriter.Write(b)
}

func idempotencyDigest(userID int64, method, path, key string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s|%s", userID, method, path, key)))
	return hex.EncodeToString(sum[:16])
}

// Idempotency 对带 Idempotency-Key 头的已认证写请求做去重与响应重放。
// 必须挂在 SessionAuth 之后（依赖认证用户）；缓存键按 userID + method + path + key 哈希隔离。
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
		digest := idempotencyDigest(u.ID, c.Request.Method, c.Request.URL.Path, key)
		cacheKey := "idem:resp:" + digest
		lockKey := "idem:lock:" + digest

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
		defer func() {
			_ = rdb.Del(context.Background(), lockKey).Err()
		}()

		bw := &idempotencyBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = bw
		c.Next()
		// 5xx 或超大响应不缓存（允许客户端重试）；4xx/2xx 缓存重放
		if !bw.overflow && bw.Status() > 0 && bw.Status() < 500 {
			if b, err := json.Marshal(storedResponse{Status: bw.Status(), Body: bw.body.String()}); err == nil {
				_ = rdb.Set(ctx, cacheKey, b, ttl).Err()
			}
		}
	}
}
