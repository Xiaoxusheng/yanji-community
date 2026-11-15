// Package admin 管理后台聚合接口：仪表盘、用户、处罚、敏感词、公告、系统设置、审计日志、角色。
package admin

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

// Service 管理后台业务。
type Service struct {
	pool     *pgxpool.Pool
	rbacSvc  *rbac.Service
	sanction *moderation.SanctionService
	gate     *moderation.Gate
	audit    *moderation.AuditLogger
	logger   *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, rbacSvc *rbac.Service, sanction *moderation.SanctionService,
	gate *moderation.Gate, audit *moderation.AuditLogger, logger *slog.Logger) *Service {
	return &Service{pool: pool, rbacSvc: rbacSvc, sanction: sanction, gate: gate, audit: audit, logger: logger}
}

// Handler 管理后台 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register 注册后台路由（每个子组由装配层包上对应权限中间件）。
func (h *Handler) Register(rg *gin.RouterGroup) {
	// dashboard.read
	dash := rg.Group("/dashboard")
	dash.GET("", h.Dashboard)

	// user.read / user.sanction / role.*
	users := rg.Group("/users")
	users.GET("", h.Users)
	users.GET("/:id", h.UserDetail)
	roles := rg.Group("/users/:id/roles")
	roles.POST("", h.AssignRole)
	roles.DELETE("", h.RemoveRole)
	sanctions := rg.Group("/users/:id/sanctions")
	sanctions.POST("", h.CreateSanction)
	sanctions.DELETE("/:sanctionId", h.LiftSanction)
	users.POST("/:id/unban", h.Unban)

	rg.GET("/sanctions", h.Sanctions)

	// moderation.manage
	swords := rg.Group("/sensitive-words")
	swords.GET("", h.ListSensitiveWords)
	swords.POST("", h.CreateSensitiveWord)
	swords.PATCH("/:id", h.UpdateSensitiveWord)
	swords.DELETE("/:id", h.DeleteSensitiveWord)

	// announcement.manage
	ann := rg.Group("/announcements")
	ann.GET("", h.ListAnnouncements)
	ann.POST("", h.CreateAnnouncement)
	ann.PATCH("/:id", h.UpdateAnnouncement)
	ann.DELETE("/:id", h.DeleteAnnouncement)

	// settings.manage / audit.read
	rg.GET("/settings", h.GetSettings)
	rg.PATCH("/settings", h.UpdateSettings)
	rg.GET("/operation-logs", h.OperationLogs)
	rg.GET("/login-logs", h.LoginLogs)

	// role.read / role.assign
	rg.GET("/roles", h.Roles)
	rg.GET("/permissions", h.Permissions)
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// ---- 仪表盘 ----

// Dashboard GET /admin/dashboard
func (h *Handler) Dashboard(c *gin.Context) {
	ctx := c.Request.Context()
	var usersTotal, usersToday, postsTotal, postsToday, commentsTotal, commentsToday,
		pendingPosts, pendingReports, activeSanctions, onlineSessions int64

	q := func(dst *int64, sql string) {
		_ = h.svc.pool.QueryRow(ctx, sql).Scan(dst)
	}
	q(&usersTotal, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`)
	q(&usersToday, `SELECT COUNT(*) FROM users WHERE created_at >= date_trunc('day', now_utc())`)
	q(&postsTotal, `SELECT COUNT(*) FROM posts WHERE deleted_at IS NULL AND status <> 'draft'`)
	q(&postsToday, `SELECT COUNT(*) FROM posts WHERE created_at >= date_trunc('day', now_utc()) AND status <> 'draft'`)
	q(&commentsTotal, `SELECT COUNT(*) FROM comments WHERE deleted_at IS NULL`)
	q(&commentsToday, `SELECT COUNT(*) FROM comments WHERE created_at >= date_trunc('day', now_utc())`)
	q(&pendingPosts, `SELECT COUNT(*) FROM posts WHERE status = 'pending_review' AND deleted_at IS NULL`)
	q(&pendingReports, `SELECT COUNT(*) FROM reports WHERE status IN ('pending','processing')`)
	q(&activeSanctions, `SELECT COUNT(*) FROM user_sanctions WHERE status = 'active' AND (ends_at IS NULL OR ends_at > now_utc())`)
	q(&onlineSessions, `SELECT COUNT(*) FROM user_sessions WHERE revoked_at IS NULL AND expires_at > now_utc() AND last_seen_at > now_utc() - interval '24 hours'`)

	// 7 日趋势
	type trend struct {
		Day      string `json:"day"`
		Posts    int64  `json:"posts"`
		Users    int64  `json:"users"`
		Comments int64  `json:"comments"`
	}
	rows, err := h.svc.pool.Query(ctx, `
		WITH days AS (
			SELECT generate_series(date_trunc('day', now_utc()) - interval '6 days', date_trunc('day', now_utc()), interval '1 day') AS d
		)
		SELECT to_char(d, 'MM-DD'),
		  (SELECT COUNT(*) FROM posts WHERE created_at >= d AND created_at < d + interval '1 day' AND status <> 'draft'),
		  (SELECT COUNT(*) FROM users WHERE created_at >= d AND created_at < d + interval '1 day'),
		  (SELECT COUNT(*) FROM comments WHERE created_at >= d AND created_at < d + interval '1 day')
		FROM days ORDER BY d`)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	trends := []trend{}
	for rows.Next() {
		var t trend
		if err := rows.Scan(&t.Day, &t.Posts, &t.Users, &t.Comments); err != nil {
			respond.Fail(c, err)
			return
		}
		trends = append(trends, t)
	}
	respond.OK(c, gin.H{
		"totals": gin.H{
			"users": usersTotal, "users_today": usersToday,
			"posts": postsTotal, "posts_today": postsToday,
			"comments": commentsTotal, "comments_today": commentsToday,
			"online_sessions": onlineSessions,
		},
		"queues": gin.H{
			"pending_posts": pendingPosts, "pending_reports": pendingReports, "active_sanctions": activeSanctions,
		},
		"trends": trends,
	})
}

// ---- 用户管理 ----

// UserRow 用户管理行。
type UserRow struct {
	ID            int64      `json:"id"`
	PublicID      string     `json:"public_id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	Status        string     `json:"status"`
	EmailVerified bool       `json:"email_verified"`
	DisplayName   string     `json:"display_name"`
	PostCount     int64      `json:"post_count"`
	CreatedAt     time.Time  `json:"created_at"`
	LastLoginAt   *time.Time `json:"last_login_at"`
}

// Users GET /admin/users?status=&q=
func (h *Handler) Users(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	where := `u.deleted_at IS NULL`
	args := []any{}
	if s := c.Query("status"); s != "" {
		args = append(args, s)
		where += ` AND u.status = $` + strconv.Itoa(len(args))
	}
	if qs := c.Query("q"); qs != "" {
		args = append(args, "%"+qs+"%")
		where += ` AND (u.username ILIKE $` + strconv.Itoa(len(args)) + ` OR u.email_norm ILIKE $` + strconv.Itoa(len(args)) + `)`
	}
	var total int64
	if err := h.svc.pool.QueryRow(c.Request.Context(),
		`SELECT COUNT(*) FROM users u WHERE `+where, args...).Scan(&total); err != nil {
		respond.Fail(c, err)
		return
	}
	args = append(args, pq.PageSize, pq.Offset)
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT u.id, u.public_id::text, u.username, u.email_norm, u.status,
		       (u.email_verified_at IS NOT NULL), COALESCE(p.display_name, u.username),
		       (SELECT COUNT(*) FROM posts WHERE author_id = u.id AND deleted_at IS NULL AND status <> 'draft'),
		       u.created_at, u.last_login_at
		FROM users u JOIN user_profiles p ON p.user_id = u.id
		WHERE `+where+`
		ORDER BY u.created_at DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	items := []UserRow{}
	for rows.Next() {
		var it UserRow
		if err := rows.Scan(&it.ID, &it.PublicID, &it.Username, &it.Email, &it.Status,
			&it.EmailVerified, &it.DisplayName, &it.PostCount, &it.CreatedAt, &it.LastLoginAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, it)
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// UserDetail GET /admin/users/:id
func (h *Handler) UserDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	ctx := c.Request.Context()
	var row UserRow
	var bio, website, location string
	err = h.svc.pool.QueryRow(ctx, `
		SELECT u.id, u.public_id::text, u.username, u.email_norm, u.status,
		       (u.email_verified_at IS NOT NULL), COALESCE(p.display_name, u.username),
		       (SELECT COUNT(*) FROM posts WHERE author_id = u.id AND deleted_at IS NULL AND status <> 'draft'),
		       u.created_at, u.last_login_at, p.bio, p.website, p.location
		FROM users u JOIN user_profiles p ON p.user_id = u.id
		WHERE u.id = $1`, id).
		Scan(&row.ID, &row.PublicID, &row.Username, &row.Email, &row.Status, &row.EmailVerified,
			&row.DisplayName, &row.PostCount, &row.CreatedAt, &row.LastLoginAt, &bio, &website, &location)
	if errors.Is(err, pgx.ErrNoRows) {
		respond.Fail(c, apperr.NotFound("用户不存在"))
		return
	}
	if err != nil {
		respond.Fail(c, err)
		return
	}
	roles, _ := h.svc.rbacSvc.UserRoles(ctx, id)
	sanctions, _ := h.svc.sanction.ListByUser(ctx, id)
	respond.OK(c, gin.H{
		"user": row, "bio": bio, "website": website, "location": location,
		"roles": roles, "sanctions": sanctions,
	})
}

// ---- 处罚 ----

type sanctionReq struct {
	Type   string `json:"sanction_type"`
	Reason string `json:"reason"`
	Days   int    `json:"days"` // 0=永久（perma_ban/warn 无需）
}

// CreateSanction POST /admin/users/:id/sanctions
func (h *Handler) CreateSanction(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req sanctionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if req.Reason == "" {
		respond.Fail(c, invalid("处罚必须填写原因"))
		return
	}
	var targetPublicID string
	if err := h.svc.pool.QueryRow(c.Request.Context(),
		`SELECT public_id::text FROM users WHERE id = $1`, id).Scan(&targetPublicID); err != nil {
		respond.Fail(c, apperr.NotFound("用户不存在"))
		return
	}
	tx, err := h.svc.pool.Begin(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(c.Request.Context())
	var duration time.Duration
	if req.Days > 0 {
		duration = time.Duration(req.Days) * 24 * time.Hour
	}
	sid, err := h.svc.sanction.Create(c.Request.Context(), tx, id, req.Type, req.Reason, duration, u.ID, 0)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	// 永久封禁同时更新用户状态
	if req.Type == "perma_ban" {
		if _, err := tx.Exec(c.Request.Context(),
			`UPDATE users SET status = 'banned' WHERE id = $1`, id); err != nil {
			respond.Fail(c, apperr.Internal(err))
			return
		}
	}
	h.svc.audit.LogInTx(c.Request.Context(), tx, moderation.Detail{
		Action: "user.sanction." + req.Type, TargetType: "user", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: req.Reason,
	})
	_ = notifications.EmitTx(c.Request.Context(), tx, notifications.Payload{
		RecipientID: id, ActorID: u.ID, Type: notifications.TypeModeration,
		Title: "你收到了一条「" + req.Type + "」处罚", Body: req.Reason,
		DedupeKey: "sanction:" + strconv.FormatInt(sid, 10),
	})
	if err := tx.Commit(c.Request.Context()); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"id": sid, "message": "处罚已创建"})
}

// LiftSanction DELETE /admin/users/:id/sanctions/:sanctionId
func (h *Handler) LiftSanction(c *gin.Context) {
	u := httpx.CurrentUser(c)
	sid, err := strconv.ParseInt(c.Param("sanctionId"), 10, 64)
	if err != nil || sid <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	tx, err := h.svc.pool.Begin(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(c.Request.Context())
	if err := h.svc.sanction.Lift(c.Request.Context(), tx, sid); err != nil {
		respond.Fail(c, err)
		return
	}
	// 若为永久封禁解除则恢复账号状态
	if _, err := tx.Exec(c.Request.Context(), `
		UPDATE users SET status = 'active'
		WHERE id = (SELECT user_id FROM user_sanctions WHERE id = $1) AND status = 'banned'`, sid); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	h.svc.audit.LogInTx(c.Request.Context(), tx, moderation.Detail{
		Action: "user.sanction.lift", TargetType: "sanction", TargetID: sid,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	if err := tx.Commit(c.Request.Context()); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "处罚已解除"})
}

// Unban POST /admin/users/:id/unban
func (h *Handler) Unban(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	ctx := c.Request.Context()
	tx, err := h.svc.pool.Begin(ctx)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1 AND status IN ('banned','suspended')`, id)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.Conflict("CONFLICT", "用户不在封禁状态"))
		return
	}
	if _, err := tx.Exec(ctx, `
		UPDATE user_sanctions SET status = 'lifted', lifted_at = now_utc()
		WHERE user_id = $1 AND status = 'active'`, id); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	h.svc.audit.LogInTx(ctx, tx, moderation.Detail{
		Action: "user.unban", TargetType: "user", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	if err := tx.Commit(ctx); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "用户已解封"})
}

// Sanctions GET /admin/sanctions
func (h *Handler) Sanctions(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.sanction.ListAll(c.Request.Context(), pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if items == nil {
		items = []map[string]any{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// ---- 角色 ----

// Roles GET /admin/roles
func (h *Handler) Roles(c *gin.Context) {
	list, err := h.svc.rbacSvc.ListRoles(c.Request.Context())
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if list == nil {
		list = []rbac.Role{}
	}
	respond.OK(c, gin.H{"items": list})
}

// Permissions GET /admin/permissions
func (h *Handler) Permissions(c *gin.Context) {
	list, err := h.svc.rbacSvc.ListPermissions(c.Request.Context())
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": list})
}

type assignRoleReq struct {
	RoleID   int64  `json:"role_id"`
	Scope    string `json:"scope"` // global | category
	Category int64  `json:"category_id"`
}

// AssignRole POST /admin/users/:id/roles
func (h *Handler) AssignRole(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req assignRoleReq
	if err := c.ShouldBindJSON(&req); err != nil || req.RoleID <= 0 {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	tx, err := h.svc.pool.Begin(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(c.Request.Context())
	if err := h.svc.rbacSvc.AssignRole(c.Request.Context(), tx, id, req.RoleID, req.Category, req.Scope); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	h.svc.audit.LogInTx(c.Request.Context(), tx, moderation.Detail{
		Action: "role.assign", TargetType: "user", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
		Note: "role_id=" + strconv.FormatInt(req.RoleID, 10) + " scope=" + req.Scope,
	})
	if err := tx.Commit(c.Request.Context()); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "角色已授予"})
}

// RemoveRole DELETE /admin/users/:id/roles?role_id=&scope_id=
func (h *Handler) RemoveRole(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	roleID, _ := strconv.ParseInt(c.Query("role_id"), 10, 64)
	scopeID, _ := strconv.ParseInt(c.Query("scope_id"), 10, 64)
	if roleID <= 0 {
		respond.Fail(c, invalid("role_id 不合法"))
		return
	}
	tx, err := h.svc.pool.Begin(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(c.Request.Context())
	if err := h.svc.rbacSvc.RemoveRole(c.Request.Context(), tx, id, roleID, scopeID); err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.audit.LogInTx(c.Request.Context(), tx, moderation.Detail{
		Action: "role.remove", TargetType: "user", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
		Note: "role_id=" + strconv.FormatInt(roleID, 10),
	})
	if err := tx.Commit(c.Request.Context()); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "角色已移除"})
}

// ---- 敏感词 ----

// ListSensitiveWords GET /admin/sensitive-words
func (h *Handler) ListSensitiveWords(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(),
		`SELECT id, word, match_type, action, is_enabled, created_at FROM sensitive_words ORDER BY id`)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type word struct {
		ID        int64     `json:"id"`
		Word      string    `json:"word"`
		MatchType string    `json:"match_type"`
		Action    string    `json:"action"`
		IsEnabled bool      `json:"is_enabled"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := []word{}
	for rows.Next() {
		var w word
		if err := rows.Scan(&w.ID, &w.Word, &w.MatchType, &w.Action, &w.IsEnabled, &w.CreatedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, w)
	}
	respond.OK(c, gin.H{"items": items})
}

type wordReq struct {
	Word      string `json:"word"`
	MatchType string `json:"match_type"`
	Action    string `json:"action"`
	IsEnabled *bool  `json:"is_enabled"`
}

// CreateSensitiveWord POST /admin/sensitive-words
func (h *Handler) CreateSensitiveWord(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req wordReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Word == "" {
		respond.Fail(c, invalid("请输入敏感词"))
		return
	}
	if req.MatchType == "" {
		req.MatchType = "contains"
	}
	if req.Action == "" {
		req.Action = "block"
	}
	if _, err := h.svc.pool.Exec(c.Request.Context(),
		`INSERT INTO sensitive_words (word, match_type, action) VALUES ($1, $2, $3)`,
		req.Word, req.MatchType, req.Action); err != nil {
		respond.Fail(c, apperr.Conflict("CONFLICT", "敏感词已存在"))
		return
	}
	h.svc.gate.Invalidate()
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "sensitive_word.create", TargetType: "sensitive_word",
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: req.Word,
	})
	respond.Created(c, gin.H{"message": "敏感词已添加"})
}

// UpdateSensitiveWord PATCH /admin/sensitive-words/:id
func (h *Handler) UpdateSensitiveWord(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req wordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	_, err = h.svc.pool.Exec(c.Request.Context(), `
		UPDATE sensitive_words SET
		  match_type = COALESCE(NULLIF($2, ''), match_type),
		  action = COALESCE(NULLIF($3, ''), action),
		  is_enabled = COALESCE($4, is_enabled)
		WHERE id = $1`, id, req.MatchType, req.Action, req.IsEnabled)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.gate.Invalidate()
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "sensitive_word.update", TargetType: "sensitive_word", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "敏感词已更新"})
}

