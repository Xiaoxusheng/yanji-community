// 问答最佳回答：采纳/取消（question 帖作者；唯一约束保证每帖至多一个最佳答案）。
package comments

import (
	"context"
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/respond"

	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/modules/points"
	"github.com/yanji-community/backend/internal/modules/posts"
)

// Deps2 采纳功能依赖（独立构造，避免与 Service 构造签名耦合）。
type Deps2 struct {
	Pool   *pgxpool.Pool
	Points *points.Service
	Logger *slog.Logger
}

// AnswerService 最佳回答业务。
type AnswerService struct {
	pool   *pgxpool.Pool
	points *points.Service
	logger *slog.Logger
}

// NewAnswerService 构造。
func NewAnswerService(d Deps2) *AnswerService {
	return &AnswerService{pool: d.Pool, points: d.Points, logger: d.Logger}
}

// ref 帖子引用。
type qPost struct {
	ID       int64
	AuthorID int64
	Status   string
	Title    string
}

func (s *AnswerService) loadPost(ctx context.Context, postPublicID string) (*qPost, error) {
	var p qPost
	err := s.pool.QueryRow(ctx, `
		SELECT id, author_id, status, title FROM posts
		WHERE public_id = $1 AND deleted_at IS NULL`, postPublicID).
		Scan(&p.ID, &p.AuthorID, &p.Status, &p.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("帖子不存在")
	}
	return &p, err
}

// Accept 采纳最佳回答。
func (s *AnswerService) Accept(ctx context.Context, actorID int64, commentPublicID string) error {
	var commentID, commentAuthor, postID int64
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.author_id, c.post_id FROM comments c WHERE c.public_id = $1`, commentPublicID).
		Scan(&commentID, &commentAuthor, &postID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("评论不存在")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	var postAuthor int64
	var postType, postStatus string
	var title string
	if err := s.pool.QueryRow(ctx, `
		SELECT author_id, post_type, status, title FROM posts WHERE id = $1`, postID).
		Scan(&postAuthor, &postType, &postStatus, &title); err != nil {
		return apperr.Internal(err)
	}
	if postType != posts.TypeQuestion {
		return apperr.BadRequest("VALIDATION_ERROR", "仅问答帖可以采纳回答")
	}
	if postAuthor != actorID {
		return apperr.Forbidden("只有提问者可以采纳回答")
	}
	if postStatus != posts.StatusPublished {
		return apperr.Conflict("CONFLICT", "帖子当前状态不可操作")
	}
	if commentAuthor == postAuthor {
		return apperr.BadRequest("VALIDATION_ERROR", "不能采纳自己的回答")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	// 先取消旧的最佳答案（部分唯一索引保证至多一个）
	if _, err := tx.Exec(ctx,
		`UPDATE comments SET is_accepted_answer = FALSE WHERE post_id = $1 AND is_accepted_answer`, postID); err != nil {
		return apperr.Internal(err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE comments SET is_accepted_answer = TRUE
		WHERE id = $1 AND status = 'published' AND parent_id IS NULL`, commentID)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.BadRequest("VALIDATION_ERROR", "仅顶层评论可被采纳")
	}
	if s.points != nil {
		_ = s.points.RewardAnswerAccepted(ctx, tx, commentID, commentAuthor)
		_ = s.points.AwardBadgeTx(ctx, tx, commentAuthor, "first_answer", 0)
	}
	_ = notifications.EmitTx(ctx, tx, notifications.Payload{
		RecipientID: commentAuthor, ActorID: actorID, Type: notifications.TypeSystem,
		ResourceType: "comment", ResourceID: commentID,
		Title:     "你的回答被采纳为最佳答案 🎉",
		Body:      "「" + truncate(title, 40) + "」",
		DedupeKey: "accept:" + itoa64(commentID),
	})
	return tx.Commit(ctx)
}

// Unaccept 取消采纳。
func (s *AnswerService) Unaccept(ctx context.Context, actorID int64, commentPublicID string) error {
	var commentID, postID, postAuthor int64
	err := s.pool.QueryRow(ctx, `
		SELECT c.id, c.post_id, p.author_id
		FROM comments c JOIN posts p ON p.id = c.post_id
		WHERE c.public_id = $1 AND c.is_accepted_answer`, commentPublicID).
		Scan(&commentID, &postID, &postAuthor)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("该评论不是最佳答案")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if postAuthor != actorID {
		return apperr.Forbidden("只有提问者可以取消采纳")
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE comments SET is_accepted_answer = FALSE WHERE id = $1`, commentID)
	return err
}

// ---- HTTP ----

// AnswerHandler 最佳回答 HTTP 层。
type AnswerHandler struct{ svc *AnswerService }

// NewAnswerHandler 构造。
func NewAnswerHandler(svc *AnswerService) *AnswerHandler { return &AnswerHandler{svc: svc} }

// Register 注册采纳路由。
func (h *AnswerHandler) Register(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireAuth(), httpx.RequireVerified())
	g.POST("/comments/:id/accept", h.Accept)
	g.DELETE("/comments/:id/accept", h.Unaccept)
}

// Accept POST /comments/:id/accept
func (h *AnswerHandler) Accept(c *gin.Context) {
	if err := h.svc.Accept(c.Request.Context(), httpx.CurrentUser(c).ID, c.Param("id")); err != nil {
		respond2(c, err)
		return
	}
	respondOK(c, gin.H{"message": "已采纳为最佳答案"})
}

// Unaccept DELETE /comments/:id/accept
func (h *AnswerHandler) Unaccept(c *gin.Context) {
	if err := h.svc.Unaccept(c.Request.Context(), httpx.CurrentUser(c).ID, c.Param("id")); err != nil {
		respond2(c, err)
		return
	}
	respondOK(c, gin.H{"message": "已取消采纳"})
}

// 审计说明：采纳/取消为用户行为而非管理操作，不写 operation_logs（处罚/审核类才写入）。

func respond2(c *gin.Context, err error) { respond.Fail(c, err) }

func respondOK(c *gin.Context, data any) { respond.OK(c, data) }

func itoa64(n int64) string {
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
