package apperr

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RespondError 将错误写入响应并中止链（供中间件/Handler 复用）。
func RespondError(c *gin.Context, err error) {
	ae := From(err)
	rid := c.GetString("request_id")
	if ae.Status >= http.StatusInternalServerError {
		var method, path string
		if c.Request != nil {
			method, path = c.Request.Method, c.Request.URL.Path
		}
		slog.Error("internal error", "method", method, "path", path, "request_id", rid, "err", err)
	}
	c.AbortWithStatusJSON(ae.Status, gin.H{
		"code": ae.Code, "message": ae.Message, "data": nil,
		"request_id": rid,
	})
}

// RespondTooMany 限流响应。
func RespondTooMany(c *gin.Context) { RespondError(c, ErrRateLimited) }
