// Package points 积分/等级/勋章/签到。
// 红线：任何积分变动必须写流水（幂等键唯一约束兜底）；余额与流水同事务更新。
package points

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/markdown"
)

// 积分来源类型。
const (
	SourceRegistration   = "registration"
	SourceSignIn         = "sign_in"
	SourcePostCreated    = "post_created"
	SourcePostLiked      = "post_liked"
	SourceCommentLiked   = "comment_liked"
	SourceAnswerAccepted = "answer_accepted"
	SourceActivityAttend = "activity_attended"
	SourceBadge          = "badge"
	SourceAdminAdjust    = "admin_adjust"
)

// 默认积分规则（阶段 2 固定值；规则可配置化为后续优化项）。
const (
	PointsRegister       = 20
	PointsSignIn         = 5
	PointsPostCreated    = 10
	PointsPostLiked      = 1
	PointsCommentLiked   = 1
	PointsAnswerAccepted = 20
)

// Repo 积分数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Service 积分业务。
type Service struct {
	repo   *Repo
	pool   *pgxpool.Pool
	audit  *moderation.AuditLogger
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), pool: pool, logger: logger}
}

// SetAudit 注入审计日志（管理端积分调整/勋章操作用）。
func (s *Service) SetAudit(a *moderation.AuditLogger) { s.audit = a }

// GrantTx 在业务事务内发放/扣减积分（幂等：idempotencyKey 冲突时整体跳过）。
// 三步：1) 插入流水（幂等键冲突即跳过）→ 2) 账户余额原子增减（下限 0）→ 3) 回填流水余额快照。
func (s *Service) GrantTx(ctx context.Context, tx pgx.Tx, userID int64, sourceType string, sourceID int64, idempotencyKey string, amount int64, note string, operatorID int64) error {
	if amount == 0 {
		return nil
	}
	// 1. 幂等插入流水
	var txID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO point_transactions (user_id, amount, balance_after, source_type, source_id, idempotency_key, note, operator_id)
		VALUES ($1, $2, 0, $3, $4, $5, $6, NULLIF($7, 0))
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id`,
		userID, amount, sourceType, sourceID, idempotencyKey, note, operatorID).Scan(&txID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // 幂等跳过：同一业务事件已入账
	}
	if err != nil {
		return err
	}
	// 2. 原子更新账户
	var newBalance int64
	err = tx.QueryRow(ctx, `
		INSERT INTO point_accounts (user_id, balance, total_earned)
		VALUES ($1, GREATEST(0, $2), GREATEST(0, $2))
		ON CONFLICT (user_id) DO UPDATE SET
		  balance = GREATEST(0, point_accounts.balance + $2),
		  total_earned = point_accounts.total_earned + GREATEST(0, $2),
		  updated_at = now_utc()
		RETURNING balance`, userID, amount).Scan(&newBalance)
	if err != nil {
		return err
	}
	// 3. 回填余额快照（可校准）
	_, err = tx.Exec(ctx,
		`UPDATE point_transactions SET balance_after = $2 WHERE id = $1`, txID, newBalance)
	return err
}

// GrantWithKey 非事务入口（使用调用方指定幂等键，并可选写入管理审计）。
func (s *Service) GrantWithKey(ctx context.Context, userID int64, sourceType string, sourceID int64, idempotencyKey string, amount int64, note string, operatorID int64, auditDetail *moderation.Detail) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.GrantTx(ctx, tx, userID, sourceType, sourceID, idempotencyKey, amount, note, operatorID); err != nil {
		return apperr.Internal(err)
	}
	if auditDetail != nil && s.audit != nil {
		s.audit.LogInTx(ctx, tx, *auditDetail)
	}
	return tx.Commit(ctx)
}

// Grant 非事务入口（生成服务端幂等键）。
func (s *Service) Grant(ctx context.Context, userID int64, sourceType string, sourceID int64, amount int64, note string, operatorID int64) error {
	key := "svc:" + sourceType + ":" + itoa(sourceID) + ":" + itoa(userID) + ":" + itoa(amount)
	return s.GrantWithKey(ctx, userID, sourceType, sourceID, key, amount, note, operatorID, nil)
}

// Account 积分账户与等级。
type Account struct {
	Balance     int64      `json:"balance"`
	TotalEarned int64      `json:"total_earned"`
	Level       LevelInfo  `json:"level"`
	NextLevel   *LevelInfo `json:"next_level,omitempty"`
	SigninDays  int        `json:"signin_days"`
	StreakDays  int        `json:"streak_days"`
	TodaySigned bool       `json:"today_signed"`
}

// LevelInfo 等级信息。
type LevelInfo struct {
	Level     int    `json:"level"`
	Name      string `json:"name"`
	MinPoints int64  `json:"min_points"`
	Color     string `json:"color"`
}

// CalculateLevel 纯函数：按累计积分计算等级（单元测试覆盖）。
func CalculateLevel(levels []LevelInfo, total int64) (cur LevelInfo, next *LevelInfo) {
	cur = LevelInfo{Level: 0, Name: "新用户", MinPoints: 0}
	for i := range levels {
		if total >= levels[i].MinPoints {
			if levels[i].Level >= cur.Level {
				cur = levels[i]
			}
		}
	}
	for i := range levels {
		if levels[i].Level > cur.Level {
			cand := levels[i]
			if next == nil || cand.Level < next.Level {
				next = &cand
			}
		}
	}
	return cur, next
}

func (s *Service) loadLevels(ctx context.Context) ([]LevelInfo, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT level, name, min_points, color FROM levels ORDER BY min_points`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LevelInfo
	for rows.Next() {
		var l LevelInfo
		if err := rows.Scan(&l.Level, &l.Name, &l.MinPoints, &l.Color); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetAccount 查询账户（含等级/签到状态）。
func (s *Service) GetAccount(ctx context.Context, userID int64) (*Account, error) {
	var balance, earned int64
	err := s.pool.QueryRow(ctx,
		`SELECT balance, total_earned FROM point_accounts WHERE user_id = $1`, userID).
		Scan(&balance, &earned)
	if errors.Is(err, pgx.ErrNoRows) {
		balance, earned = 0, 0
	} else if err != nil {
		return nil, err
	}
	levels, err := s.loadLevels(ctx)
	if err != nil {
		return nil, err
	}
	cur, next := CalculateLevel(levels, earned)

	var signinDays int
	_ = s.pool.QueryRow(ctx,
		`SELECT COUNT(*)::int FROM sign_ins WHERE user_id = $1`, userID).Scan(&signinDays)
	streak := 0
	_ = s.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(cnt), 0)::int FROM (
			SELECT COUNT(*) AS cnt FROM (
				SELECT check_date, check_date - (ROW_NUMBER() OVER (ORDER BY check_date))::int AS grp
				FROM sign_ins WHERE user_id = $1 AND check_date <= CURRENT_DATE
			) g GROUP BY grp
		) c`, userID).Scan(&streak)
	var todaySigned bool
	_ = s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM sign_ins WHERE user_id = $1 AND check_date = CURRENT_DATE)`, userID).Scan(&todaySigned)

	return &Account{
		Balance: balance, TotalEarned: earned, Level: cur, NextLevel: next,
		SigninDays: signinDays, StreakDays: streak, TodaySigned: todaySigned,
	}, nil
}

