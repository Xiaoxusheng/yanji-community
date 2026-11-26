package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 认证 HTTP 层。
type Handler struct {
	svc          *Service
	captcha      CaptchaVerifier
	cookieDomain string
	cookieSecure bool
	canAdmin     func(ctx context.Context, userID int64) bool
}

// NewHandler 构造。
func NewHandler(svc *Service, cookieDomain string, cookieSecure bool) *Handler {
	return &Handler{
		svc:          svc,
		captcha:      NewCaptchaStore(nil),
		cookieDomain: cookieDomain,
		cookieSecure: cookieSecure,
	}
}

// SetCaptcha 注入图形验证码验证器。
func (h *Handler) SetCaptcha(v CaptchaVerifier) {
	if v != nil {
		h.captcha = v
	}
}

// SetCanAdmin 注入后台入口权限判断回调（仅用于 UI 显隐，安全校验由 RBAC 中间件负责）。
func (h *Handler) SetCanAdmin(fn func(ctx context.Context, userID int64) bool) { h.canAdmin = fn }

// SetSessionCookie 写入会话 Cookie（HttpOnly + SameSite=Lax，生产 Secure）。
// 注意：Gin 必须在 c.SetCookie 之前调用 c.SetSameSite 才能将 SameSite 写入响应头。
func (h *Handler) SetSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, token, maxAge, "/", h.cookieDomain, h.cookieSecure, true)
}

// ClearSessionCookie 清除会话 Cookie。
func (h *Handler) ClearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(CookieName, "", -1, "/", h.cookieDomain, h.cookieSecure, true)
}

// Captcha GET /auth/captcha —— 生成一次性图形验证码。
func (h *Handler) Captcha(c *gin.Context) {
	ch, err := h.captcha.Generate(c.Request.Context())
	if err != nil {
		respond.Fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	respond.OK(c, ch)
}

// CurrentToken 从 Cookie 取会话 token。
func CurrentToken(c *gin.Context) string {
	t, _ := c.Cookie(CookieName)
	return t
}

type registerReq struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	CaptchaID   string `json:"captcha_id"`
	CaptchaCode string `json:"captcha_code"`
}

// Register POST /auth/register
func (h *Handler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入注册信息"))
		return
	}
	if err := h.captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode); err != nil {
		respond.Fail(c, err)
		return
	}
	if err := h.svc.Register(c.Request.Context(), RegisterRequest{
		Username: req.Username, Email: req.Email, Password: req.Password,
	}); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"message": "注册成功，请查收验证邮件完成验证"})
}

type loginReq struct {
	Account     string `json:"account"`
	Password    string `json:"password"`
	CaptchaID   string `json:"captcha_id"`
	CaptchaCode string `json:"captcha_code"`
}

// Login POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入账号、密码与验证码"))
		return
	}
	if err := h.captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode); err != nil {
		respond.Fail(c, err)
		return
	}
	token, err := h.svc.Login(c.Request.Context(), LoginRequest{
		Account: req.Account, Password: req.Password,
		IP: c.ClientIP(), UA: c.Request.UserAgent(),
	})
	if err != nil {
		respond.Fail(c, err)
		return
	}
	h.SetSessionCookie(c, token, int(h.svc.SessionCookieTTL().Seconds()))
	respond.OK(c, gin.H{"message": "登录成功"})
}

// Logout POST /auth/logout
func (h *Handler) Logout(c *gin.Context) {
	if u := httpx.CurrentUser(c); u != nil {
		_ = h.svc.Logout(c.Request.Context(), u.SessionID)
	}
	h.ClearSessionCookie(c)
	respond.OK(c, gin.H{"message": "已退出登录"})
}

// LogoutAll POST /auth/logout-all
func (h *Handler) LogoutAll(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.Fail(c, apperr.ErrUnauthenticated)
		return
	}
	if err := h.svc.LogoutAll(c.Request.Context(), u.ID); err != nil {
		respond.Fail(c, err)
		return
	}
	h.ClearSessionCookie(c)
	respond.OK(c, gin.H{"message": "已退出全部设备"})
}

