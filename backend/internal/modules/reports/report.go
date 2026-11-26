// Package reports 举报：提交（防重）、后台队列、分派与处理。
package reports

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
	"github.com/yanji-community/backend/internal/shared/rbac"
	"github.com/yanji-community/backend/internal/shared/respond"
)

var validReasons = map[string]bool{
	"spam": true, "harassment": true, "porn_violence": true, "illegal": true,
	"infringement": true, "misinformation": true, "flame": true, "other": true,
}

var validTargets = map[string]bool{"post": true, "comment": true, "user": true, "message": true}

// Report 举报记录。
type Report struct {
	ID             int64      `json:"id"`
	Reporter       string     `json:"reporter"`
	TargetType     string     `json:"target_type"`
	TargetLabel    string     `json:"target_label"`
	ReasonCode     string     `json:"reason_code"`
	Description    string     `json:"description"`
	Status         string     `json:"status"`
	Assignee       string     `json:"assignee"`
	ResolutionNote string     `json:"resolution_note"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Repo 举报数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// ResolveTarget 解析目标 public_id → 内部 ID 与摘要标签。
// 严格校验目标可见性，防止借举报枚举私有/隐藏/未发布内容或他人私信。
func (r *Repo) ResolveTarget(ctx context.Context, reporterID int64, targetType, targetPublicID string) (int64, string, error) {
	switch targetType {
	case "post":
		var id int64
		var title string
		err := r.pool.QueryRow(ctx, `
			SELECT p.id, p.title
			FROM posts p
			JOIN categories c ON c.id = p.category_id
			WHERE p.public_id = $1 AND p.deleted_at IS NULL
			  AND p.status IN ('published', 'locked')
			  AND c.status = 'active' AND c.visibility = 'public'`, targetPublicID).
			Scan(&id, &title)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", apperr.NotFound("举报目标不存在")
		}
		return id, "帖子：" + title, err
	case "comment":
		var id int64
		var body string
		err := r.pool.QueryRow(ctx, `
			SELECT cm.id, cm.body_markdown
			FROM comments cm
			JOIN posts p ON p.id = cm.post_id
			JOIN categories c ON c.id = p.category_id
			WHERE cm.public_id = $1 AND cm.status = 'published'
			  AND p.deleted_at IS NULL AND p.status IN ('published', 'locked')
			  AND c.status = 'active' AND c.visibility = 'public'`, targetPublicID).Scan(&id, &body)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", apperr.NotFound("举报目标不存在")
		}
		if len([]rune(body)) > 40 {
			body = string([]rune(body)[:40]) + "…"
		}
		return id, "评论：" + body, err
	case "user":
		var id int64
		var username string
		err := r.pool.QueryRow(ctx,
			`SELECT id, username FROM users WHERE public_id = $1 AND deleted_at IS NULL`, targetPublicID).
			Scan(&id, &username)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", apperr.NotFound("举报目标不存在")
		}
		return id, "用户：" + username, err
	case "message":
		var id int64
		var body string
		err := r.pool.QueryRow(ctx, `
			SELECT m.id, m.body_text
			FROM messages m
			JOIN conversation_members cm ON cm.conversation_id = m.conversation_id AND cm.user_id = $2
			WHERE m.id::text = $1 AND m.status = 'sent'`, targetPublicID, reporterID).Scan(&id, &body)
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", apperr.NotFound("举报目标不存在")
		}
		if len([]rune(body)) > 40 {
			body = string([]rune(body)[:40]) + "…"
		}
		return id, "私信：" + body, err
	}
	return 0, "", apperr.BadRequest("VALIDATION_ERROR", "举报类型不合法")
}

// Create 提交举报（处理中的同目标同举报者不重复受理）。
func (r *Repo) Create(ctx context.Context, reporterID int64, targetType string, targetID int64, reasonCode, description string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO reports (reporter_id, target_type, target_id, reason_code, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, reporterID, targetType, targetID, reasonCode, description).Scan(&id)
	if err != nil && err.Error() == `ERROR: duplicate key value violates unique constraint "idx_reports_open_unique" (SQLSTATE 23505)` {
		return 0, apperr.Conflict("CONFLICT", "你已提交过该内容的举报，正在处理中")
	}
	return id, err
}

// List 后台举报列表。allowedCategoryIDs 为 nil 表示全局管理员，否则仅限授权版块内的帖子/评论举报。
func (r *Repo) List(ctx context.Context, status string, allowedCategoryIDs []int64, limit, offset int) ([]Report, int64, error) {
	where := `1=1`
	args := []any{}
	if status != "" {
		args = append(args, status)
		where += ` AND rp.status = $` + strconv.Itoa(len(args))
	}
	if allowedCategoryIDs != nil {
		args = append(args, allowedCategoryIDs)
		idx := strconv.Itoa(len(args))
		where += ` AND (
			(rp.target_type = 'post' AND EXISTS(SELECT 1 FROM posts p WHERE p.id = rp.target_id AND p.category_id = ANY($` + idx + `)))
			OR (rp.target_type = 'comment' AND EXISTS(SELECT 1 FROM comments cm JOIN posts p ON p.id = cm.post_id WHERE cm.id = rp.target_id AND p.category_id = ANY($` + idx + `)))
		)`
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM reports rp WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT rp.id, COALESCE(ru.username, ''), rp.target_type, rp.target_id,
		       COALESCE(rp.reason_code, ''), COALESCE(rp.description, ''), rp.status,
		       COALESCE(au.username, ''), COALESCE(rp.resolution_note, ''), rp.resolved_at, rp.created_at
		FROM reports rp
		LEFT JOIN users ru ON ru.id = rp.reporter_id
		LEFT JOIN users au ON au.id = rp.assignee_id
		WHERE `+where+`
		ORDER BY rp.created_at DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	type rawRow struct {
		rp       Report
		targetID int64
	}
	var raw []rawRow
	for rows.Next() {
		var rr rawRow
		var reasonCode string
		if err := rows.Scan(&rr.rp.ID, &rr.rp.Reporter, &rr.rp.TargetType, &rr.targetID, &reasonCode, &rr.rp.Description,
			&rr.rp.Status, &rr.rp.Assignee, &rr.rp.ResolutionNote, &rr.rp.ResolvedAt, &rr.rp.CreatedAt); err != nil {
			return nil, 0, err
		}
		rr.rp.ReasonCode = reasonCode
		raw = append(raw, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	out := make([]Report, 0, len(raw))
	for _, rr := range raw {
		rr.rp.TargetLabel = targetLabel(ctx, r.pool, rr.rp.TargetType, rr.targetID)
		out = append(out, rr.rp)
	}
	return out, total, nil
}

// Get 单条举报（含目标摘要）。
func (r *Repo) Get(ctx context.Context, id int64) (*Report, error) {
	var rp Report
	var targetID int64
	err := r.pool.QueryRow(ctx, `
		SELECT rp.id, COALESCE(ru.username, ''), rp.target_type, rp.target_id,
		       rp.reason_code, COALESCE(rp.description, ''), rp.status,
		       COALESCE(au.username, ''), COALESCE(rp.resolution_note, ''), rp.resolved_at, rp.created_at
		FROM reports rp
		LEFT JOIN users ru ON ru.id = rp.reporter_id
		LEFT JOIN users au ON au.id = rp.assignee_id
		WHERE rp.id = $1`, id).
		Scan(&rp.ID, &rp.Reporter, &rp.TargetType, &targetID, &rp.ReasonCode, &rp.Description,
			&rp.Status, &rp.Assignee, &rp.ResolutionNote, &rp.ResolvedAt, &rp.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("举报不存在")
	}
	if err != nil {
		return nil, err
	}
	rp.TargetLabel = targetLabel(ctx, r.pool, rp.TargetType, targetID)
	return &rp, nil
}

func targetLabel(ctx context.Context, pool *pgxpool.Pool, targetType string, targetID int64) string {
	var label string
	switch targetType {
	case "post":
		_ = pool.QueryRow(ctx, `SELECT '帖子：' || title FROM posts WHERE id = $1`, targetID).Scan(&label)
	case "comment":
		var body string
		_ = pool.QueryRow(ctx, `SELECT body_markdown FROM comments WHERE id = $1`, targetID).Scan(&body)
		if len([]rune(body)) > 40 {
			body = string([]rune(body)[:40]) + "…"
		}
		label = "评论：" + body
	case "user":
		_ = pool.QueryRow(ctx, `SELECT '用户：' || username FROM users WHERE id = $1`, targetID).Scan(&label)
	case "message":
		var body string
		_ = pool.QueryRow(ctx, `SELECT body_text FROM messages WHERE id = $1`, targetID).Scan(&body)
		if len([]rune(body)) > 40 {
			body = string([]rune(body)[:40]) + "…"
		}
		label = "私信：" + body
	}
	return label
}

// Assign 分派。
func (r *Repo) Assign(ctx context.Context, id, assigneeID int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reports SET status = 'processing', assignee_id = $2, updated_at = now_utc()
		 WHERE id = $1 AND status IN ('pending','processing')`, id, assigneeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.Conflict("CONFLICT", "举报不存在或已处理完成")
	}
	return nil
}

