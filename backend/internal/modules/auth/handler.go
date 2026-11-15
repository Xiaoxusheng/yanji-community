package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 认证 HTTP 层。
type Handler struct {
	svc          *Service
	cookieDomain string
	cookieSecure bool
}

// NewHandler 构造。
func NewHandler(svc *Service, cookieDomain string, cookieSecure bool) *Handler {
	return &Handler{svc: svc, cookieDomain: cookieDomain, cookieSecure: cookieSecure}
}

// SetSessionCookie 写入会话 Cookie（HttpOnly + SameSite=Lax，生产 Secure）。
func (h *Handler) SetSessionCookie(c *gin.Context, token string, maxAge int) {
	c.SetCookie(CookieName, token, maxAge, "/", h.cookieDomain, h.cookieSecure, true)
	if !h.cookieSecure {
		c.SetSameSite(http.SameSiteLaxMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
}

// ClearSessionCookie 清除会话 Cookie。
func (h *Handler) ClearSessionCookie(c *gin.Context) {
	c.SetCookie(CookieName, "", -1, "/", h.cookieDomain, h.cookieSecure, true)
}

// CurrentToken 从 Cookie 取会话 token。
func CurrentToken(c *gin.Context) string {
	t, _ := c.Cookie(CookieName)
	return t
}

type registerReq struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register POST /auth/register
func (h *Handler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入注册信息"))
		return
	}
	if err := h.svc.Register(c.Request.Context(), RegisterRequest(req)); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"message": "注册成功，请查收验证邮件完成验证"})
}

type loginReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// Login POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入账号与密码"))
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

// Me GET /auth/me —— 游客返回 authenticated:false。
func (h *Handler) Me(c *gin.Context) {
	u := httpx.CurrentUser(c)
	if u == nil {
		respond.OK(c, gin.H{"authenticated": false, "user": nil})
		return
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
	Email string `json:"email"`
}

// ForgotPassword POST /auth/password/forgot
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req forgotReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请输入注册邮箱"))
		return
	}
	// 防枚举：一律返回成功
	if err := h.svc.ForgotPassword(c.Request.Context(), req.Email); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "如该邮箱已注册，将收到密码重置邮件"})
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
