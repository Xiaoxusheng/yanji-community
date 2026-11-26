package auth

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

const (
	loginLockWindow      = 15 * time.Minute
	loginMaxFailPerPair  = 5
	loginMaxFailPerAcc   = 12
	loginLockMemCap      = 10000
	loginFailLuaScript   = `local c = redis.call('INCR', KEYS[1]); if c == 1 then redis.call('EXPIRE', KEYS[1], ARGV[1]) end; return c`
	dummyBcryptHashConst = "$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW"
)

type loginFailEntry struct {
	count     int
	expiresAt time.Time
}

// Service 认证业务逻辑。事务边界与状态机全部在此层控制。
type Service struct {
	pool   *pgxpool.Pool
	repo   *Repo
	logger *slog.Logger
	rdb    *redis.Client

	failMu  sync.Mutex
	failMem map[string]loginFailEntry

	sessionTTL time.Duration
	publicURL  string
	points     pointsRegistrar
}

// pointsRegistrar 注册积分奖励接口（由 points 模块实现，避免循环依赖）。
type pointsRegistrar interface {
	GrantTx(ctx context.Context, tx pgx.Tx, userID int64, sourceType string, sourceID int64, idempotencyKey string, amount int64, note string, operatorID int64) error
}

// SetPoints 注入积分服务（注册奖励）。
func (s *Service) SetPoints(p pointsRegistrar) { s.points = p }

// SetRedis 注入 Redis 客户端（用于登录防暴破计数，未设置时自动使用进程内存限流兜底）。
func (s *Service) SetRedis(r *redis.Client) { s.rdb = r }