// DeleteSensitiveWord DELETE /admin/sensitive-words/:id
func (h *Handler) DeleteSensitiveWord(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	if _, err := h.svc.pool.Exec(c.Request.Context(), `DELETE FROM sensitive_words WHERE id = $1`, id); err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.gate.Invalidate()
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "sensitive_word.delete", TargetType: "sensitive_word", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "敏感词已删除"})
}

// ---- 公告 ----

type announcementReq struct {
	Title    string     `json:"title"`
	Body     string     `json:"body"`
	Status   string     `json:"status"`
	StartsAt *time.Time `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
}

// ListAnnouncements GET /admin/announcements
func (h *Handler) ListAnnouncements(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT a.id, a.title, a.body, a.status, a.starts_at, a.ends_at, COALESCE(u.username, ''), a.published_at, a.created_at
		FROM announcements a JOIN users u ON u.id = a.author_id
		ORDER BY a.created_at DESC LIMIT 100`)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type ann struct {
		ID          int64      `json:"id"`
		Title       string     `json:"title"`
		Body        string     `json:"body"`
		Status      string     `json:"status"`
		StartsAt    *time.Time `json:"starts_at"`
		EndsAt      *time.Time `json:"ends_at"`
		Author      string     `json:"author"`
		PublishedAt *time.Time `json:"published_at"`
		CreatedAt   time.Time  `json:"created_at"`
	}
	items := []ann{}
	for rows.Next() {
		var a ann
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.Status, &a.StartsAt, &a.EndsAt, &a.Author, &a.PublishedAt, &a.CreatedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, a)
	}
	respond.OK(c, gin.H{"items": items})
}