// Transaction 流水行。
type Transaction struct {
	ID           int64     `json:"id"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	SourceType   string    `json:"source_type"`
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListTransactions 分页流水。
func (s *Service) ListTransactions(ctx context.Context, userID int64, limit, offset int) ([]Transaction, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM point_transactions WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, amount, balance_after, source_type, note, created_at
		FROM point_transactions WHERE user_id = $1
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.Amount, &t.BalanceAfter, &t.SourceType, &t.Note, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// DailyCheckIn 每日签到（数据库主键防重 + 积分幂等键双保险）。
func (s *Service) DailyCheckIn(ctx context.Context, userID int64) (awarded int64, already bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, false, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx,
		`INSERT INTO sign_ins (user_id, check_date, points) VALUES ($1, CURRENT_DATE, $2) ON CONFLICT DO NOTHING`,
		userID, PointsSignIn)
	if err != nil {
		return 0, false, apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return 0, true, nil
	}
	key := "signin:" + itoa(userID) + ":" + time.Now().UTC().Format("2006-01-02")
	if err := s.GrantTx(ctx, tx, userID, SourceSignIn, 0, key, PointsSignIn, "每日签到", 0); err != nil {
		return 0, false, apperr.Internal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, apperr.Internal(err)
	}
	s.logger.Info("user checked in", "user_id", userID)
	// 连续签到 7 天 → 自动授予「七日之约」勋章（幂等）
	acc, err := s.GetAccount(ctx, userID)
	if err == nil && acc.StreakDays >= 7 {
		btx, berr := s.pool.Begin(ctx)
		if berr == nil {
			_ = s.AwardBadgeTx(ctx, btx, userID, "streak_7", 0)
			_ = btx.Commit(ctx)
		}
	}
	return PointsSignIn, false, nil
}

// ---- 勋章 ----

// Badge 勋章定义。
type Badge struct {
	ID            int64     `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Icon          string    `json:"icon"`
	ConditionType string    `json:"condition_type"`
	IsEnabled     bool      `json:"is_enabled"`
	HolderCount   int64     `json:"holder_count,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ListBadges 全部勋章。
func (s *Service) ListBadges(ctx context.Context) ([]Badge, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.code, b.name, b.description, b.icon, b.condition_type, b.is_enabled, b.created_at,
		       (SELECT COUNT(*) FROM user_badges ub WHERE ub.badge_id = b.id) AS holders
		FROM badges b ORDER BY b.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Badge
	for rows.Next() {
		var b Badge
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Description, &b.Icon, &b.ConditionType, &b.IsEnabled, &b.CreatedAt, &b.HolderCount); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// AwardBadgeTx 授予勋章（幂等）。
func (s *Service) AwardBadgeTx(ctx context.Context, tx pgx.Tx, userID int64, code string, awardedBy int64) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO user_badges (user_id, badge_id, awarded_by)
		SELECT $1, id, NULLIF($2, 0) FROM badges WHERE code = $3 AND is_enabled = TRUE
		ON CONFLICT (user_id, badge_id) DO NOTHING`, userID, awardedBy, code)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return nil // 已持有或勋章不存在，静默幂等
	}
	return nil
}

