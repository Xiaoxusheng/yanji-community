// Package auth 实现注册、登录、会话、邮箱验证与密码重置。
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/outbox"
)

const (
	CookieName = "yanji_session"
	bcryptCost = 12
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// TokenTTL 各类令牌有效期。
var (
	SessionTTL       = 7 * 24 * time.Hour
	EmailTokenTTL    = 24 * time.Hour
	PasswordResetTTL = 30 * time.Minute
)

// Repo 认证数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// UserRow 用户核心字段。
type UserRow struct {
	ID            int64
	PublicID      string
	Username      string
	Email         string
	Status        string
	EmailVerified bool
	CreatedAt     time.Time
}

// CreateTx 在事务内创建用户、资料并授予 member 角色，返回新用户。
func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, username, emailNorm, passwordHash string) (UserRow, error) {
	row := tx.QueryRow(ctx, `
		INSERT INTO users (username, email_norm, password_hash, status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, public_id, username, email_norm, status, (email_verified_at IS NOT NULL), created_at`,
		username, emailNorm, passwordHash)
	var u UserRow
	if err := row.Scan(&u.ID, &u.PublicID, &u.Username, &u.Email, &u.Status, &u.EmailVerified, &u.CreatedAt); err != nil {
		if strings.Contains(err.Error(), "users_username_unique") ||
			strings.Contains(err.Error(), "users_email_unique") {
			return u, apperr.ErrEmailTaken
		}
		return u, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO user_profiles (user_id, display_name) VALUES ($1, $2)`, u.ID, u.Username); err != nil {
		return u, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id, scope_type)
		SELECT $1, id, 'global' FROM roles WHERE code = 'member'
		ON CONFLICT DO NOTHING`, u.ID); err != nil {
		return u, err
	}
	return u, nil
}

// FindByIdentity 按用户名或邮箱查找（登录用）。
func (r *Repo) FindByIdentity(ctx context.Context, account string) (*LoginIdentity, error) {
	var li LoginIdentity
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, email_norm, password_hash, status, (email_verified_at IS NOT NULL)
		FROM users
		WHERE (username = $1 OR email_norm = lower($2)) AND deleted_at IS NULL
		LIMIT 1`, account, account).
		Scan(&li.ID, &li.Username, &li.Email, &li.PasswordHash, &li.Status, &li.EmailVerified)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &li, nil
}

// LoginIdentity 登录所需的用户字段。
type LoginIdentity struct {
	ID            int64
	Username      string
	Email         string
	PasswordHash  string
	Status        string
	EmailVerified bool
}

// CreateSession 创建会话并返回原始 token（仅此一次可见，DB 只存摘要）。
func (r *Repo) CreateSession(ctx context.Context, userID int64, deviceName, ua, ip string, ttl time.Duration) (string, error) {
	token := randomToken()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_sessions (user_id, token_hash, device_name, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5, now_utc() + ($6::text)::interval)`,
		userID, TokenHash(token), deviceName, ua, ip, fmt.Sprintf("%d seconds", int(ttl.Seconds())))
	if err != nil {
		return "", err
	}
	return token, nil
}

// RevokeSession 撤销会话（校验归属）。
func (r *Repo) RevokeSession(ctx context.Context, userID, sessionID int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = now_utc() WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		sessionID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("会话不存在")
	}
	return nil
}