// CreateAnnouncement POST /admin/announcements
func (h *Handler) CreateAnnouncement(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req announcementReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
		respond.Fail(c, invalid("请输入公告标题"))
		return
	}
	status := "draft"
	if req.Status == "published" {
		status = "published"
	}
	var id int64
	err := h.svc.pool.QueryRow(c.Request.Context(), `
		INSERT INTO announcements (title, body, status, starts_at, ends_at, author_id, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $3 = 'published' THEN now_utc() ELSE NULL END)
		RETURNING id`, req.Title, req.Body, status, req.StartsAt, req.EndsAt, u.ID).Scan(&id)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "announcement.create", TargetType: "announcement", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: req.Title,
	})
	respond.Created(c, gin.H{"id": id})
}

// UpdateAnnouncement PATCH /admin/announcements/:id
func (h *Handler) UpdateAnnouncement(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req announcementReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	tag, err := h.svc.pool.Exec(c.Request.Context(), `
		UPDATE announcements SET
		  title = COALESCE(NULLIF($2, ''), title),
		  body = COALESCE($3, body),
		  status = COALESCE(NULLIF($4, ''), status),
		  starts_at = COALESCE($5, starts_at),
		  ends_at = COALESCE($6, ends_at),
		  published_at = CASE WHEN $4 = 'published' THEN COALESCE(published_at, now_utc()) ELSE published_at END,
		  updated_at = now_utc()
		WHERE id = $1`, id, req.Title, req.Body, req.Status, req.StartsAt, req.EndsAt)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.NotFound("公告不存在"))
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "announcement.update", TargetType: "announcement", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "公告已更新"})
}

