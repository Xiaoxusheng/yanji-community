package apperr

import "github.com/gin-gonic/gin"

// RespondError 将错误写入响应并中止链（供中间件/Handler 复用）。
func RespondError(c *gin.Context, err error) {
	ae := From(err)
	c.AbortWithStatusJSON(ae.Status, gin.H{
		"code": ae.Code, "message": ae.Message, "data": nil,
		"request_id": c.GetString("request_id"),
	})
}

// RespondTooMany 限流响应。
func RespondTooMany(c *gin.Context) { RespondError(c, ErrRateLimited) }