// Me GET /auth/me —— 游客返回 authenticated:false（禁止缓存敏感账号信息）。
func (h *Handler) Me(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.OK(c, gin.H{"authenticated": false, "user": nil})
		return
	}
	canAdmin := false
	if h.canAdmin != nil {
		canAdmin = h.canAdmin(c.Request.Context(), u.ID)
	}
	respond.OK(c, gin.H{
		"authenticated": true,
		"user": gin.H{
			"id":             u.PublicID,
			"username":       u.Username,
			"email":          u.Email,
			"email_verified": u.EmailVerified,
			"status":         u.Status,
			"sanctioned":     u.Muted(),
			"sanction_type":  u.SanctionType,
			"can_admin":      canAdmin,
		},
	})
}

type tokenReq struct {
	Token string `json:"token"`
}

// VerifyEmail POST /auth/email/verify
func (h *Handler) VerifyEmail(c *gin.Context) {
	var req tokenReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "缺少验证令牌"))
		return
	}
	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "邮箱验证成功，现在可以发帖和评论了"})
}

// ResendVerification POST /auth/email/resend
func (h *Handler) ResendVerification(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.Fail(c, apperr.ErrUnauthenticated)
		return
	}
	if err := h.svc.ResendVerification(c.Request.Context(), u.ID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "验证邮件已发送，请查收"})
}

type forgotReq struct {
	Email       string `json:"email"`
	CaptchaID   string `json:"captcha_id"`
	CaptchaCode string `json:"captcha_code"`
}

// ForgotPassword POST /auth/password/forgot
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入注册邮箱与验证码"))
		return
	}
	if err := h.captcha.Verify(c.Request.Context(), req.CaptchaID, req.CaptchaCode); err != nil {
		respond.Fail(c, err)
		return
	}
	// 防枚举：一律返回成功
	if err := h.svc.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "如该邮箱已注册，将收到密码重置邮件"})
}

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword POST /auth/password/change —— 需登录，验证原密码并轮换会话。
func (h *Handler) ChangePassword(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.Fail(c, apperr.ErrUnauthenticated)
		return
	}
	var req changePwdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入当前密码与新密码"))
		return
	}
	newToken, err := h.svc.ChangePassword(
		c.Request.Context(), u.ID, req.OldPassword, req.NewPassword, c.ClientIP(), c.Request.UserAgent(),
	)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	h.SetSessionCookie(c, newToken, int(h.svc.SessionCookieTTL().Seconds()))
	respond.OK(c, gin.H{"message": "密码修改成功，其他设备已自动下线"})
}

type resetReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// ResetPassword POST /auth/password/reset
func (h *Handler) ResetPassword(c *gin.Context) {
	var req resetReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "缺少重置令牌"))
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		respond.Fail(c, err)
		return
	}
	h.ClearSessionCookie(c)
	respond.OK(c, gin.H{"message": "密码已重置，请使用新密码登录"})
}

// Sessions GET /auth/sessions
func (h *Handler) Sessions(c *gin.Context) {
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.Fail(c, apperr.ErrUnauthenticated)
		return
	}
	list, err := h.svc.Sessions(c.Request.Context(), u.ID, u.SessionID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": list})
}

type revokeReq struct {
	ID int64 `json:"id" uri:"id"`
}

// RevokeSession DELETE /auth/sessions/:id
func (h *Handler) RevokeSession(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.Fail(c, apperr.ErrUnauthenticated)
		return
	}
	var uri revokeReq
	if err := c.ShouldBindUri(&uri); err != nil || uri.ID <= 0 {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "会话 ID 不合法"))
		return
	}
	if err := h.svc.RevokeSession(c.Request.Context(), u.ID, uri.ID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "会话已撤销"})
}