// Resolve 处理举报：resolution = action_taken | dismissed。
func (r *Repo) Resolve(ctx context.Context, id, operatorID int64, resolution, note string) error {
	status := "resolved"
	if resolution == "dismissed" {
		status = "dismissed"
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE reports SET status = $2, resolution_note = $3, assignee_id = $4,
		       resolved_at = now_utc(), updated_at = now_utc()
		WHERE id = $1 AND status IN ('pending','processing')`, id, status, note, operatorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.Conflict("CONFLICT", "举报不存在或已处理完成")
	}
	return nil
}

// TargetAuthor 举报目标作者（处理结果通知）。
func (r *Repo) TargetAuthor(ctx context.Context, targetType string, targetID int64) (int64, string) {
	var authorID int64
	var title string
	switch targetType {
	case "post":
		_ = r.pool.QueryRow(ctx, `SELECT author_id, title FROM posts WHERE id = $1`, targetID).
			Scan(&authorID, &title)
	case "comment":
		_ = r.pool.QueryRow(ctx, `SELECT author_id, '你的评论' FROM comments WHERE id = $1`, targetID).
			Scan(&authorID, &title)
	default:
		return 0, ""
	}
	return authorID, title
}

// ---- Service / HTTP ----

// Service 举报业务。
type Service struct {
	repo    *Repo
	rbacSvc *rbac.Service
	audit   *moderation.AuditLogger
	logger  *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, rbacSvc *rbac.Service, audit *moderation.AuditLogger, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), rbacSvc: rbacSvc, audit: audit, logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// Handler 举报 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterUser 用户侧路由。
func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	rg.POST("/reports", httpx.RequireAuth(), httpx.RequireVerified(), h.Create)
}

// RegisterAdmin 后台路由。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/reports", h.List)
	rg.POST("/reports/:id/assign", h.Assign)
	rg.POST("/reports/:id/resolve", h.Resolve)
}

type createReq struct {
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	ReasonCode  string `json:"reason_code"`
	Description string `json:"description"`
}

// Create POST /reports
func (h *Handler) Create(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if !validTargets[req.TargetType] {
		respond.Fail(c, invalid("举报类型不合法"))
		return
	}
	if !validReasons[req.ReasonCode] {
		respond.Fail(c, invalid("举报原因不合法"))
		return
	}
	if len([]rune(req.Description)) > 1000 {
		respond.Fail(c, invalid("描述最长 1000 字"))
		return
	}
	targetID, _, err := h.svc.repo.ResolveTarget(c.Request.Context(), u.ID, req.TargetType, req.TargetID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	id, err := h.svc.repo.Create(c.Request.Context(), u.ID, req.TargetType, targetID, req.ReasonCode, req.Description)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"id": id, "message": "举报已提交，我们会尽快处理"})
}

// checkScope 校验操作者是否有权受理/处理指定举报：全局 report.handle 放行；
// 单版块版主仅允许处理授权版块内的 post/comment 举报，禁止访问 user/message 举报。
func (s *Service) checkScope(ctx context.Context, actorID, reportID int64) error {
	var targetType string
	var targetID int64
	err := s.repo.pool.QueryRow(ctx, `SELECT target_type, target_id FROM reports WHERE id = $1`, reportID).
		Scan(&targetType, &targetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("举报不存在")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	global, err := s.rbacSvc.HasGlobalPermission(ctx, actorID, "report.handle")
	if err != nil {
		return apperr.Internal(err)
	}
	if global {
		return nil
	}
	var categoryID int64
	switch targetType {
	case "post":
		if err := s.repo.pool.QueryRow(ctx, `SELECT category_id FROM posts WHERE id = $1`, targetID).Scan(&categoryID); err != nil {
			return apperr.Forbidden("无权处理该举报")
		}
	case "comment":
		if err := s.repo.pool.QueryRow(ctx, `
			SELECT p.category_id FROM comments cm JOIN posts p ON p.id = cm.post_id
			WHERE cm.id = $1`, targetID).Scan(&categoryID); err != nil {
			return apperr.Forbidden("无权处理该举报")
		}
	default:
		// 私信与用户举报仅全局管理员可处理
		return apperr.Forbidden("该类举报仅限全局管理员处理")
	}
	can, err := s.rbacSvc.CanModerateCategory(ctx, actorID, categoryID)
	if err != nil {
		return apperr.Internal(err)
	}
	if !can {
		return apperr.Forbidden("只能处理授权版块内的举报")
	}
	return nil
}

// List GET /admin/reports
func (h *Handler) List(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var allowedCats []int64
	global, err := h.svc.rbacSvc.HasGlobalPermission(c.Request.Context(), u.ID, "report.handle")
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if !global {
		ids, err := h.svc.rbacSvc.ModeratedCategoryIDs(c.Request.Context(), u.ID)
		if err != nil {
			respond.Fail(c, apperr.Internal(err))
			return
		}
		if len(ids) == 0 {
			respond.Page(c, []Report{}, pq.Page, pq.PageSize, 0)
			return
		}
		allowedCats = ids
	}
	items, total, err := h.svc.repo.List(c.Request.Context(), c.Query("status"), allowedCats, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if items == nil {
		items = []Report{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// Assign POST /admin/reports/:id/assign
func (h *Handler) Assign(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	if err := h.svc.checkScope(c.Request.Context(), u.ID, id); err != nil {
		respond.Fail(c, err)
		return
	}
	if err := h.svc.repo.Assign(c.Request.Context(), id, u.ID); err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "report.assign", TargetType: "report", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "已受理该举报"})
}

type resolveReq struct {
	Resolution string `json:"resolution"` // action_taken | dismissed
	Note       string `json:"note"`
	// 附带处置动作（可选）：hide_post / delete_comment / warn_user
	HandleAction string `json:"handle_action"`
}

// Resolve POST /admin/reports/:id/resolve
func (h *Handler) Resolve(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	if err := h.svc.checkScope(c.Request.Context(), u.ID, id); err != nil {
		respond.Fail(c, err)
		return
	}
	var req resolveReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if req.Resolution != "action_taken" && req.Resolution != "dismissed" {
		respond.Fail(c, invalid("resolution 取值不合法"))
		return
	}
	if req.Resolution == "action_taken" && req.Note == "" {
		respond.Fail(c, invalid("处理举报时必须填写处理说明"))
		return
	}
	rp, err := h.svc.repo.Get(c.Request.Context(), id)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if err := h.svc.repo.Resolve(c.Request.Context(), id, u.ID, req.Resolution, req.Note); err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "report.resolve", TargetType: "report", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
		Note: req.Resolution + " / " + req.Note,
	})
	// 通知举报目标作者
	if rp.TargetType == "post" || rp.TargetType == "comment" {
		var targetID int64
		_ = h.svc.repo.pool.QueryRow(c.Request.Context(),
			`SELECT target_id FROM reports WHERE id = $1`, id).Scan(&targetID)
		authorID, title := h.svc.repo.TargetAuthor(c.Request.Context(), rp.TargetType, targetID)
		if authorID > 0 && authorID != u.ID {
			if tx, txErr := h.svc.repo.pool.Begin(c.Request.Context()); txErr == nil {
				defer tx.Rollback(c.Request.Context())
				msg := "你在「" + title + "」中的内容经举报核实已被处理"
				if req.Resolution == "dismissed" {
					msg = "针对你内容的举报经核实不成立"
				}
				if emitErr := notifications.EmitTx(c.Request.Context(), tx, notifications.Payload{
					RecipientID: authorID, ActorID: u.ID, Type: notifications.TypeModeration,
					ResourceType: rp.TargetType, ResourceID: targetID,
					Title: msg, Body: req.Note,
					DedupeKey: "reportresolve:" + strconv.FormatInt(id, 10),
				}); emitErr == nil {
					_ = tx.Commit(c.Request.Context())
				}
			}
		}
	}
	respond.OK(c, gin.H{"message": "举报已处理"})
}
