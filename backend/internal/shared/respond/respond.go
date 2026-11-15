// Package respond 统一 HTTP 响应结构 {code, message, data, request_id}。
package respond

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// PageData 列表分页包装。
type PageData struct {
	Items    any   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
	HasMore  bool  `json:"has_more"`
}

type envelope struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
	RequestID string `json:"request_id"`
}

func requestID(c *gin.Context) string {
	if v := c.GetString("request_id"); v != "" {
		return v
	}
	return c.GetHeader("X-Request-Id")
}

// OK 成功响应。
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Code: "OK", Message: "success", Data: data, RequestID: requestID(c)})
}

// Created 创建成功响应（201）。
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Code: "OK", Message: "created", Data: data, RequestID: requestID(c)})
}

// Accepted 请求已受理（如进入审核队列）。
func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, envelope{Code: "CONTENT_PENDING_REVIEW", Message: "内容已提交，等待审核", Data: data, RequestID: requestID(c)})
}

// Page 列表响应。
func Page(c *gin.Context, items any, page, pageSize int, total int64) {
	hasMore := int64(page*pageSize) < total
	OK(c, PageData{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: hasMore})
}

// Fail 错误响应：业务错误透出稳定 Code，未知错误归一 INTERNAL_ERROR。
// 服务端错误的底层原因由调用方或日志中间件记录，不返回给客户端。
func Fail(c *gin.Context, err error) {
	ae := apperr.From(err)
	c.AbortWithStatusJSON(ae.Status, envelope{Code: ae.Code, Message: ae.Message, Data: nil, RequestID: requestID(c)})
}
