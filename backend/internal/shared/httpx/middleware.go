// Package httpx 提供 Gin 全局中间件：请求 ID、恢复、CORS、安全头、限流、认证与 RBAC。
// 权限判断只在服务端执行；前端隐藏按钮仅是体验优化。
package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
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

var validRequestIDRe = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

// AuthUser 认证后的用户上下文（由 auth 模块装配）。
type AuthUser struct {
	ID              int64
	PublicID        string
	Username        string
	Email           string
	EmailVerified   bool
	Status          string
	SessionID       int64
	SanctionType    string     // 最高优先级生效处罚（兼容展示）
	SanctionEnds    *time.Time // 临时处罚结束时间
	ActiveSanctions []string   // 全部仍在生效期内的处罚类型集合（防多处罚覆盖绕过）
}

// Verified 判断账号是否已完成邮箱验证。
func (u *AuthUser) Verified() bool { return u != nil && u.EmailVerified }

// restrictiveSanctions 限制互动的处罚类型（warn 仅提醒，不限制）。
var restrictiveSanctions = map[string]bool{"mute": true, "post_ban": true, "temp_ban": true, "perma_ban": true}

func (u *AuthUser) activeSet() map[string]bool {
	out := map[string]bool{}
	if u == nil {
		return out
	}
	for _, s := range u.ActiveSanctions {
		if s != "" {
			out[s] = true
		}
	}
	if u.SanctionType != "" {
		if u.SanctionEnds == nil || !u.SanctionEnds.Before(time.Now().UTC()) {
			out[u.SanctionType] = true
		}
	}
	return out
}

// IsBanned 判断账号是否处于封禁处罚（temp_ban / perma_ban）。
func (u *AuthUser) IsBanned() bool {
	if u == nil {
		return false
	}
	if u.Status == "banned" || u.Status == "suspended" {
		return true
	}
	set := u.activeSet()
	return set["temp_ban"] || set["perma_ban"]
}

// Muted 判断账号是否处于限制互动的处罚期。
func (u *AuthUser) Muted() bool {
	for s := range u.activeSet() {
		if restrictiveSanctions[s] {
			return true
		}
	}
	return false
}

// CanPost 是否可发帖（post_ban/temp_ban/perma_ban 期间禁止）。
func (u *AuthUser) CanPost() bool {
	if u.IsBanned() {
		return false
	}
	set := u.activeSet()
	return !set["post_ban"] && !set["temp_ban"] && !set["perma_ban"]
}

// CanComment 是否可评论/发私信（mute/temp_ban/perma_ban 期间禁止）。
func (u *AuthUser) CanComment() bool {
	if u.IsBanned() {
		return false
	}
	set := u.activeSet()
	return !set["mute"] && !set["temp_ban"] && !set["perma_ban"]
}

// SessionValidator 由 auth 模块实现：校验会话 Cookie 并返回用户上下文。
type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (*AuthUser, error)
}

// PermissionChecker 由 RBAC 服务实现。
type PermissionChecker interface {
	HasPermission(ctx context.Context, userID int64, code string) (bool, error)
}

// GlobalPermissionChecker 仅匹配全局作用域权限（防止单版块版主越权全局管理接口）。
type GlobalPermissionChecker interface {
	HasGlobalPermission(ctx context.Context, userID int64, code string) (bool, error)
}

// ---- 基础中间件 ----

