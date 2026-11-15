// Package moderation 治理能力：敏感词检查、用户处罚、审计日志。
package moderation

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ---- 敏感词 ----

// Word 敏感词。
type Word struct {
	ID        int64     `json:"id"`
	Word      string    `json:"word"`
	MatchType string    `json:"match_type"`
	Action    string    `json:"action"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// Gate 敏感词网关（60s 缓存）。
type Gate struct {
	pool   *pgxpool.Pool
	mu     sync.RWMutex
	words  []Word
	loaded time.Time
}

// NewGate 构造。
func NewGate(pool *pgxpool.Pool) *Gate { return &Gate{pool: pool} }

func (g *Gate) load(ctx context.Context) error {
	g.mu.RLock()
	if time.Since(g.loaded) < 60*time.Second {
		defer g.mu.RUnlock()
		return nil
	}
	g.mu.RUnlock()

	rows, err := g.pool.Query(ctx,
		`SELECT id, word, match_type, action, is_enabled, created_at FROM sensitive_words WHERE is_enabled = TRUE`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var words []Word
	for rows.Next() {
		var w Word
		if err := rows.Scan(&w.ID, &w.Word, &w.MatchType, &w.Action, &w.IsEnabled, &w.CreatedAt); err != nil {
			return err
		}
		words = append(words, w)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	g.mu.Lock()
	g.words, g.loaded = words, time.Now()
	g.mu.Unlock()
	return nil
}

// Check 检查文本：返回 ""（通过）/ "block"（拒绝）/ "review"（转审核）。
func (g *Gate) Check(ctx context.Context, text string) (string, error) {
	if err := g.load(ctx); err != nil {
		return "", err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	lower := strings.ToLower(text)
	action := ""
	for _, w := range g.words {
		var hit bool
		if w.MatchType == "exact" {
			hit = strings.TrimSpace(lower) == strings.ToLower(w.Word)
		} else {
			hit = strings.Contains(lower, strings.ToLower(w.Word))
		}
		if !hit {
			continue
		}
		if w.Action == "block" {
			return "block", nil // block 优先级最高，直接短路
		}
		action = "review"
	}
	return action, nil
}

// Invalidate 清空敏感词缓存（后台修改后调用）。
func (g *Gate) Invalidate() {
	g.mu.Lock()
	g.loaded = time.Time{}
	g.mu.Unlock()
}

// ---- 处罚 ----

// SanctionService 用户处罚。
type SanctionService struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewSanctionService 构造。
func NewSanctionService(pool *pgxpool.Pool, logger *slog.Logger) *SanctionService {
	return &SanctionService{pool: pool, logger: logger}
}

// Sanction 处罚记录。
type Sanction struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	Type      string     `json:"sanction_type"`
	Reason    string     `json:"reason"`
	StartsAt  time.Time  `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	Status    string     `json:"status"`
	Operator  int64      `json:"operator_id"`
	CreatedAt time.Time  `json:"created_at"`
	LiftedAt  *time.Time `json:"lifted_at"`
}

// ValidType 校验处罚类型并返回是否永久。
var validTypes = map[string]bool{"warn": true, "mute": true, "post_ban": true, "temp_ban": true, "perma_ban": true}