// New 构造认证服务。
func New(pool *pgxpool.Pool, logger *slog.Logger, sessionTTL time.Duration, publicURL string) *Service {
	return &Service{
		pool: pool, repo: NewRepo(pool), logger: logger,
		failMem:    make(map[string]loginFailEntry),
		sessionTTL: sessionTTL, publicURL: publicURL,
	}
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
	username := strings.TrimSpace(req.Username)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if err := ValidateRegister(username, email, req.Password); err != nil {
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

	u, err := s.repo.CreateTx(ctx, tx, username, email, hash)
	if err != nil {
		return err
	}
	token, err := s.repo.CreateEmailTokenTx(ctx, tx, u.ID, EmailTokenTTL)
	if err != nil {
		return apperr.Internal(err)
	}
	if err := EmitVerificationEmail(ctx, tx, u.Email, u.Username, token); err != nil {
		return apperr.Internal(err)
	}
	// 注册奖励（幂等键：registration:{uid}）
	if s.points != nil {
		if err := s.points.GrantTx(ctx, tx, u.ID, "registration", u.ID,
			"registration:"+itoa64(u.ID), 20, "加入社区奖励", 0); err != nil {
			return apperr.Internal(err)
		}
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

// Login 登录并返回会话 token（含单账号防暴破锁定与等时密码校验，防止用户名枚举）。
func (s *Service) Login(ctx context.Context, req LoginRequest) (string, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" || req.Password == "" {
		return "", apperr.BadRequest("VALIDATION_ERROR", "请输入账号与密码")
	}
	if err := s.checkLoginLockout(ctx, account, req.IP); err != nil {
		return "", err
	}
	identity, err := s.repo.FindByIdentity(ctx, account)
	if err != nil {
		return "", apperr.Internal(err)
	}
	if identity == nil {
		// 保持等时消耗，防止通过耗时差异枚举账号是否存在
		_ = CheckPassword(dummyBcryptHashConst, req.Password)
		s.recordLoginFailure(ctx, account, req.IP)
		return "", apperr.BadRequest("INVALID_CREDENTIALS", "账号或密码不正确")
	}
	if err := s.checkLoginLockout(ctx, "uid:"+itoa64(identity.ID), req.IP); err != nil {
		return "", err
	}
	if !CheckPassword(identity.PasswordHash, req.Password) {
		s.repo.LogLogin(ctx, identity.ID, false, req.IP, req.UA)
		s.recordLoginFailure(ctx, account, req.IP)
		// 同步按用户 ID 计数，防止交替使用用户名与邮箱登录将尝试上限翻倍
		s.recordLoginFailure(ctx, "uid:"+itoa64(identity.ID), req.IP)
		return "", apperr.BadRequest("INVALID_CREDENTIALS", "账号或密码不正确")
	}
	if identity.Status == "banned" || identity.Status == "suspended" {
		s.repo.LogLogin(ctx, identity.ID, false, req.IP, req.UA)
		return "", apperr.ErrAccountSuspended
	}
	if banned, berr := s.repo.HasActiveBan(ctx, identity.ID); berr == nil && banned {
		s.repo.LogLogin(ctx, identity.ID, false, req.IP, req.UA)
		return "", apperr.ErrAccountSuspended
	}
	s.clearLoginFailure(ctx, account, req.IP)
	s.clearLoginFailure(ctx, "uid:"+itoa64(identity.ID), req.IP)
	token, err := s.repo.CreateSession(ctx, identity.ID, deviceFromUA(req.UA), req.UA, req.IP, s.sessionTTL)
	if err != nil {
		return "", apperr.Internal(err)
	}
	_, _ = s.pool.Exec(ctx,
		`UPDATE users SET last_login_at = now_utc(), last_active_at = now_utc() WHERE id = $1`, identity.ID)
	s.repo.LogLogin(ctx, identity.ID, true, req.IP, req.UA)
	return token, nil
}

func loginPairKey(account, ip string) string {
	return "login:fail:pair:" + TokenHash(strings.ToLower(strings.TrimSpace(account))+"|"+strings.TrimSpace(ip))
}

func loginAccKey(account string) string {
	return "login:fail:acc:" + TokenHash(strings.ToLower(strings.TrimSpace(account)))
}

func (s *Service) checkLoginLockout(ctx context.Context, account, ip string) error {
	pairKey := loginPairKey(account, ip)
	accKey := loginAccKey(account)
	if s.rdb != nil {
		if n, err := s.rdb.Get(ctx, pairKey).Int(); err == nil && n >= loginMaxFailPerPair {
			return apperr.TooMany("登录失败次数过多，请 15 分钟后再试")
		}
		if n, err := s.rdb.Get(ctx, accKey).Int(); err == nil && n >= loginMaxFailPerAcc {
			return apperr.TooMany("该账号登录尝试过于频繁，请 15 分钟后再试")
		}
		return nil
	}
	now := time.Now().UTC()
	s.failMu.Lock()
	defer s.failMu.Unlock()
	if ent, ok := s.failMem[pairKey]; ok && !now.After(ent.expiresAt) && ent.count >= loginMaxFailPerPair {
		return apperr.TooMany("登录失败次数过多，请 15 分钟后再试")
	}
	if ent, ok := s.failMem[accKey]; ok && !now.After(ent.expiresAt) && ent.count >= loginMaxFailPerAcc {
		return apperr.TooMany("该账号登录尝试过于频繁，请 15 分钟后再试")
	}
	return nil
}

func (s *Service) recordLoginFailure(ctx context.Context, account, ip string) {
	pairKey := loginPairKey(account, ip)
	accKey := loginAccKey(account)
	if s.rdb != nil {
		ttlSec := int(loginLockWindow.Seconds())
		_, _ = s.rdb.Eval(ctx, loginFailLuaScript, []string{pairKey}, ttlSec).Result()
		_, _ = s.rdb.Eval(ctx, loginFailLuaScript, []string{accKey}, ttlSec).Result()
		return
	}
	now := time.Now().UTC()
	s.failMu.Lock()
	defer s.failMu.Unlock()
	if len(s.failMem) >= loginLockMemCap {
		for k, v := range s.failMem {
			if now.After(v.expiresAt) {
				delete(s.failMem, k)
			}
		}
		for len(s.failMem) >= loginLockMemCap {
			for k := range s.failMem {
				delete(s.failMem, k)
				break
			}
		}
	}
	for _, k := range []string{pairKey, accKey} {
		ent := s.failMem[k]
		if now.After(ent.expiresAt) {
			ent = loginFailEntry{count: 0, expiresAt: now.Add(loginLockWindow)}
		}
		ent.count++
		s.failMem[k] = ent
	}
}

func (s *Service) clearLoginFailure(ctx context.Context, account, ip string) {
	pairKey := loginPairKey(account, ip)
	accKey := loginAccKey(account)
	if s.rdb != nil {
		_ = s.rdb.Del(ctx, pairKey, accKey).Err()
		return
	}
	s.failMu.Lock()
	delete(s.failMem, pairKey)
	delete(s.failMem, accKey)
	s.failMu.Unlock()
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
	token, err := s.repo.CreateEmailTokenTx(ctx, tx, userID, EmailTokenTTL)
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
	email = strings.ToLower(strings.TrimSpace(email))
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
	token, err := s.repo.CreateResetTokenTx(ctx, tx, userID, PasswordResetTTL)
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
	if err := ValidatePassword(newPassword); err != nil {
		return err
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
	if err := s.repo.RevokeAllSessionsTx(ctx, tx, userID); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// ChangePassword 已登录用户验证当前密码后修改密码，吊销全部旧会话并为当前设备轮换签发新会话。
func (s *Service) ChangePassword(ctx context.Context, userID int64, oldPassword, newPassword, ip, ua string) (string, error) {
	if oldPassword == "" || newPassword == "" {
		return "", apperr.BadRequest("VALIDATION_ERROR", "请输入当前密码与新密码")
	}
	if oldPassword == newPassword {
		return "", apperr.BadRequest("VALIDATION_ERROR", "新密码不能与当前密码相同")
	}
	if err := ValidatePassword(newPassword); err != nil {
		return "", err
	}
	pwdLockAcc := "chpwd:" + itoa64(userID)
	if err := s.checkLoginLockout(ctx, pwdLockAcc, ip); err != nil {
		return "", apperr.TooMany("当前密码验证失败次数过多，请 15 分钟后再试")
	}
	hash, err := s.repo.GetPasswordHash(ctx, userID)
	if err != nil {
		return "", err
	}
	if !CheckPassword(hash, oldPassword) {
		s.recordLoginFailure(ctx, pwdLockAcc, ip)
		return "", apperr.BadRequest("INVALID_CREDENTIALS", "当前密码不正确")
	}
	s.clearLoginFailure(ctx, pwdLockAcc, ip)
	newHash, err := HashPassword(newPassword)
	if err != nil {
		return "", apperr.Internal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.UpdatePasswordTx(ctx, tx, userID, newHash); err != nil {
		return "", apperr.Internal(err)
	}
	if err := s.repo.RevokeAllSessionsTx(ctx, tx, userID); err != nil {
		return "", apperr.Internal(err)
	}
	token, err := s.repo.CreateSessionTx(ctx, tx, userID, deviceFromUA(ua), ua, ip, s.sessionTTL)
	if err != nil {
		return "", apperr.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", apperr.Internal(err)
	}
	s.logger.Info("password changed", "user_id", userID)
	return token, nil
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

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
