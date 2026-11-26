// Package appeals 申诉：针对处罚/内容处理提交申诉，管理员处理（不覆盖原始处罚历史）。
package appeals

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Appeal 申诉单。
type Appeal struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Username    string     `json:"username"`
	TargetType  string     `json:"target_type"`
	TargetID    int64      `json:"target_id"`
	TargetLabel string     `json:"target_label"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	HandledBy   string     `json:"handled_by"`
	HandleNote  string     `json:"handle_note"`
	HandledAt   *time.Time `json:"handled_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Service 申诉业务。
type Service struct {
	pool   *pgxpool.Pool
	sanc   *moderation.SanctionService
	audit  *moderation.AuditLogger
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, sanc *moderation.SanctionService, audit *moderation.AuditLogger, logger *slog.Logger) *Service {
	return &Service{pool: pool, sanc: sanc, audit: audit, logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// Create 提交申诉（校验目标归属 + 开放申诉唯一）。
func (s *Service) Create(ctx context.Context, userID int64, targetType string, targetID int64, reason string) (int64, error) {
	if targetType != "sanction" && targetType != "moderation_action" {
		return 0, invalid("申诉类型不合法")
	}
	if len([]rune(reason)) < 5 || len([]rune(reason)) > 2000 {
		return 0, invalid("申诉理由需为 5-2000 字")
	}
	// 目标必须属于申诉人本人
	switch targetType {
	case "sanction":
		var owner int64
		err := s.pool.QueryRow(ctx,
			`SELECT user_id FROM user_sanctions WHERE id = $1`, targetID).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && owner != userID) {
			return 0, apperr.NotFound("处罚记录不存在")
		}
		if err != nil {
			return 0, apperr.Internal(err)
		}
	case "moderation_action":
		var owner *int64
		err := s.pool.QueryRow(ctx,
			`SELECT target_user_id FROM moderation_actions WHERE id = $1`, targetID).Scan(&owner)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (owner == nil || *owner != userID)) {
			return 0, apperr.NotFound("处理记录不存在")
		}
		if err != nil {
			return 0, apperr.Internal(err)
		}
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO appeals (user_id, target_type, target_id, reason)
		VALUES ($1, $2, $3, $4) RETURNING id`, userID, targetType, targetID, reason).Scan(&id)
	if err != nil && err.Error() == `ERROR: duplicate key value violates unique constraint "idx_appeals_open_unique" (SQLSTATE 23505)` {
		return 0, apperr.Conflict("CONFLICT", "该对象已有待处理的申诉，请耐心等待结果")
	}
	return id, err
}

// ListMine 我的申诉。
func (s *Service) ListMine(ctx context.Context, userID int64, limit, offset int) ([]Appeal, int64, error) {
	return s.list(ctx, " AND a.user_id = $1", []any{userID}, limit, offset)
}

// ListAll 后台全量。
func (s *Service) ListAll(ctx context.Context, status string, limit, offset int) ([]Appeal, int64, error) {
	args := []any{}
	where := ""
	if status != "" {
		args = append(args, status)
		where = " AND a.status = $1"
	}
	return s.list(ctx, where, args, limit, offset)
}

func (s *Service) list(ctx context.Context, extraWhere string, extraArgs []any, limit, offset int) ([]Appeal, int64, error) {
	where := `1=1` + extraWhere
	var total int64
	if err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM appeals a WHERE `+where, extraArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args := append(extraArgs, limit, offset)
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.user_id, COALESCE(p.display_name, u.username), a.target_type, a.target_id,
		       a.reason, a.status, COALESCE(hu.username, ''), a.handle_note, a.handled_at, a.created_at
		FROM appeals a
		JOIN users u ON u.id = a.user_id
		JOIN user_profiles p ON p.user_id = u.id
		LEFT JOIN users hu ON hu.id = a.handled_by
		WHERE `+where+`
		ORDER BY a.created_at DESC LIMIT $`+itoa(int64(len(args)-1))+` OFFSET $`+itoa(int64(len(args))), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Appeal
	for rows.Next() {
		var a Appeal
		if err := rows.Scan(&a.ID, &a.UserID, &a.Username, &a.TargetType, &a.TargetID,
			&a.Reason, &a.Status, &a.HandledBy, &a.HandleNote, &a.HandledAt, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		a.TargetLabel = s.targetLabel(ctx, a.TargetType, a.TargetID)
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (s *Service) targetLabel(ctx context.Context, targetType string, targetID int64) string {
	var label string
	switch targetType {
	case "sanction":
		_ = s.pool.QueryRow(ctx,
			`SELECT '处罚：' || sanction_type || ' ' || reason FROM user_sanctions WHERE id = $1`, targetID).Scan(&label)
	case "moderation_action":
		_ = s.pool.QueryRow(ctx,
			`SELECT '管理操作：' || action_type FROM moderation_actions WHERE id = $1`, targetID).Scan(&label)
	}
	if label == "" {
		return targetType + " #" + itoa(targetID)
	}
	return label
}

// Handle 处理申诉。accepted=true 时：对 sanction 类申诉解除对应处罚（保留历史记录）。
func (s *Service) Handle(ctx context.Context, adminID int64, appealID int64, accepted bool, note string) error {
	var targetType string
	var targetID, userID int64
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT target_type, target_id, user_id, status FROM appeals WHERE id = $1`, appealID).
		Scan(&targetType, &targetID, &userID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("申诉不存在")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if status != "pending" {
		return apperr.Conflict("CONFLICT", "该申诉已处理")
	}
	if userID == adminID {
		return apperr.Forbidden("不能审批自己的申诉")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	newStatus := "rejected"
	if accepted {
		newStatus = "accepted"
	}
	tag, err := tx.Exec(ctx, `
		UPDATE appeals SET status = $2, handled_by = $3, handle_note = $4, handled_at = now_utc()
		WHERE id = $1 AND status = 'pending'`, appealID, newStatus, adminID, note)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.Conflict("CONFLICT", "该申诉已处理")
	}
	// 申诉成立且目标为处罚 → 解除对应处罚，且仅在不存在其他生效封禁处罚时才恢复账号状态
	if accepted && targetType == "sanction" {
		if err := s.sanc.Lift(ctx, tx, targetID); err != nil {
			return err
		}
		var stillBanned bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_sanctions
				WHERE user_id = $1 AND status = 'active'
				  AND sanction_type IN ('perma_ban', 'temp_ban')
				  AND (ends_at IS NULL OR ends_at > now_utc())
			)`, userID).Scan(&stillBanned); err != nil {
			return apperr.Internal(err)
		}
		if !stillBanned {
			if _, err := tx.Exec(ctx, `
				UPDATE users SET status = 'active'
				WHERE id = $1 AND status IN ('banned','suspended')`, userID); err != nil {
				return apperr.Internal(err)
			}
		}
	}
	s.audit.LogInTx(ctx, tx, moderation.Detail{
		Action: "appeal.handle", TargetType: "appeal", TargetID: appealID,
		ActorID: adminID, RequestID: ctxRequestId(ctx), IP: ctxClientIP(ctx), Note: newStatus + " / " + note,
	})
	_ = notifications.EmitTx(ctx, tx, notifications.Payload{
		RecipientID: userID, ActorID: adminID, Type: notifications.TypeModeration,
		Title:     map[bool]string{true: "你的申诉已通过", false: "你的申诉未通过"}[accepted],
		Body:      note,
		DedupeKey: "appeal:" + itoa(appealID),
	})
	return tx.Commit(ctx)
}

type ctxKey string

const (
	ctxKeyRequestID ctxKey = "request_id"
	ctxKeyClientIP  ctxKey = "client_ip"
)

func ctxRequestId(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func ctxClientIP(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyClientIP).(string); ok {
		return v
	}
	return ""
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// ---- HTTP ----

// Handler 申诉 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterUser 用户侧路由。
func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireAuth())
	g.POST("/appeals", h.Create)
	g.GET("/me/appeals", h.ListMine)
	g.GET("/me/sanctions", h.MySanctions)
}

// RegisterAdmin 后台路由。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/appeals", h.AdminList)
	rg.POST("/appeals/:id/handle", h.Handle)
}

type createReq struct {
	TargetType string `json:"target_type"`
	TargetID   int64  `json:"target_id"`
	Reason     string `json:"reason"`
}

// Create POST /appeals
func (h *Handler) Create(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	id, err := h.svc.Create(c.Request.Context(), u.ID, req.TargetType, req.TargetID, req.Reason)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"id": id, "message": "申诉已提交，管理团队会尽快处理"})
}

// ListMine GET /me/appeals
func (h *Handler) ListMine(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.ListMine(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if items == nil {
		items = []Appeal{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// MySanctions GET /me/sanctions —— 我的处罚记录（申诉入口）。
func (h *Handler) MySanctions(c *gin.Context) {
	u := httpx.CurrentUser(c)
	list, err := h.svc.sanc.ListByUser(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if list == nil {
		list = []moderation.Sanction{}
	}
	respond.OK(c, gin.H{"items": list})
}

// AdminList GET /admin/appeals
func (h *Handler) AdminList(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.ListAll(c.Request.Context(), c.Query("status"), pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if items == nil {
		items = []Appeal{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// Handle POST /admin/appeals/:id/handle
func (h *Handler) Handle(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req struct {
		Accepted bool   `json:"accepted"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if req.Note == "" {
		respond.Fail(c, invalid("必须填写处理说明"))
		return
	}
	ctx := c.Request.Context()
	ctx = context.WithValue(ctx, ctxKeyRequestID, c.GetString("request_id"))
	ctx = context.WithValue(ctx, ctxKeyClientIP, c.ClientIP())
	if err := h.svc.Handle(ctx, u.ID, id, req.Accepted, req.Note); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "申诉已处理"})
}
