package comments

import (
	"context"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/modules/posts"
	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/markdown"
	"github.com/yanji-community/backend/internal/shared/rbac"
)

// PostGate 帖子门面（由 posts 模块实现）。
type PostGate interface {
	RefForComment(ctx context.Context, publicID string) (*posts.PostRef, error)
	TouchActivity(ctx context.Context, tx pgx.Tx, postID int64) error
}

// Deps 评论模块依赖。
type Deps struct {
	Pool   *pgxpool.Pool
	RBAC   *rbac.Service
	Gate   *moderation.Gate
	Audit  *moderation.AuditLogger
	Logger *slog.Logger
	Post   PostGate
}

// Service 评论业务。
type Service struct {
	repo *Repo
	deps Deps
}

// New 构造。
func New(deps Deps) *Service {
	return &Service{repo: NewRepo(deps.Pool), deps: deps}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// buildTree 组装评论树（深度≤3，软删除保留占位；按 parent 链挂接，防止跨根误挂）。
func buildTree(rows []row, viewerID int64) []Comment {
	byID := make(map[int64]*Comment, len(rows))
	children := map[int64][]int64{}
	var rootIDs []int64
	for i := range rows {
		r := &rows[i]
		c := r.toComment(viewerID, 0)
		byID[r.ID] = &c
		if r.ParentID == nil {
			rootIDs = append(rootIDs, r.ID)
		} else {
			children[*r.ParentID] = append(children[*r.ParentID], r.ID)
		}
	}
	// DFS 收集每个根的后代（兄弟按时间升序）
	var collect func(id int64) []Comment
	collect = func(id int64) []Comment {
		kids := children[id]
		sort.Slice(kids, func(i, j int) bool {
			return byID[kids[i]].CreatedAt.Before(byID[kids[j]].CreatedAt)
		})
		out := make([]Comment, 0, len(kids))
		for _, kid := range kids {
			c := byID[kid]
			c.Replies = collect(kid)
			out = append(out, *c)
		}
		return out
	}
	roots := make([]Comment, 0, len(rootIDs))
	for _, rid := range rootIDs {
		c := byID[rid]
		c.Replies = collect(rid)
		roots = append(roots, *c)
	}
	return roots
}

// List 评论树（严格校验帖子与版块可见性，防非公开帖 IDOR）。
func (s *Service) List(ctx context.Context, postPublicID string, viewerID int64, sort string, limit, offset int) ([]Comment, int64, error) {
	ref, err := s.deps.Post.RefForComment(ctx, postPublicID)
	if err != nil {
		return nil, 0, err
	}
	canMod := false
	if viewerID > 0 && s.deps.RBAC != nil {
		canMod, _ = s.deps.RBAC.CanModerateCategory(ctx, viewerID, ref.CategoryID)
	}
	if ref.CategoryVisibility != "public" && !canMod {
		return nil, 0, apperr.NotFound("帖子不存在")
	}
	isPublicStatus := ref.Status == posts.StatusPublished || ref.Status == posts.StatusLocked
	if !isPublicStatus && viewerID != ref.AuthorID && !canMod {
		return nil, 0, apperr.NotFound("帖子不存在")
	}
	rows, total, err := s.repo.ListTree(ctx, ref.ID, viewerID, sort, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return buildTree(rows, viewerID), total, nil
}

// Create 创建评论/回复。
func (s *Service) Create(ctx context.Context, actorID int64, postPublicID string, body string, parentPublicID, quotePublicID string) (string, string, error) {
	body = strings.TrimSpace(body)
	if len([]rune(body)) < 1 || len([]rune(body)) > 10000 {
		return "", "", invalid("评论内容需为 1-10000 字")
	}
	ref, err := s.deps.Post.RefForComment(ctx, postPublicID)
	if err != nil {
		return "", "", err
	}
	if ref.CategoryStatus != "active" || ref.CategoryVisibility != "public" {
		return "", "", apperr.NotFound("帖子不存在")
	}
	if ref.Status == posts.StatusLocked {
		return "", "", apperr.ErrPostLocked
	}
	if ref.Status != posts.StatusPublished {
		return "", "", apperr.NotFound("帖子不存在")
	}
	// 敏感词
	action, err := s.deps.Gate.Check(ctx, body)
	if err != nil {
		return "", "", apperr.Internal(err)
	}
	status := "published"
	if action == "block" {
		return "", "", apperr.ErrContentBlocked
	}
	if action == "review" {
		status = "pending_review"
	}

	var parentID, rootID, quoteID *int64
	depth := 0
	var notifyReplyTo int64
	if parentPublicID != "" {
		parent, err := s.repo.GetByPublicID(ctx, parentPublicID)
		if err != nil {
			return "", "", err
		}
		if parent.PostID != ref.ID {
			return "", "", invalid("回复目标不属于该帖子")
		}
		if parent.Status != "published" {
			return "", "", invalid("该评论不可回复")
		}
		if parent.Depth >= MaxDepth {
			// 超过深度 → 引用回复（挂到目标同级）
			quoteID = &parent.ID
			if parent.ParentID != nil {
				pid := *parent.ParentID
				parentID = &pid
			}
			if parent.HasRoot() {
				rid := parent.RootVal()
				rootID = &rid
			}
			depth = MaxDepth
			notifyReplyTo = parent.AuthorID
		} else {
			pid := parent.ID
			parentID = &pid
			depth = parent.Depth + 1
			if parent.HasRoot() {
				rid := parent.RootVal()
				rootID = &rid
			} else {
				rootID = &pid
			}
			notifyReplyTo = parent.AuthorID
		}
	}
	if quotePublicID != "" && quoteID == nil {
		q, err := s.repo.GetByPublicID(ctx, quotePublicID)
		if err != nil {
			return "", "", err
		}
		if q.PostID == ref.ID {
			quoteID = &q.ID
		}
	}

	rendered, err := markdown.Render(body)
	if err != nil {
		return "", "", apperr.Internal(err)
	}

	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return "", "", apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	id, publicID, err := s.repo.InsertTx(ctx, tx, ref.ID, actorID, parentID, rootID, quoteID, depth, body, rendered, status)
	if err != nil {
		return "", "", apperr.Internal(err)
	}
	// 仅公开评论更新计数、活跃度与通知；待审评论审核通过后再计入
	if status == "published" {
		if parentID != nil {
			if err := s.repo.IncReplyCount(ctx, tx, *parentID, 1); err != nil {
				return "", "", apperr.Internal(err)
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE posts SET comment_count = comment_count + 1 WHERE id = $1`, ref.ID); err != nil {
			return "", "", apperr.Internal(err)
		}
		if err := s.deps.Post.TouchActivity(ctx, tx, ref.ID); err != nil {
			return "", "", apperr.Internal(err)
		}

		mentioned, err := s.mentionUserIDs(ctx, body, actorID)
		if err != nil {
			return "", "", apperr.Internal(err)
		}
		mentionedSet := map[int64]bool{}
		for _, m := range mentioned {
			mentionedSet[m] = true
		}
		// 帖子作者
		if _, isMentioned := mentionedSet[ref.AuthorID]; !isMentioned {
			if err := notifications.EmitTx(ctx, tx, notifications.Payload{
				RecipientID: ref.AuthorID, ActorID: actorID, Type: notifications.TypeComment,
				ResourceType: "post", ResourceID: ref.ID,
				Title: "你的帖子「" + truncate(ref.Title, 40) + "」收到了新评论",
				Body:  truncate(body, 80), DedupeKey: "cmt:" + strconv.FormatInt(id, 10) + ":author",
			}); err != nil {
				return "", "", apperr.Internal(err)
			}
		}
		// 被回复者
		if notifyReplyTo > 0 && notifyReplyTo != ref.AuthorID {
			if _, isMentioned := mentionedSet[notifyReplyTo]; !isMentioned {
				if err := notifications.EmitTx(ctx, tx, notifications.Payload{
					RecipientID: notifyReplyTo, ActorID: actorID, Type: notifications.TypeReply,
					ResourceType: "comment", ResourceID: id,
					Title: "有人回复了你在「" + truncate(ref.Title, 40) + "」中的评论",
					Body:  truncate(body, 80), DedupeKey: "cmt:" + strconv.FormatInt(id, 10) + ":reply",
				}); err != nil {
					return "", "", apperr.Internal(err)
				}
			}
		}
		// @提及
		for _, m := range mentioned {
			if err := notifications.EmitTx(ctx, tx, notifications.Payload{
				RecipientID: m, ActorID: actorID, Type: notifications.TypeMention,
				ResourceType: "comment", ResourceID: id,
				Title: "你在「" + truncate(ref.Title, 40) + "」中被 @ 提及",
				Body:  truncate(body, 80), DedupeKey: "mention:" + strconv.FormatInt(id, 10) + ":" + strconv.FormatInt(m, 10),
			}); err != nil {
				return "", "", apperr.Internal(err)
			}
		}
	}
	return publicID, status, tx.Commit(ctx)
}

const maxMentionsPerComment = 10

func (s *Service) mentionUserIDs(ctx context.Context, body string, actorID int64) ([]int64, error) {
	matches := mentionRe.FindAllStringSubmatch(body, -1)
	seen := map[string]bool{}
	var names []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
			if len(names) >= maxMentionsPerComment {
				break
			}
		}
	}
	ids, err := s.repo.ResolveMentions(ctx, names)
	if err != nil {
		return nil, err
	}
	var out []int64
	for _, id := range ids {
		if id != actorID {
			out = append(out, id)
		}
	}
	return out, nil
}

// Update 编辑评论（作者本人；命中 review 敏感词转回待审并同步计数）。
func (s *Service) Update(ctx context.Context, actorID int64, publicID, body string) error {
	cm, err := s.repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	if cm.AuthorID != actorID {
		return apperr.Forbidden("只能编辑自己的评论")
	}
	if cm.Status != "published" {
		return invalid("该评论当前不可编辑")
	}
	body = strings.TrimSpace(body)
	if len([]rune(body)) < 1 || len([]rune(body)) > 10000 {
		return invalid("评论内容需为 1-10000 字")
	}
	action, err := s.deps.Gate.Check(ctx, body)
	if err != nil {
		return apperr.Internal(err)
	}
	if action == "block" {
		return apperr.ErrContentBlocked
	}
	newStatus := ""
	if action == "review" {
		newStatus = "pending_review"
	}
	rendered, err := markdown.Render(body)
	if err != nil {
		return apperr.Internal(err)
	}
	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.UpdateTx(ctx, tx, cm.ID, body, rendered, newStatus); err != nil {
		return apperr.Internal(err)
	}
	if newStatus == "pending_review" {
		if _, err := tx.Exec(ctx,
			`UPDATE posts SET comment_count = GREATEST(0, comment_count - 1) WHERE id = $1`, cm.PostID); err != nil {
			return apperr.Internal(err)
		}
		if cm.ParentID != nil {
			if err := s.repo.IncReplyCount(ctx, tx, *cm.ParentID, -1); err != nil {
				return apperr.Internal(err)
			}
		}
	}
	return tx.Commit(ctx)
}

// Delete 删除评论（作者或版主/管理员），软删除并维护计数。
func (s *Service) Delete(ctx context.Context, actor *Viewer, actorID int64, requestID string, publicID string) error {
	cm, err := s.repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return err
	}
	allowed := cm.AuthorID == actorID
	if !allowed {
		ok, err := s.deps.RBAC.HasPermission(ctx, actorID, "comment.moderate")
		if err != nil {
			return apperr.Internal(err)
		}
		if ok {
			// 版主范围校验：查询帖子所属版块
			var categoryID int64
			if err := s.deps.Pool.QueryRow(ctx,
				`SELECT category_id FROM posts WHERE id = $1`, cm.PostID).Scan(&categoryID); err != nil {
				return apperr.Internal(err)
			}
			can, err := s.deps.RBAC.CanModerateCategory(ctx, actorID, categoryID)
			if err != nil {
				return apperr.Internal(err)
			}
			allowed = can
		}
	}
	if !allowed {
		return apperr.Forbidden("没有权限删除该评论")
	}
	if cm.Status == "deleted" {
		return nil
	}
	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.SoftDeleteTx(ctx, tx, cm.ID); err != nil {
		return apperr.Internal(err)
	}
	if cm.Status == "published" {
		if _, err := tx.Exec(ctx,
			`UPDATE posts SET comment_count = GREATEST(0, comment_count - 1) WHERE id = $1`, cm.PostID); err != nil {
			return apperr.Internal(err)
		}
		if cm.ParentID != nil {
			if err := s.repo.IncReplyCount(ctx, tx, *cm.ParentID, -1); err != nil {
				return apperr.Internal(err)
			}
		}
	}
	if cm.AuthorID != actorID {
		s.deps.Audit.LogInTx(ctx, tx, moderation.Detail{
			Action: "comment.delete", TargetType: "comment", TargetID: cm.ID,
			ActorID: actorID, RequestID: requestID, Note: "版主/管理员删除评论",
		})
	}
	return tx.Commit(ctx)
}

// Viewer 评论模块观察者（与 posts.Viewer 一致的语义）。
type Viewer = posts.Viewer

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