// RevokeAllSessions 撤销用户全部会话。
func (r *Repo) RevokeAllSessions(ctx context.Context, userID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE user_sessions SET revoked_at = now_utc() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// ValidateToken 校验会话 token，返回用户上下文。
func (r *Repo) ValidateToken(ctx context.Context, token string) (*SessionUser, error) {
	hash := TokenHash(token)
	var su SessionUser
	var expiresAt time.Time
	var revoked *time.Time
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, u.id, u.public_id::text, u.username, u.email_norm, u.status,
		       (u.email_verified_at IS NOT NULL), s.expires_at, s.revoked_at
		FROM user_sessions s
		JOIN users u ON u.id = s.user_id AND u.deleted_at IS NULL
		WHERE s.token_hash = $1`, hash).
		Scan(&su.SessionID, &su.ID, &su.PublicID, &su.Username, &su.Email, &su.Status,
			&su.EmailVerified, &expiresAt, &revoked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if revoked != nil || expiresAt.Before(time.Now().UTC()) {
		return nil, nil
	}
	// 节流更新活跃时间（60s 一次）
	_, _ = r.pool.Exec(ctx, `
		UPDATE user_sessions SET last_seen_at = now_utc()
		WHERE id = $1 AND last_seen_at < now_utc() - interval '60 seconds'`, su.SessionID)
	_, _ = r.pool.Exec(ctx, `
		UPDATE users SET last_active_at = now_utc()
		WHERE id = $1 AND (last_active_at IS NULL OR last_active_at < now_utc() - interval '60 seconds')`, su.ID)

	// 查询生效处罚
	_ = r.pool.QueryRow(ctx, `
		SELECT sanction_type, ends_at FROM user_sanctions
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC LIMIT 1`, su.ID).
		Scan(&su.SanctionType, &su.SanctionEnds)
	return &su, nil
}

// SessionUser 会话携带的用户上下文。
type SessionUser struct {
	ID            int64
	PublicID      string
	Username      string
	Email         string
	Status        string
	EmailVerified bool
	SessionID     int64
	SanctionType  string
	SanctionEnds  *time.Time
}

// SessionRow 设备列表行。
type SessionRow struct {
	ID         int64     `json:"id"`
	DeviceName string    `json:"device_name"`
	IPAddress  string    `json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Revoked    bool      `json:"revoked"`
	Current    bool      `json:"current"`
}

// ListSessions 返回用户有效会话列表。
func (r *Repo) ListSessions(ctx context.Context, userID, currentSessionID int64) ([]SessionRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, device_name, ip_address, created_at, last_seen_at, expires_at,
		       (revoked_at IS NOT NULL)
		FROM user_sessions
		WHERE user_id = $1 AND expires_at > now_utc() AND revoked_at IS NULL
		ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SessionRow
	for rows.Next() {
		var s SessionRow
		if err := rows.Scan(&s.ID, &s.DeviceName, &s.IPAddress, &s.CreatedAt, &s.LastSeenAt,
			&s.ExpiresAt, &s.Revoked); err != nil {
			return nil, err
		}
		s.Current = s.ID == currentSessionID
		out = append(out, s)
	}
	return out, rows.Err()
}

// CreateEmailToken 创建一次性邮箱验证令牌，返回原始 token。
func (r *Repo) CreateEmailToken(ctx context.Context, userID int64, ttl time.Duration) (string, error) {
	token := randomToken()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now_utc() + ($3::text)::interval)`,
		userID, TokenHash(token), fmt.Sprintf("%d seconds", int(ttl.Seconds())))
	return token, err
}

// CreateResetToken 创建一次性密码重置令牌，返回原始 token。
func (r *Repo) CreateResetToken(ctx context.Context, userID int64, ttl time.Duration) (string, error) {
	token := randomToken()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, now_utc() + ($3::text)::interval)`,
		userID, TokenHash(token), fmt.Sprintf("%d seconds", int(ttl.Seconds())))
	return token, err
}