// DeleteAnnouncement DELETE /admin/announcements/:id
func (h *Handler) DeleteAnnouncement(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	if _, err := h.svc.pool.Exec(c.Request.Context(), `DELETE FROM announcements WHERE id = $1`, id); err != nil {
		respond.Fail(c, err)
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "announcement.delete", TargetType: "announcement", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "公告已删除"})
}

// ---- 公告公开接口 ----

// PublicAnnouncements GET /announcements
func (h *Handler) PublicAnnouncements(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT id, title, body, published_at
		FROM announcements
		WHERE status = 'published' AND (ends_at IS NULL OR ends_at > now_utc())
		ORDER BY published_at DESC LIMIT 10`)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type ann struct {
		ID          int64      `json:"id"`
		Title       string     `json:"title"`
		Body        string     `json:"body"`
		PublishedAt *time.Time `json:"published_at"`
	}
	items := []ann{}
	for rows.Next() {
		var a ann
		if err := rows.Scan(&a.ID, &a.Title, &a.Body, &a.PublishedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, a)
	}
	respond.OK(c, gin.H{"items": items})
}

// ---- 系统设置 ----

// GetSettings GET /admin/settings
func (h *Handler) GetSettings(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(),
		`SELECT key, value, value_type, description, is_public, updated_at FROM system_configs ORDER BY key`)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type conf struct {
		Key         string    `json:"key"`
		Value       string    `json:"value"`
		ValueType   string    `json:"value_type"`
		Description string    `json:"description"`
		IsPublic    bool      `json:"is_public"`
		UpdatedAt   time.Time `json:"updated_at"`
	}
	items := []conf{}
	for rows.Next() {
		var cfg conf
		if err := rows.Scan(&cfg.Key, &cfg.Value, &cfg.ValueType, &cfg.Description, &cfg.IsPublic, &cfg.UpdatedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, cfg)
	}
	respond.OK(c, gin.H{"items": items})
}

// UpdateSettings PATCH /admin/settings {items: [{key, value}]}
func (h *Handler) UpdateSettings(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req struct {
		Items []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	if len(req.Items) > 50 {
		respond.Fail(c, invalid("单次最多更新 50 项"))
		return
	}
	ctx := c.Request.Context()
	tx, err := h.svc.pool.Begin(ctx)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(ctx)
	for _, it := range req.Items {
		if it.Key == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE system_configs SET value = $2, updated_by = $3, updated_at = now_utc() WHERE key = $1`,
			it.Key, it.Value, u.ID); err != nil {
			respond.Fail(c, err)
			return
		}
	}
	h.svc.audit.LogInTx(ctx, tx, moderation.Detail{
		Action: "settings.update", TargetType: "system_config",
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	if err := tx.Commit(ctx); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "设置已保存"})
}