// Create 创建处罚（事务内调用；调用方负责审计与通知）。
func (s *SanctionService) Create(ctx context.Context, tx pgx.Tx, userID int64, sanctionType, reason string, duration time.Duration, operatorID int64, sourceReportID int64) (int64, error) {
	if !validTypes[sanctionType] {
		return 0, errInvalid("处罚类型不合法")
	}
	var endsAt *time.Time
	if duration > 0 && sanctionType != "perma_ban" {
		t := time.Now().UTC().Add(duration)
		endsAt = &t
	}
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO user_sanctions (user_id, sanction_type, reason, ends_at, operator_id, source_report_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, 0))
		RETURNING id`, userID, sanctionType, reason, endsAt, operatorID, sourceReportID).Scan(&id)
	return id, err
}

// Lift 解除处罚。
func (s *SanctionService) Lift(ctx context.Context, tx pgx.Tx, sanctionID int64) error {
	tag, err := tx.Exec(ctx, `
		UPDATE user_sanctions SET status = 'lifted', lifted_at = now_utc()
		WHERE id = $1 AND status = 'active'`, sanctionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errInvalid("处罚不存在或已解除")
	}
	return nil
}

// ListByUser 用户处罚记录。
func (s *SanctionService) ListByUser(ctx context.Context, userID int64) ([]Sanction, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, sanction_type, reason, starts_at, ends_at, status, operator_id, created_at, lifted_at
		FROM user_sanctions WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Sanction
	for rows.Next() {
		var sc Sanction
		if err := rows.Scan(&sc.ID, &sc.UserID, &sc.Type, &sc.Reason, &sc.StartsAt, &sc.EndsAt,
			&sc.Status, &sc.Operator, &sc.CreatedAt, &sc.LiftedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// ListAll 处罚列表（管理后台）。
func (s *SanctionService) ListAll(ctx context.Context, limit, offset int) ([]map[string]any, int64, error) {
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_sanctions`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT us.id, u.username, us.sanction_type, us.reason, us.starts_at, us.ends_at, us.status,
		       COALESCE(op.username, ''), us.created_at, us.lifted_at
		FROM user_sanctions us
		JOIN users u ON u.id = us.user_id
		JOIN users op ON op.id = us.operator_id
		ORDER BY us.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, opIgnore int64
		var username, stype, reason, status, operator string
		var startsAt time.Time
		var endsAt, lifted *time.Time
		if err := rows.Scan(&id, &username, &stype, &reason, &startsAt, &endsAt, &status, &operator, &opIgnore, &lifted); err != nil {
			return nil, 0, err
		}
		out = append(out, map[string]any{
			"id": id, "username": username, "sanction_type": stype, "reason": reason,
			"starts_at": startsAt, "ends_at": endsAt, "status": status, "operator": operator, "lifted_at": lifted,
		})
	}
	return out, total, rows.Err()
}

// ExpireDue 将过期处罚标记为 expired（worker 周期调用）。
func (s *SanctionService) ExpireDue(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE user_sanctions SET status = 'expired'
		WHERE status = 'active' AND ends_at IS NOT NULL AND ends_at <= now_utc()`)
	return tag.RowsAffected(), err
}

// ---- 审计日志 ----

// AuditLogger 重要操作审计。
type AuditLogger struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewAuditLogger 构造。
func NewAuditLogger(pool *pgxpool.Pool, logger *slog.Logger) *AuditLogger {
	return &AuditLogger{pool: pool, logger: logger}
}

// Detail 审计详情。
type Detail struct {
	Action     string
	TargetType string
	TargetID   int64
	ActorID    int64
	RequestID  string
	IP         string
	Note       string
}

// Log 记录审计日志（独立连接写入；审计失败只记录日志不阻断业务）。
func (a *AuditLogger) Log(ctx context.Context, d Detail) {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO operation_logs (actor_id, action, target_type, target_id, request_id, detail, ip_address)
		VALUES (NULLIF($1,0), $2, $3, NULLIF($4,0), $5,
		        jsonb_build_object('note', $6), $7)`,
		d.ActorID, d.Action, d.TargetType, d.TargetID, d.RequestID, d.Note, d.IP)
	if err != nil {
		a.logger.Error("审计日志写入失败", "action", d.Action, "err", err)
	}
}

// LogInTx 在业务事务内记录（保证与业务同生共死）。
func (a *AuditLogger) LogInTx(ctx context.Context, tx pgx.Tx, d Detail) {
	_, err := tx.Exec(ctx, `
		INSERT INTO operation_logs (actor_id, action, target_type, target_id, request_id, detail, ip_address)
		VALUES (NULLIF($1,0), $2, $3, NULLIF($4,0), $5, jsonb_build_object('note', $6), $7)`,
		d.ActorID, d.Action, d.TargetType, d.TargetID, d.RequestID, d.Note, d.IP)
	if err != nil {
		a.logger.Error("审计日志写入失败(tx)", "action", d.Action, "err", err)
	}
}

func errInvalid(msg string) error {
	return errInvalidImpl(msg)
}