// UserBadges 用户勋章列表。
func (s *Service) UserBadges(ctx context.Context, userID int64) ([]Badge, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.code, b.name, b.description, b.icon, b.condition_type, b.is_enabled, b.created_at, 0
		FROM user_badges ub JOIN badges b ON b.id = ub.badge_id
		WHERE ub.user_id = $1 ORDER BY ub.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Badge
	for rows.Next() {
		var b Badge
		if err := rows.Scan(&b.ID, &b.Code, &b.Name, &b.Description, &b.Icon, &b.ConditionType, &b.IsEnabled, &b.CreatedAt, &b.HolderCount); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---- 业务奖励钩子（供其他模块在事务内调用） ----

// RewardFirstLike 帖子首次被点赞 → 作者奖励（每帖一次）。
func (s *Service) RewardFirstLike(ctx context.Context, tx pgx.Tx, postID, authorID int64) error {
	return s.GrantTx(ctx, tx, authorID, SourcePostLiked, postID,
		"like_post:"+itoa(postID), PointsPostLiked, "帖子获得首个点赞", 0)
}

// RewardFirstCommentLike 评论首次被点赞 → 作者奖励（每条一次）。
func (s *Service) RewardFirstCommentLike(ctx context.Context, tx pgx.Tx, commentID, authorID int64) error {
	return s.GrantTx(ctx, tx, authorID, SourceCommentLiked, commentID,
		"like_comment:"+itoa(commentID), PointsCommentLiked, "评论获得首个点赞", 0)
}

// RewardAnswerAccepted 最佳回答被采纳 → 回答者奖励（每问答帖限领一次，防止反复切换最佳答案刷积分）。
func (s *Service) RewardAnswerAccepted(ctx context.Context, tx pgx.Tx, postID, commentID, authorID int64) error {
	return s.GrantTx(ctx, tx, authorID, SourceAnswerAccepted, commentID,
		"accept_post:"+itoa(postID), PointsAnswerAccepted, "回答被采纳为最佳答案", 0)
}

// RewardPostPublished 帖子发布奖励（每帖一次，publish 场景调用）。
func (s *Service) RewardPostPublished(ctx context.Context, tx pgx.Tx, postID, authorID int64) error {
	return s.GrantTx(ctx, tx, authorID, SourcePostCreated, postID,
		"post_created:"+itoa(postID), PointsPostCreated, "发布内容奖励", 0)
}

// ---- 等级工具 ----

var _ = strings.TrimSpace
var _ = markdown.Render
var _ = apperr.ErrForbidden

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
