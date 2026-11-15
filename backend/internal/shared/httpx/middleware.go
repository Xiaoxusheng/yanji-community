// Package httpx 提供 Gin 全局中间件：请求 ID、恢复、CORS、安全头、限流、认证与 RBAC。
// 权限判断只在服务端执行；前端隐藏按钮仅是体验优化。
package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// CtxKeys gin context 键名。
const (
	CtxRequestID = "request_id"
	CtxAuthUser  = "auth_user"
)

// AuthUser 认证后的用户上下文（由 auth 模块装配）。
type AuthUser struct {
	ID            int64
	PublicID      string
	Username      string
	Email         string
	EmailVerified bool
	Status        string
	SessionID     int64
	SanctionType  string     // 当前生效处罚（空表示无）
	SanctionEnds  *time.Time // 临时处罚结束时间
}

// Verified 判断账号是否已完成邮箱验证。
func (u *AuthUser) Verified() bool { return u != nil && u.EmailVerified }

// restrictiveSanctions 限制互动的处罚类型（warn 仅提醒，不限制）。
var restrictiveSanctions = map[string]bool{"mute": true, "post_ban": true, "temp_ban": true, "perma_ban": true}

// Muted 判断账号是否处于限制互动的处罚期。
func (u *AuthUser) Muted() bool {
	return restrictiveSanctions[u.sanctionActive()]
}

// sanctionActive 返回仍在生效期的处罚类型（惰性过期）。
func (u *AuthUser) sanctionActive() string {
	if u == nil || u.SanctionType == "" {
		return ""
	}
	if u.SanctionEnds != nil && u.SanctionEnds.Before(time.Now().UTC()) {
		return ""
	}
	return u.SanctionType
}

// CanPost 是否可发帖（post_ban/temp_ban/perma_ban 期间禁止）。
func (u *AuthUser) CanPost() bool {
	switch u.sanctionActive() {
	case "post_ban", "temp_ban", "perma_ban":
		return false
	}
	return true
}

// CanComment 是否可评论（mute/temp_ban/perma_ban 期间禁止）。
func (u *AuthUser) CanComment() bool {
	switch u.sanctionActive() {
	case "mute", "temp_ban", "perma_ban":
		return false
	}
	return true
}

// SessionValidator 由 auth 模块实现：校验会话 Cookie 并返回用户上下文。
type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (*AuthUser, error)
}

// PermissionChecker 由 RBAC 服务实现。
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID int64, code string) (bool, error)
}

// ---- 基础中间件 ----

// RequestID 为每个请求生成/透传请求 ID。
func RequestID(header string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(header)
		if rid == "" {
			rid = fmt.Sprintf("req_%d_%04d", time.Now().UnixNano(), time.Now().Nanosecond()%10000)
		}
		c.Set(CtxRequestID, rid)
		c.Header(header, rid)
		c.Next()
	}
}

// Recovery 捕获 panic，记录日志并返回统一错误。
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered", "request_id", c.GetString(CtxRequestID),
					"path", c.Request.URL.Path, "panic", r)
				c.AbortWithStatusJSON(http.StatusInternalServerError,
					map[string]any{"code": "INTERNAL_ERROR", "message": "服务内部错误", "data": nil,
						"request_id": c.GetString(CtxRequestID)})
			}
		}()
		c.Next()
	}
}

// CORS 允许配置来源携带凭据跨域。
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range allowOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Idempotency-Key, X-Request-Id")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// OriginCheck 对不安全方法校验 Origin 头（存在时必须属于允许来源）。
// 浏览器跨站请求必带 Origin → 拒绝即缓解 CSRF；非浏览器客户端（curl/服务间调用）无 Origin 放行。
func OriginCheck(allowOrigins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range allowOrigins {
		allowed[strings.TrimRight(o, "/")] = true
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" || allowed[origin] {
			c.Next()
			return
		}
		apperr.RespondError(c, apperr.Forbidden("非法来源请求"))
	}
}

// SecureHeaders 添加基础安全响应头。
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// ---- 限流 ----

// RateLimit 固定窗口限流（Redis INCR + EXPIRE 60s）。
// 维度：登录用户按 user，匿名按 IP。limit <= 0 表示不限。
func RateLimit(rdb *redis.Client, bucket string, limit int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit <= 0 || rdb == nil {
			c.Next()
			return
		}
		subject := c.ClientIP()
		if u := CurrentUser(c); u != nil {
			subject = fmt.Sprintf("u%d", u.ID)
		}
		key := fmt.Sprintf("rl:%s:%s", bucket, subject)
		ctx := c.Request.Context()
		var count int64
		pipe := rdb.TxPipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, time.Minute)
		if _, err := pipe.Exec(ctx); err == nil {
			count = incr.Val()
		}
		if count > int64(limit) {
			apperr.RespondTooMany(c)
			return
		}
		c.Next()
	}
}

// ---- 认证 ----

// SessionAuth 可选认证：有有效会话则注入 AuthUser，无会话继续（游客）。
func SessionAuth(v SessionValidator, cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err != nil || token == "" {
			c.Next()
			return
		}
		u, err := v.ValidateSession(c.Request.Context(), token)
		if err != nil {
			c.Error(err) // 记录但不中断，游客态继续
			c.Next()
			return
		}
		if u != nil {
			c.Set(CtxAuthUser, u)
		}
		c.Next()
	}
}

// RequireAuth 强制认证；封禁/处罚账号拒绝互动。
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if u.Status == "banned" || u.Status == "suspended" {
			apperr.RespondError(c, apperr.ErrAccountSuspended)
			return
		}
		c.Next()
	}
}

// RequireVerified 强制已验证邮箱。
func RequireVerified() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if !u.Verified() {
			apperr.RespondError(c, apperr.ErrEmailNotVerified)
			return
		}
		c.Next()
	}
}

// RequireCanPost 发帖前处罚检查（post_ban/temp_ban/perma_ban 拒绝）。
func RequireCanPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if !u.CanPost() {
			apperr.RespondError(c, apperr.ErrSanctioned)
			return
		}
		c.Next()
	}
}

// RequireCanComment 评论前处罚检查（mute/temp_ban/perma_ban 拒绝）。
func RequireCanComment() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if !u.CanComment() {
			apperr.RespondError(c, apperr.ErrSanctioned)
			return
		}
		c.Next()
	}
}

// RequirePermission 强制指定权限点。
func RequirePermission(pc PermissionChecker, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if u.Status == "banned" || u.Status == "suspended" {
			apperr.RespondError(c, apperr.ErrAccountSuspended)
			return
		}
		ok, err := pc.HasPermission(c.Request.Context(), u.ID, code)
		if err != nil {
			apperr.RespondError(c, apperr.Internal(err))
			return
		}
		if !ok {
			apperr.RespondError(c, apperr.ErrForbidden)
			return
		}
		c.Next()
	}
}

// CurrentUser 从 context 取认证用户（可能为 nil，表示游客）。
func CurrentUser(c *gin.Context) *AuthUser {
	if v, ok := c.Get(CtxAuthUser); ok {
		if u, ok := v.(*AuthUser); ok {
			return u
		}
	}
	return nil
}