// RequestID 为每个请求生成/透传请求 ID（校验入参格式防止日志与响应头注入）。
func RequestID(header string) gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := strings.TrimSpace(c.GetHeader(header))
		if !validRequestIDRe.MatchString(rid) {
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

// OriginCheck 对不安全方法校验 Origin / Sec-Fetch-Site 头（防 CSRF）。
// 浏览器跨站请求必带 Origin 或 Sec-Fetch-Site=cross-site；非浏览器客户端（curl/测试）无上述头放行。
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
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		if origin == "" {
			// 无 Origin 时若浏览器明确声明跨站（Sec-Fetch-Site: cross-site）则拒绝
			if strings.EqualFold(strings.TrimSpace(c.GetHeader("Sec-Fetch-Site")), "cross-site") {
				apperr.RespondError(c, apperr.Forbidden("非法跨站请求"))
				return
			}
			c.Next()
			return
		}
		if allowed[origin] {
			c.Next()
			return
		}
		// 同源反代场景（如局域网 IP / 自定义端口访问）：限定 http(s) 且 Host 严格一致
		if u, err := url.Parse(origin); err == nil &&
			(u.Scheme == "http" || u.Scheme == "https") &&
			u.Host != "" &&
			strings.EqualFold(u.Host, c.Request.Host) {
			// 若反代传递了 X-Forwarded-Proto，则协议也须一致
			if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" && !strings.EqualFold(proto, u.Scheme) {
				apperr.RespondError(c, apperr.Forbidden("非法来源协议"))
				return
			}
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
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Next()
	}
}

// NoStore 禁止敏感接口响应被浏览器或代理缓存。
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}

// ---- 限流（Redis 固定窗口 Lua 原子计数 + 有界内存降级兜底） ----

var rlLua = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then
	redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return c
`)

type memRateEntry struct {
	count   int64
	resetAt time.Time
}

type memRateLimiter struct {
	mu    sync.Mutex
	items map[string]memRateEntry
	cap   int
}

func newMemRateLimiter(capacity int) *memRateLimiter {
	return &memRateLimiter{items: make(map[string]memRateEntry), cap: capacity}
}

func (m *memRateLimiter) allow(key string, limit int, window time.Duration) bool {
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) >= m.cap {
		for k, v := range m.items {
			if now.After(v.resetAt) {
				delete(m.items, k)
			}
		}
		for len(m.items) >= m.cap {
			for k := range m.items {
				delete(m.items, k)
				break
			}
		}
	}
	ent := m.items[key]
	if now.After(ent.resetAt) {
		ent = memRateEntry{count: 0, resetAt: now.Add(window)}
	}
	ent.count++
	m.items[key] = ent
	return ent.count <= int64(limit)
}

// RateLimit 固定窗口限流（优先 Redis Lua 原子计数，Redis 缺失或异常时自动降级有界内存限流）。
// 维度：登录用户按 user，匿名按 IP。limit <= 0 表示不限。
func RateLimit(rdb *redis.Client, bucket string, limit int) gin.HandlerFunc {
	fallback := newMemRateLimiter(10000)
	return func(c *gin.Context) {
		if limit <= 0 {
			c.Next()
			return
		}
		subject := c.ClientIP()
		if u := CurrentUser(c); u != nil {
			subject = fmt.Sprintf("u%d", u.ID)
		}
		key := fmt.Sprintf("rl:%s:%s", bucket, subject)
		if rdb != nil {
			res, err := rlLua.Run(c.Request.Context(), rdb, []string{key}, 60).Int64()
			if err == nil {
				if res > int64(limit) {
					apperr.RespondTooMany(c)
					return
				}
				c.Next()
				return
			}
		}
		// Redis 未连接或临时故障时走有界内存兜底，杜绝 fail-open
		if !fallback.allow(key, limit, time.Minute) {
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

// RequireAuth 强制认证；封禁（含 temp_ban/perma_ban）账号拒绝访问。
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if u.IsBanned() {
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
		if u.IsBanned() {
			apperr.RespondError(c, apperr.ErrAccountSuspended)
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

// RequireCanComment 评论/私信前处罚检查（mute/temp_ban/perma_ban 拒绝）。
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

// RequirePermission 强制指定权限点（含版块范围角色，需要对象级范围校验的接口由 service 二次校验）。
func RequirePermission(pc PermissionChecker, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if u.IsBanned() {
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

// RequireGlobalPermission 强制要求全局作用域权限点（禁止单版块版主越权全局接口）。
func RequireGlobalPermission(pc GlobalPermissionChecker, code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			apperr.RespondError(c, apperr.ErrUnauthenticated)
			return
		}
		if u.IsBanned() {
			apperr.RespondError(c, apperr.ErrAccountSuspended)
			return
		}
		ok, err := pc.HasGlobalPermission(c.Request.Context(), u.ID, code)
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
