// Package apperr 定义统一业务错误：HTTP 状态码 + 稳定错误码 + 用户可读消息。
// 前端按 Code 处理业务分支，不解析 message。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError 业务错误。
type AppError struct {
	Status  int    // HTTP 状态码
	Code    string // 稳定错误码
	Message string // 用户可读消息（中文）
	cause   error
}

// Error 实现 error。
func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.cause)
	}
	return e.Code + ": " + e.Message
}

// Unwrap 暴露底层原因（日志用）。
func (e *AppError) Unwrap() error { return e.cause }

// New 构造业务错误。
func New(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// Wrap 在业务错误上附加底层原因。
func Wrap(err error, status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message, cause: err}
}

// From 将任意错误转换为 AppError；未知错误归一为 INTERNAL_ERROR（不泄露内部细节）。
func From(err error) *AppError {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae
	}
	return Internal(err)
}

// ---- 常用工厂 ----

func BadRequest(code, msg string) *AppError { return New(http.StatusBadRequest, code, msg) }
func Unauthorized(msg string) *AppError     { return New(http.StatusUnauthorized, "UNAUTHENTICATED", msg) }
func Forbidden(msg string) *AppError        { return New(http.StatusForbidden, "FORBIDDEN", msg) }
func NotFound(msg string) *AppError         { return New(http.StatusNotFound, "NOT_FOUND", msg) }
func Conflict(code, msg string) *AppError   { return New(http.StatusConflict, code, msg) }
func TooMany(msg string) *AppError          { return New(http.StatusTooManyRequests, "RATE_LIMITED", msg) }
func Internal(cause error) *AppError {
	return Wrap(cause, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误，请稍后重试")
}

// ---- 常用错误码 ----

var (
	ErrValidation       = BadRequest("VALIDATION_ERROR", "请求参数不合法")
	ErrUnauthenticated  = Unauthorized("请先登录")
	ErrForbidden        = Forbidden("没有权限执行该操作")
	ErrNotFound         = NotFound("资源不存在或已被删除")
	ErrEmailNotVerified = New(http.StatusForbidden, "EMAIL_NOT_VERIFIED", "请先完成邮箱验证")
	ErrAccountSuspended = New(http.StatusForbidden, "ACCOUNT_SUSPENDED", "账号已被封禁，暂无法执行该操作")
	ErrSanctioned       = New(http.StatusForbidden, "SANCTIONED", "当前账号处于处罚期，无法执行该操作")
	ErrPostLocked       = Conflict("POST_LOCKED", "该帖子已锁定，无法新增评论")
	ErrPendingReview    = New(http.StatusAccepted, "CONTENT_PENDING_REVIEW", "内容已提交，等待审核")
	ErrRateLimited      = TooMany("操作过于频繁，请稍后再试")
	ErrEmailTaken       = Conflict("CONFLICT", "用户名或邮箱已被使用")
	ErrContentBlocked   = BadRequest("CONTENT_BLOCKED", "内容包含违规信息，无法发布")
)