// ConsumeEmailToken 校验并消费验证令牌，返回 user_id。
func (r *Repo) ConsumeEmailToken(ctx context.Context, token string) (int64, error) {
	var userID int64
	err := r.pool.QueryRow(ctx, `
		UPDATE email_verification_tokens
		SET used_at = now_utc()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now_utc()
		RETURNING user_id`, TokenHash(token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.BadRequest("VALIDATION_ERROR", "验证链接无效或已过期")
	}
	return userID, err
}

// ConsumeResetToken 校验并消费重置令牌，返回 user_id。
func (r *Repo) ConsumeResetToken(ctx context.Context, token string) (int64, error) {
	var userID int64
	err := r.pool.QueryRow(ctx, `
		UPDATE password_reset_tokens
		SET used_at = now_utc()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now_utc()
		RETURNING user_id`, TokenHash(token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.BadRequest("VALIDATION_ERROR", "重置链接无效或已过期")
	}
	return userID, err
}

// SetEmailVerified 标记邮箱已验证并激活账号。
func (r *Repo) SetEmailVerified(ctx context.Context, tx pgx.Tx, userID int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE users
		SET email_verified_at = now_utc(),
		    status = CASE WHEN status = 'pending' THEN 'active' ELSE status END
		WHERE id = $1`, userID)
	return err
}

// UpdatePasswordTx 更新密码。
func (r *Repo) UpdatePasswordTx(ctx context.Context, tx pgx.Tx, userID int64, passwordHash string) error {
	_, err := tx.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now_utc() WHERE id = $1`, userID, passwordHash)
	return err
}

// IsEmailVerified 是否已验证。
func (r *Repo) IsEmailVerified(ctx context.Context, userID int64) (bool, error) {
	var v bool
	err := r.pool.QueryRow(ctx,
		`SELECT email_verified_at IS NOT NULL FROM users WHERE id = $1`, userID).Scan(&v)
	return v, err
}

// FindEmailForReset 返回邮箱对应的用户 ID（不存在时为 0）。
func (r *Repo) FindEmailForReset(ctx context.Context, email string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE email_norm = lower($1) AND deleted_at IS NULL`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// LogLogin 记录登录日志。
func (r *Repo) LogLogin(ctx context.Context, userID int64, success bool, ip, ua string) {
	_, _ = r.pool.Exec(ctx,
		`INSERT INTO login_logs (user_id, success, ip_address, user_agent) VALUES ($1, $2, $3, $4)`,
		userID, success, ip, ua)
}

// ---- 工具 ----

func randomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// TokenHash 令牌摘要（DB 只存摘要）。
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashPassword 生成 bcrypt 慢哈希。
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(b), err
}

// CheckPassword 校验密码。
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ValidateRegister 校验注册参数。
func ValidateRegister(username, email, password string) error {
	if !usernameRe.MatchString(username) {
		return apperr.BadRequest("VALIDATION_ERROR", "用户名需为 3-20 位字母、数字或下划线")
	}
	if !emailRe.MatchString(email) || len(email) > 254 {
		return apperr.BadRequest("VALIDATION_ERROR", "邮箱格式不正确")
	}
	if len(password) < 8 || len(password) > 72 {
		return apperr.BadRequest("VALIDATION_ERROR", "密码长度需为 8-72 位")
	}
	return nil
}

// EmitVerificationEmail 写入验证邮件 Outbox 事件。
func EmitVerificationEmail(ctx context.Context, tx pgx.Tx, email, username, token string) error {
	return outbox.EmitTx(ctx, tx, "email.verification", 0, map[string]string{
		"to": email, "username": username, "token": token,
	})
}

// EmitPasswordResetEmail 写入重置邮件 Outbox 事件。
func EmitPasswordResetEmail(ctx context.Context, tx pgx.Tx, email, username, token string) error {
	return outbox.EmitTx(ctx, tx, "email.password_reset", 0, map[string]string{
		"to": email, "username": username, "token": token,
	})
}

// EmitWelcomeEmail 写入欢迎邮件 Outbox 事件。
func EmitWelcomeEmail(ctx context.Context, tx pgx.Tx, email, username string) error {
	return outbox.EmitTx(ctx, tx, "email.welcome", 0, map[string]string{
		"to": email, "username": username,
	})
}

var _ = slog.Default // 保留 slog 引用（logger 由调用方注入）
