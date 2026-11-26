package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	httpx "github.com/yanji-community/backend/internal/shared/httpx"
)

// Redis 频道名（通知事件发布）。
const (
	notifyChannelPrefix = "notify:user:"
	maxSSEConnsPerUser  = 5
)

var (
	sseConnMu    sync.Mutex
	sseConnCount = map[int64]int{}
)

func acquireSSEConn(userID int64) bool {
	sseConnMu.Lock()
	defer sseConnMu.Unlock()
	if sseConnCount[userID] >= maxSSEConnsPerUser {
		return false
	}
	sseConnCount[userID]++
	return true
}

func releaseSSEConn(userID int64) {
	sseConnMu.Lock()
	defer sseConnMu.Unlock()
	if sseConnCount[userID] <= 1 {
		delete(sseConnCount, userID)
		return
	}
	sseConnCount[userID]--
}

// NotifyChannel 用户通知频道。
func NotifyChannel(userID int64) string {
	return fmt.Sprintf("%s%d", notifyChannelPrefix, userID)
}

// SSEEvent 推送给前端的事件负载。
type SSEEvent struct {
	Type  string `json:"type"` // 通知类型
	Title string `json:"title"`
	Actor string `json:"actor,omitempty"`
}

// PublishNotification 通知插入成功后向 Redis 发布实时事件（消费者内调用，失败不阻断）。
func PublishNotification(ctx context.Context, rdb redis.UniversalClient, recipientID int64, ev SSEEvent) {
	if rdb == nil {
		return
	}
	if v := reflect.ValueOf(rdb); v.Kind() == reflect.Ptr && v.IsNil() {
		return
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_ = rdb.Publish(ctx, NotifyChannel(recipientID), b).Err()
}

// StreamHandler GET /me/notifications/stream —— SSE 实时通知。
// 断线后前端通过重新拉取 /me/notifications（补拉）保证不丢消息：SSE 仅做"提醒"，
// 通知本体始终以数据库为准（先落库后推送）。
func StreamHandler(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := httpx.CurrentUser(c)
		if u == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if u.IsBanned() {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if rdb == nil {
			c.String(http.StatusServiceUnavailable, "stream unavailable")
			return
		}
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.String(http.StatusInternalServerError, "streaming unsupported")
			return
		}
		if !acquireSSEConn(u.ID) {
			c.String(http.StatusTooManyRequests, "too many active streams")
			return
		}
		defer releaseSSEConn(u.ID)

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-store, no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		ctx := c.Request.Context()
		sub := rdb.Subscribe(ctx, NotifyChannel(u.ID))
		defer sub.Close()

		// 初始连接确认
		fmt.Fprint(c.Writer, ": connected\n\n")
		flusher.Flush()

		msgs := sub.Channel()
		heartbeat := time.NewTicker(25 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeat.C:
				if _, err := fmt.Fprint(c.Writer, ": ping\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				// 防止消息内意外换行破坏 SSE 帧边界
				safePayload := strings.ReplaceAll(strings.ReplaceAll(msg.Payload, "\r", ""), "\n", " ")
				if _, err := fmt.Fprintf(c.Writer, "event: notification\ndata: %s\n\n", safePayload); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}
