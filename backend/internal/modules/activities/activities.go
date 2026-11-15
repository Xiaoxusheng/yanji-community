// Package activities 活动：报名（并发名额安全）、取消、后台管理与出席奖励。
package activities

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
	"github.com/yanji-community/backend/internal/modules/points"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Activity 活动。
type Activity struct {
	ID               int64      `json:"id"`
	Title            string     `json:"title"`
	Description      string     `json:"description"`
	CoverKey         string     `json:"cover_key"`
	Location         string     `json:"location"`
	StartsAt         time.Time  `json:"starts_at"`
	EndsAt           *time.Time `json:"ends_at"`
	SignupStartsAt   *time.Time `json:"signup_starts_at"`
	SignupEndsAt     *time.Time `json:"signup_ends_at"`
	Capacity         int        `json:"capacity"`
	ParticipantCount int64      `json:"participant_count"`
	Status           string     `json:"status"`
	RewardPoints     int        `json:"reward_points"`
	CreatedBy        string     `json:"created_by_name"`
	IsJoined         bool       `json:"is_joined"`
	CreatedAt        time.Time  `json:"created_at"`
}

// Repo 活动数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

const actCols = `a.id, a.title, a.description, a.cover_key, a.location, a.starts_at, a.ends_at,
	a.signup_starts_at, a.signup_ends_at, a.capacity,
	(SELECT COUNT(*) FROM activity_participants ap WHERE ap.activity_id = a.id AND ap.status = 'signed_up'),
	a.status, a.reward_points, COALESCE(u.username, ''), a.created_at`

