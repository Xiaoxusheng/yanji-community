package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// Service 认证业务逻辑。事务边界与状态机全部在此层控制。
type Service struct {
	pool   *pgxpool.Pool
	repo   *Repo
	logger *slog.Logger

	sessionTTL time.Duration
	publicURL  string
}

// New 构造认证服务。
func New(pool *pgxpool.Pool, logger *slog.Logger, sessionTTL time.Duration, publicURL string) *Service {
	return &Service{pool: pool, repo: NewRepo(pool), logger: logger, sessionTTL: sessionTTL, publicURL: publicURL}
}

// Repo 暴露给路由层（校验会话）。
func (s *Service) Repo() *Repo { return s.repo }

// Me 当前登录用户信息。
type Me struct {
	ID            string    `json:"id"`
	Username      string    `json:"username"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

// RegisterRequest 注册请求。
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register 公开注册：创建用户、写验证邮件事件。防枚举：冲突统一提示。
func (s *Service) Register(ctx context.Context, req RegisterRequest) error {
	if err := ValidateRegister(req.Username, req.Email, req.Password); err != nil {
		return err
	}
	hash, err := HashPassword(req.Password)
	if err != nil {
		return apperr.Internal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	u, err := s.repo.CreateTx(ctx, tx, req.Username, req.Email, hash)
	if err != nil {
		return err
	}
	token, err := s.repo.CreateEmailToken(ctx, u.ID, EmailTokenTTL)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := EmitVerificationEmail(ctx, tx, u.Email, u.Username, token); err != nil {
		return apperr.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(err)
	}
	s.logger.Info("user registered", "username", u.Username)
	return nil
}

// LoginRequest 登录请求。
type LoginRequest struct {
	Account  string `json:"account"` // 用户名或邮箱
	Password string `json:"password"`
	IP       string `json:"-"`
	UA       string `json:"-"`
}

// Login 登录并返回会话 token（由 Handler 写入 Cookie）。
func (s *Service) Login(ctx context.Context, req LoginRequest) (string, error) {
	if req.Account == "" || req.Password == "" {
		return "", apperr.BadRequest("VALIDATION_ERROR", "请输入账号与密码")
	}
	identity, err := s.repo.FindByIdentity(ctx, req.Account)
	if err != nil {
		return "", apperr.Internal(err)
	}
	if identity == nil || !CheckPassword(identity.PasswordHash, req.Password) {
		if identity != nil {
			s.repo.LogLogin(ctx, identity.ID, false, req.IP, req.UA)
		}
		return "", apperr.BadRequest("INVALID_CREDENTIALS", "账号或密码不正确")
	}
	if identity.Status == "banned" || identity.Status == "suspended" {
		s.repo.LogLogin(ctx, identity.ID, false, req.IP, req.UA)
		return "", apperr.ErrAccountSuspended
	}
	token, err := s.repo.CreateSession(ctx, identity.ID, deviceFromUA(req.UA), req.UA, req.IP, s.sessionTTL)
	if err != nil {
		return "", apperr.Internal(err)
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE users SET last_login_at = now_utc(), last_active_at = now_utc() WHERE id = $1`, identity.ID)
	s.repo.LogLogin(ctx, identity.ID, true, req.IP, req.UA)
	return token, nil
}

// Logout 撤销当前会话。
func (s *Service) Logout(ctx context.Context, sessionID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = now_utc() WHERE id = $1`, sessionID)
	return err
}

// LogoutAll 撤销全部会话。
func (s *Service) LogoutAll(ctx context.Context, userID int64) error {
	return s.repo.RevokeAllSessions(ctx, userID)
}

// ValidateSession 实现 httpx.SessionValidator。
func (s *Service) ValidateSession(ctx context.Context, token string) (*SessionUser, error) {
	return s.repo.ValidateToken(ctx, token)
}

// Sessions 列出会话设备。
func (s *Service) Sessions(ctx context.Context, userID, currentSessionID int64) ([]SessionRow, error) {
	return s.repo.ListSessions(ctx, userID, currentSessionID)
}

// RevokeSession 撤销指定会话。
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID int64) error {
	return s.repo.RevokeSession(ctx, userID, sessionID)
}

// VerifyEmail 邮箱验证令牌兑换。
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	userID, err := s.repo.ConsumeEmailToken(ctx, token)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.SetEmailVerified(ctx, tx, userID); err != nil {
		return apperr.Internal(err)
	}
	var email, username string
	if err := tx.QueryRow(ctx, `SELECT email_norm, username FROM users WHERE id = $1`, userID).
		Scan(&email, &username); err != nil {
		return apperr.Internal(err)
	}
	if err := EmitWelcomeEmail(ctx, tx, email, username); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// ResendVerification 重发验证邮件（已验证用户静默返回，防枚举）。
func (s *Service) ResendVerification(ctx context.Context, userID int64) error {
	verified, err := s.repo.IsEmailVerified(ctx, userID)
	if err != nil {
		return apperr.Internal(err)
	}
	if verified {
		return nil
	}
	return s.sendVerificationFor(ctx, userID)
}

func (s *Service) sendVerificationFor(ctx context.Context, userID int64) error {
	var email, username string
	err := s.pool.QueryRow(ctx, `SELECT email_norm, username FROM users WHERE id = $1`, userID).
		Scan(&email, &username)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("用户不存在")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	token, err := s.repo.CreateEmailToken(ctx, userID, EmailTokenTTL)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := EmitVerificationEmail(ctx, tx, email, username, token); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// ForgotPassword 发起密码重置；无论邮箱是否存在都返回成功（防枚举）。
func (s *Service) ForgotPassword(ctx context.Context, email string) error {
	userID, err := s.repo.FindEmailForReset(ctx, email)
	if err != nil {
		return apperr.Internal(err)
	}
	if userID == 0 {
		return nil
	}
	var username string
	_ = s.pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&username)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	token, err := s.repo.CreateResetToken(ctx, userID, PasswordResetTTL)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := EmitPasswordResetEmail(ctx, tx, email, username, token); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// ResetPassword 重置密码并撤销全部会话。
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if len(newPassword) < 8 || len(newPassword) > 72 {
		return apperr.BadRequest("VALIDATION_ERROR", "密码长度需为 8-72 位")
	}
	userID, err := s.repo.ConsumeResetToken(ctx, token)
	if err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return apperr.Internal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.UpdatePasswordTx(ctx, tx, userID, hash); err != nil {
		return apperr.Internal(err)
	}
	if err := s.repo.RevokeAllSessions(ctx, userID); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// VerificationLink 生成前端验证链接。
func (s *Service) VerificationLink(token string) string {
	return s.publicURL + "/verify-email?token=" + token
}

// ResetLink 生成前端重置密码链接。
func (s *Service) ResetLink(token string) string {
	return s.publicURL + "/reset-password?token=" + token
}

// SessionCookieTTL 会话有效期。
func (s *Service) SessionCookieTTL() time.Duration { return s.sessionTTL }

// deviceFromUA 从 User-Agent 提取简短设备名。
func deviceFromUA(ua string) string {
	ua = strings.TrimSpace(ua)
	if ua == "" {
		return "未知设备"
	}
	if len(ua) > 60 {
		ua = ua[:60]
	}
	return ua
}

// strings 引用
var _ = errors.Is
