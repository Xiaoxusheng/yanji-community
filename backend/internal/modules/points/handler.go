package points

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Handler 积分 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// RegisterUser 用户侧路由。
func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireAuth())
	{
		g.GET("/me/points", h.MyPoints)
		g.GET("/me/points/transactions", h.MyTransactions)
		g.POST("/me/check-in", httpx.RequireVerified(), h.CheckIn)
	}
}

// RegisterAdmin 后台路由。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/badges", h.AdminBadges)
	rg.POST("/badges", h.CreateBadge)
	rg.POST("/users/:id/points-adjust", h.AdjustPoints)
	rg.POST("/users/:id/badges", h.GrantBadge)
	rg.DELETE("/users/:id/badges/:badgeId", h.RevokeBadge)
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// MyPoints GET /me/points
func (h *Handler) MyPoints(c *gin.Context) {
	u := httpx.CurrentUser(c)
	acc, err := h.svc.GetAccount(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"account": acc})
}

// MyTransactions GET /me/points/transactions
func (h *Handler) MyTransactions(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	items, total, err := h.svc.ListTransactions(c.Request.Context(), u.ID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if items == nil {
		items = []Transaction{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

// CheckIn POST /me/check-in
func (h *Handler) CheckIn(c *gin.Context) {
	u := httpx.CurrentUser(c)
	awarded, already, err := h.svc.DailyCheckIn(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if already {
		respond.OK(c, gin.H{"awarded": 0, "already": true, "message": "今天已经签到过了"})
		return
	}
	acc, err := h.svc.GetAccount(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{
		"awarded": awarded, "already": false,
		"balance": acc.Balance, "streak_days": acc.StreakDays,
		"message": "签到成功",
	})
}

// Badges GET /badges
func (h *Handler) Badges(c *gin.Context) {
	items, err := h.svc.ListBadges(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if items == nil {
		items = []Badge{}
	}
	respond.OK(c, gin.H{"items": items})
}

// UserBadges GET /users/:username/badges
func (h *Handler) UserBadges(c *gin.Context) {
	var userID int64
	err := h.svc.pool.QueryRow(c.Request.Context(),
		`SELECT id FROM users WHERE username = $1 AND deleted_at IS NULL`, c.Param("username")).Scan(&userID)
	if err != nil {
		respond.Fail(c, apperr.NotFound("用户不存在"))
		return
	}
	items, err := h.svc.UserBadges(c.Request.Context(), userID)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if items == nil {
		items = []Badge{}
	}
	respond.OK(c, gin.H{"items": items})
}

// AdminBadges GET /admin/badges
func (h *Handler) AdminBadges(c *gin.Context) { h.Badges(c) }

// CreateBadge POST /admin/badges
func (h *Handler) CreateBadge(c *gin.Context) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Icon        string `json:"icon"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Code == "" || req.Name == "" {
		respond.Fail(c, invalid("请输入勋章 code 与名称"))
		return
	}
	if _, err := h.svc.pool.Exec(c.Request.Context(), `
		INSERT INTO badges (code, name, description, icon, condition_type)
		VALUES ($1, $2, $3, $4, 'manual')
		ON CONFLICT (code) DO NOTHING`, req.Code, req.Name, req.Description, req.Icon); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.Created(c, gin.H{"message": "勋章已创建"})
}

type adjustReq struct {
	Amount int64  `json:"amount"`
	Note   string `json:"note"`
}

// AdjustPoints POST /admin/users/:id/points-adjust
func (h *Handler) AdjustPoints(c *gin.Context) {
	u := httpx.CurrentUser(c)
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req adjustReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount == 0 {
		respond.Fail(c, invalid("amount 不能为 0"))
		return
	}
	if req.Amount > 100000 || req.Amount < -100000 {
		respond.Fail(c, invalid("单次调整幅度过大"))
		return
	}
	if req.Note == "" {
		respond.Fail(c, invalid("必须填写调整说明（写入流水与审计）"))
		return
	}
	// 服务端生成幂等键（一次请求一次账，同金额多次调整不冲突）
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	key := "adminadj:" + hex.EncodeToString(buf)
	audit := &moderation.Detail{
		Action: "points.adjust", TargetType: "user", TargetID: userID,
		ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(),
		Note: itoa(req.Amount) + "；" + req.Note,
	}
	if err := h.svc.GrantWithKey(c.Request.Context(), userID, SourceAdminAdjust, 0, key, req.Amount, req.Note, u.ID, audit); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "积分已调整"})
}

// GrantBadge POST /admin/users/:id/badges {badge_id}
func (h *Handler) GrantBadge(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	var req struct {
		BadgeID int64 `json:"badge_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.BadgeID <= 0 {
		respond.Fail(c, invalid("badge_id 不合法"))
		return
	}
	tx, err := h.svc.pool.Begin(c.Request.Context())
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer tx.Rollback(c.Request.Context())
	var code string
	if err := tx.QueryRow(c.Request.Context(),
		`SELECT code FROM badges WHERE id = $1`, req.BadgeID).Scan(&code); err != nil {
		respond.Fail(c, apperr.NotFound("勋章不存在"))
		return
	}
	if err := h.svc.AwardBadgeTx(c.Request.Context(), tx, userID, code, httpx.CurrentUser(c).ID); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if h.svc.audit != nil {
		h.svc.audit.LogInTx(c.Request.Context(), tx, moderation.Detail{
			Action: "badge.grant", TargetType: "user", TargetID: userID,
			ActorID: httpx.CurrentUser(c).ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: code,
		})
	}
	if err := tx.Commit(c.Request.Context()); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	respond.OK(c, gin.H{"message": "勋章已授予"})
}

// RevokeBadge DELETE /admin/users/:id/badges/:badgeId
func (h *Handler) RevokeBadge(c *gin.Context) {
	u := httpx.CurrentUser(c)
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	badgeID, err2 := strconv.ParseInt(c.Param("badgeId"), 10, 64)
	if err != nil || err2 != nil || userID <= 0 || badgeID <= 0 {
		respond.Fail(c, invalid("ID 不合法"))
		return
	}
	tag, err := h.svc.pool.Exec(c.Request.Context(),
		`DELETE FROM user_badges WHERE user_id = $1 AND badge_id = $2`, userID, badgeID)
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	if tag.RowsAffected() == 0 {
		respond.Fail(c, apperr.NotFound("该用户未持有此勋章"))
		return
	}
	if h.svc.audit != nil {
		h.svc.audit.Log(c.Request.Context(), moderation.Detail{
			Action: "badge.revoke", TargetType: "user", TargetID: userID,
			ActorID: u.ID, RequestID: c.GetString("request_id"), IP: c.ClientIP(), Note: "badge_id=" + itoa(badgeID),
		})
	}
	respond.OK(c, gin.H{"message": "勋章已收回"})
}

var _ = context.Background