func scanAct(row pgx.Row) (*Activity, error) {
	var a Activity
	err := row.Scan(&a.ID, &a.Title, &a.Description, &a.CoverKey, &a.Location, &a.StartsAt,
		&a.EndsAt, &a.SignupStartsAt, &a.SignupEndsAt, &a.Capacity,
		&a.ParticipantCount, &a.Status, &a.RewardPoints, &a.CreatedBy, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// List 公开活动列表（已发布）。
func (r *Repo) List(ctx context.Context, limit, offset int) ([]Activity, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM activities WHERE status = 'published'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+actCols+`
		FROM activities a JOIN users u ON u.id = a.created_by
		WHERE a.status = 'published'
		ORDER BY a.starts_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Activity
	for rows.Next() {
		a, err := scanAct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *a)
	}
	return out, total, rows.Err()
}

// Get 详情。
func (r *Repo) Get(ctx context.Context, id int64) (*Activity, error) {
	a, err := scanAct(r.pool.QueryRow(ctx, `
		SELECT `+actCols+`
		FROM activities a JOIN users u ON u.id = a.created_by WHERE a.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("活动不存在")
	}
	return a, err
}

// JoinTx 报名（FOR UPDATE 行锁 + 唯一约束 + 容量检查，并发安全）。
// 返回 false 表示已报名（幂等）。
func (r *Repo) JoinTx(ctx context.Context, tx pgx.Tx, activityID, userID int64) (bool, error) {
	// 行锁锁定活动行，串行化同一活动的报名竞争
	var capacity int
	var signupStarts, signupEnds *time.Time
	var status string
	var endsAt *time.Time
	err := tx.QueryRow(ctx, `
		SELECT capacity, signup_starts_at, signup_ends_at, status, ends_at
		FROM activities WHERE id = $1 FOR UPDATE`, activityID).
		Scan(&capacity, &signupStarts, &signupEnds, &status, &endsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, apperr.NotFound("活动不存在")
	}
	if err != nil {
		return false, err
	}
	if status != "published" {
		return false, apperr.Conflict("CONFLICT", "活动当前不可报名")
	}
	now := time.Now().UTC()
	if signupStarts != nil && now.Before(*signupStarts) {
		return false, apperr.Conflict("CONFLICT", "报名尚未开始")
	}
	if signupEnds != nil && now.After(*signupEnds) {
		return false, apperr.Conflict("CONFLICT", "报名已截止")
	}
	if endsAt != nil && now.After(*endsAt) {
		return false, apperr.Conflict("CONFLICT", "活动已结束")
	}
	// 已有报名记录
	var existing string
	err = tx.QueryRow(ctx,
		`SELECT status FROM activity_participants WHERE activity_id = $1 AND user_id = $2 FOR UPDATE`,
		activityID, userID).Scan(&existing)
	if err == nil {
		if existing == "signed_up" {
			return false, nil // 幂等
		}
		// cancelled → 重新报名，需再查容量
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if capacity > 0 {
		var count int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM activity_participants
			WHERE activity_id = $1 AND status = 'signed_up'`, activityID).Scan(&count); err != nil {
			return false, err
		}
		if count >= capacity {
			return false, apperr.Conflict("CONFLICT", "活动名额已满")
		}
	}
	if existing == "cancelled" {
		tag, err := tx.Exec(ctx,
			`UPDATE activity_participants SET status = 'signed_up', signed_at = now_utc()
			 WHERE activity_id = $1 AND user_id = $2`, activityID, userID)
		if err != nil {
			return false, err
		}
		return tag.RowsAffected() > 0, nil
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO activity_participants (activity_id, user_id) VALUES ($1, $2)
		ON CONFLICT (activity_id, user_id) DO NOTHING`, activityID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// Cancel 取消报名。
func (r *Repo) Cancel(ctx context.Context, activityID, userID int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE activity_participants SET status = 'cancelled'
		 WHERE activity_id = $1 AND user_id = $2 AND status = 'signed_up'`, activityID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("未找到有效报名")
	}
	return nil
}

// ---- Service ----

// Service 活动业务。
type Service struct {
	repo   *Repo
	pool   *pgxpool.Pool
	points *points.Service
	audit  *moderation.AuditLogger
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, pointsSvc *points.Service, audit *moderation.AuditLogger, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), pool: pool, points: pointsSvc, audit: audit, logger: logger}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// Join 报名（事务 + 通知）。
func (s *Service) Join(ctx context.Context, userID int64, activityID int64) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	joined, err := s.repo.JoinTx(ctx, tx, activityID, userID)
	if err != nil || !joined {
		return false, err
	}
	var creator, title string
	_ = tx.QueryRow(ctx, `SELECT created_by, title FROM activities WHERE id = $1`, activityID).Scan(&creator, &title)
	var creatorID int64
	_, _ = strconv.ParseInt(creator, 10, 64)
	_ = tx.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, creator).Scan(&creatorID)
	_ = notifications.EmitTx(ctx, tx, notifications.Payload{
		RecipientID: creatorID, ActorID: userID, Type: notifications.TypeSystem,
		ResourceType: "post", ResourceID: activityID,
		Title:     "「" + title + "」有新的报名",
		DedupeKey: "actjoin:" + itoa(activityID) + ":" + itoa(userID),
	})
	if err := tx.Commit(ctx); err != nil {
		return false, apperr.Internal(err)
	}
	return true, nil
}

// ---- HTTP ----

// Handler 活动 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterPublic 公开路由。
func (h *Handler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/activities", h.List)
	rg.GET("/activities/:id", h.Get)
	join := rg.Group("")
	join.Use(httpx.RequireAuth(), httpx.RequireVerified())
	join.POST("/activities/:id/join", h.Join)
	join.DELETE("/activities/:id/join", h.Cancel)
}

// RegisterAdmin 后台路由。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/activities", h.AdminList)
	rg.POST("/activities", h.Create)
	rg.PATCH("/activities/:id", h.Update)
	rg.DELETE("/activities/:id", h.Delete)
	rg.POST("/activities/:id/attend", h.MarkAttend)
}

// List GET /activities
func (h *Handler) List(c *gin.Context) {
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.repo.List(c.Request.Context(), pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if items == nil {
		items = []Activity{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// Get GET /activities/:id
func (h *Handler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	a, err := h.svc.repo.Get(c.Request.Context(), id)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if u := httpx.CurrentUser(c); u != nil {
		_ = h.svc.pool.QueryRow(c.Request.Context(), `
			SELECT EXISTS(SELECT 1 FROM activity_participants
			              WHERE activity_id = $1 AND user_id = $2 AND status = 'signed_up')`,
			id, u.ID).Scan(&a.IsJoined)
	}
	respond.OK(c, gin.H{"activity": a})
}

// Join POST /activities/:id/join
func (h *Handler) Join(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	joined, err := h.svc.Join(c.Request.Context(), httpx.CurrentUser(c).ID, id)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	msg := "报名成功"
	if !joined {
		msg = "你已在该活动的报名名单中"
	}
	respond.OK(c, gin.H{"joined": true, "message": msg})
}

// Cancel DELETE /activities/:id/join
func (h *Handler) Cancel(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	if err := h.svc.repo.Cancel(c.Request.Context(), id, httpx.CurrentUser(c).ID); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"joined": false, "message": "已取消报名"})
}

type actReq struct {
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Location       string     `json:"location"`
	StartsAt       time.Time  `json:"starts_at"`
	EndsAt         *time.Time `json:"ends_at"`
	SignupStartsAt *time.Time `json:"signup_starts_at"`
	SignupEndsAt   *time.Time `json:"signup_ends_at"`
	Capacity       int        `json:"capacity"`
	Status         string     `json:"status"`
	RewardPoints   int        `json:"reward_points"`
}

// AdminList GET /admin/activities
func (h *Handler) AdminList(c *gin.Context) {
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT `+actCols+`
		FROM activities a JOIN users u ON u.id = a.created_by
		ORDER BY a.starts_at DESC LIMIT 100`)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer rows.Close()
	items := []Activity{}
	for rows.Next() {
		a, err := scanAct(rows)
		if err != nil {
			respond.Fail(c, apperr.Internal(err))
			return
		}
		items = append(items, *a)
	}
	respond.OK(c, gin.H{"items": items})
}

// Create POST /admin/activities
func (h *Handler) Create(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req actReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" || req.StartsAt.IsZero() {
		respond.Fail(c, invalid("请填写活动标题与开始时间"))
		return
	}
	status := "draft"
	if req.Status == "published" {
		status = "published"
	}
	var id int64
	err := h.svc.pool.QueryRow(c.Request.Context(), `
		INSERT INTO activities (title, description, location, starts_at, ends_at, signup_starts_at, signup_ends_at, capacity, status, reward_points, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		req.Title, req.Description, req.Location, req.StartsAt, req.EndsAt,
		req.SignupStartsAt, req.SignupEndsAt, req.Capacity, status, req.RewardPoints, u.ID).Scan(&id)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "activity.create", TargetType: "activity", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: req.Title,
	})
	respond.Created(c, gin.H{"id": id})
}

// Update PATCH /admin/activities/:id
func (h *Handler) Update(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req actReq
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, invalid("请求体格式不正确"))
		return
	}
	tag, err := h.svc.pool.Exec(c.Request.Context(), `
		UPDATE activities SET
		  title = COALESCE(NULLIF($2, ''), title),
		  description = COALESCE($3, description),
		  location = COALESCE($4, location),
		  starts_at = COALESCE($5, starts_at),
		  ends_at = $6,
		  signup_starts_at = $7,
		  signup_ends_at = $8,
		  capacity = COALESCE($9, capacity),
		  status = COALESCE(NULLIF($10, ''), status),
		  reward_points = COALESCE($11, reward_points),
		  updated_at = now_utc()
		WHERE id = $1`,
		id, req.Title, req.Description, req.Location, req.StartsAt,
		req.EndsAt, req.SignupStartsAt, req.SignupEndsAt, req.Capacity, req.Status, req.RewardPoints)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.NotFound("活动不存在"))
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "activity.update", TargetType: "activity", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "活动已更新"})
}

// Delete DELETE /admin/activities/:id
func (h *Handler) Delete(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	tag, err := h.svc.pool.Exec(c.Request.Context(), `DELETE FROM activities WHERE id = $1`, id)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.NotFound("活动不存在"))
		return
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "activity.delete", TargetType: "activity", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	respond.OK(c, gin.H{"message": "活动已删除"})
}

// MarkAttend POST /admin/activities/:id/attend {user_id} —— 标记出席并发放活动积分。
func (h *Handler) MarkAttend(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req struct {
		UserID int64 `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		respond.Fail(c, invalid("user_id 不合法"))
		return
	}
	tx, err := h.svc.pool.Begin(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(c.Request.Context())
	tag, err := tx.Exec(c.Request.Context(), `
		UPDATE activity_participants SET status = 'attended'
		WHERE activity_id = $1 AND user_id = $2 AND status = 'signed_up'`, id, req.UserID)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.Conflict("CONFLICT", "该用户不在报名名单或已标记出席"))
		return
	}
	var reward int
	var title string
	_ = tx.QueryRow(c.Request.Context(),
		`SELECT reward_points, title FROM activities WHERE id = $1`, id).Scan(&reward, &title)
	if reward > 0 && h.svc.points != nil {
		_ = h.svc.points.GrantTx(c.Request.Context(), tx, req.UserID, points.SourceActivityAttend, id,
			"attend:"+itoa(id)+":"+itoa(req.UserID), int64(reward), "参加活动「"+title+"」", u.ID)
	}
	h.svc.audit.Log(c.Request.Context(), moderation.Detail{
		Action: "activity.attend", TargetType: "activity", TargetID: id,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
	})
	if err := tx.Commit(c.Request.Context()); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "已标记出席" + func() string {
		if reward > 0 {
			return "，奖励积分已发放"
		}
		return ""
	}()})
}

// Participants GET /admin/activities/:id/participants（管理端名单）。
func (h *Handler) Participants(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT ap.user_id, COALESCE(p.display_name, u.username), ap.status, ap.signed_at
		FROM activity_participants ap
		JOIN users u ON u.id = ap.user_id
		JOIN user_profiles p ON p.user_id = u.id
		WHERE ap.activity_id = $1 ORDER BY ap.signed_at`, id)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer rows.Close()
	type part struct {
		UserID   int64     `json:"user_id"`
		Name     string    `json:"name"`
		Status   string    `json:"status"`
		SignedAt time.Time `json:"signed_at"`
	}
	items := []part{}
	for rows.Next() {
		var p part
		if err := rows.Scan(&p.UserID, &p.Name, &p.Status, &p.SignedAt); err != nil {
			respond.Fail(c, apperr.Internal(err))
			return
		}
		items = append(items, p)
	}
	respond.OK(c, gin.H{"items": items})
}

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