// PublicSettings GET /settings/public —— 公开配置（前台读取）。
func (h *Handler) PublicSettings(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(),
		`SELECT key, value FROM system_configs WHERE is_public = TRUE`)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			respond.Fail(c, err)
			return
		}
		m[k] = v
	}
	respond.OK(c, gin.H{"settings": m})
}

// ---- 日志 ----

// OperationLogs GET /admin/operation-logs
func (h *Handler) OperationLogs(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var total int64
	if err := h.svc.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM operation_logs`).Scan(&total); err != nil {
		respond.Fail(c, err)
		return
	}
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT ol.id, COALESCE(u.username, ''), ol.action, ol.target_type, ol.target_id,
		       ol.request_id, COALESCE(ol.detail->>'note', ''), ol.ip_address, ol.created_at
		FROM operation_logs ol LEFT JOIN users u ON u.id = ol.actor_id
		ORDER BY ol.created_at DESC LIMIT $1 OFFSET $2`, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type logRow struct {
		ID         int64     `json:"id"`
		Actor      string    `json:"actor"`
		Action     string    `json:"action"`
		TargetType string    `json:"target_type"`
		TargetID   int64     `json:"target_id"`
		RequestID  string    `json:"request_id"`
		Note       string    `json:"note"`
		IP         string    `json:"ip"`
		CreatedAt  time.Time `json:"created_at"`
	}
	items := []logRow{}
	for rows.Next() {
		var l logRow
		if err := rows.Scan(&l.ID, &l.Actor, &l.Action, &l.TargetType, &l.TargetID, &l.RequestID, &l.Note, &l.IP, &l.CreatedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, l)
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// LoginLogs GET /admin/login-logs
func (h *Handler) LoginLogs(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var total int64
	if err := h.svc.pool.QueryRow(c.Request.Context(), `SELECT COUNT(*) FROM login_logs`).Scan(&total); err != nil {
		respond.Fail(c, err)
		return
	}
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT ll.id, COALESCE(u.username, '未知'), ll.success, ll.ip_address, ll.created_at
		FROM login_logs ll LEFT JOIN users u ON u.id = ll.user_id
		ORDER BY ll.created_at DESC LIMIT $1 OFFSET $2`, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type logRow struct {
		ID        int64     `json:"id"`
		Username  string    `json:"username"`
		Success   bool      `json:"success"`
		IP        string    `json:"ip"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := []logRow{}
	for rows.Next() {
		var l logRow
		if err := rows.Scan(&l.ID, &l.Username, &l.Success, &l.IP, &l.CreatedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, l)
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

var _ = context.Background

// ---- 评论管理 ----

// Comments GET /admin/comments?status=&q=
func (h *Handler) Comments(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	where := `cm.status <> 'deleted' OR cm.deleted_at IS NOT NULL`
	args := []any{}
	if st := c.Query("status"); st != "" {
		args = append(args, st)
		where = `cm.status = $` + strconv.Itoa(len(args))
	}
	if qs := c.Query("q"); qs != "" {
		args = append(args, "%"+qs+"%")
		where += ` AND cm.body_markdown ILIKE $` + strconv.Itoa(len(args))
	}
	var total int64
	if err := h.svc.pool.QueryRow(c.Request.Context(),
		`SELECT COUNT(*) FROM comments cm WHERE `+where, args...).Scan(&total); err != nil {
		respond.Fail(c, err)
		return
	}
	args = append(args, pq.PageSize, pq.Offset)
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT cm.id, cm.public_id::text, COALESCE(up.display_name, u.username), p.title, p.public_id::text,
		       cm.body_markdown, cm.status, cm.like_count, cm.created_at
		FROM comments cm
		JOIN users u ON u.id = cm.author_id
		JOIN user_profiles up ON up.user_id = u.id
		JOIN posts p ON p.id = cm.post_id
		WHERE `+where+`
		ORDER BY cm.created_at DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	type commentRow struct {
		ID        int64     `json:"id"`
		PublicID  string    `json:"public_id"`
		Author    string    `json:"author"`
		PostTitle string    `json:"post_title"`
		PostID    string    `json:"post_id"`
		Body      string    `json:"body"`
		Status    string    `json:"status"`
		LikeCount int64     `json:"like_count"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := []commentRow{}
	for rows.Next() {
		var it commentRow
		if err := rows.Scan(&it.ID, &it.PublicID, &it.Author, &it.PostTitle, &it.PostID,
			&it.Body, &it.Status, &it.LikeCount, &it.CreatedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, it)
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// ModerateComment POST /admin/comments/:id/moderate {action: hide|restore, reason}
func (h *Handler) ModerateComment(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	status := ""
	switch req.Action {
	case "hide":
		status = "hidden"
	case "restore":
		status = "published"
	default:
		respond.Fail(c, invalid("不支持的动作"))
		return
	}
	ctx := c.Request.Context()
	tx, err := h.svc.pool.Begin(ctx)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE comments SET status = $2, updated_at = now_utc() WHERE id = $1`, id, status)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.NotFound("评论不存在"))
		return
	}
	var authorID int64
	_ = tx.QueryRow(ctx, `SELECT author_id FROM comments WHERE id = $1`, id).Scan(&authorID)
	h.svc.audit.LogInTx(ctx, tx, moderation.Detail{
		Action: "comment." + req.Action, TargetType: "comment", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: req.Reason,
	})
	if authorID > 0 && authorID != u.ID {
		_ = notifications.EmitTx(ctx, tx, notifications.Payload{
			RecipientID: authorID, ActorID: u.ID, Type: notifications.TypeModeration,
			ResourceType: "comment", ResourceID: id,
			Title: "你的评论被管理操作：" + req.Action,
			Body:  req.Reason, DedupeKey: "cmtmod:" + req.Action + ":" + strconv.FormatInt(id, 10),
		})
	}
	if err := tx.Commit(ctx); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "评论已处理"})
}

// UserRoles GET /admin/users/:id/roles
func (h *Handler) UserRoles(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	list, err := h.svc.rbacSvc.UserRoles(c.Request.Context(), id)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if list == nil {
		list = []map[string]any{}
	}
	respond.OK(c, gin.H{"items": list})
}
