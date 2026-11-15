package posts

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/modules/categories"
	"github.com/yanji-community/backend/internal/modules/moderation"
	"github.com/yanji-community/backend/internal/modules/points"
	"github.com/yanji-community/backend/internal/modules/polls"
	"github.com/yanji-community/backend/internal/modules/tags"
	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/markdown"
	"github.com/yanji-community/backend/internal/shared/rbac"
)

// Deps 帖子模块依赖（消费方接口，避免循环依赖）。
type Deps struct {
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	RBAC    *rbac.Service
	Cats    *categories.Service
	Tags    *tags.Service
	Gate    *moderation.Gate
	Audit   *moderation.AuditLogger
	Logger  *slog.Logger
	MaxTags int
	Follows FollowsProvider
	Users   UsersProvider
	Points  *points.Service
	Polls   *polls.Service
}

// Service 帖子业务。
type Service struct {
	repo *Repo
	deps Deps
}

// New 构造。
func New(deps Deps) *Service {
	return &Service{repo: NewRepo(deps.Pool), deps: deps}
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// ---- 查询 ----

func (s *Service) loadTagsForRows(ctx context.Context, rows []postRow) ([]PostItem, error) {
	ids := make([]int64, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	tagMap, err := s.deps.Tags.TagsForPosts(ctx, ids)
	if err != nil {
		return nil, err
	}
	items := make([]PostItem, 0, len(rows))
	for _, r := range rows {
		tags := []TagRef{}
		for _, t := range tagMap[r.ID] {
			tags = append(tags, TagRef{ID: t.ID, Name: t.Name, Slug: t.Slug})
		}
		items = append(items, toItem(r, tags))
	}
	return items, nil
}

// Feed 首页信息流。
func (s *Service) Feed(ctx context.Context, p FeedParams) ([]PostItem, int64, error) {
	rows, total, err := s.repo.Feed(ctx, p)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.loadTagsForRows(ctx, rows)
	return items, total, err
}

// ListByCategory 版块列表。
func (s *Service) ListByCategory(ctx context.Context, cat *categories.Category, viewerID int64, sort string, limit, offset int) ([]PostItem, int64, error) {
	rows, total, err := s.repo.ListByCategory(ctx, cat.ID, viewerID, sort, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.loadTagsForRows(ctx, rows)
	return items, total, err
}

// ListByTag / ListByAuthor。
func (s *Service) ListByTag(ctx context.Context, tagID int64, viewerID int64, limit, offset int) ([]PostItem, int64, error) {
	rows, total, err := s.repo.ListByTag(ctx, tagID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.loadTagsForRows(ctx, rows)
	return items, total, err
}

func (s *Service) ListByAuthor(ctx context.Context, authorID, viewerID int64, limit, offset int) ([]PostItem, int64, error) {
	rows, total, err := s.repo.ListByAuthor(ctx, authorID, viewerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.loadTagsForRows(ctx, rows)
	return items, total, err
}

// MyPosts 我的帖子。
func (s *Service) MyPosts(ctx context.Context, authorID int64, status string, limit, offset int) ([]PostItem, int64, error) {
	rows, total, err := s.repo.MyPosts(ctx, authorID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	items, err := s.loadTagsForRows(ctx, rows)
	return items, total, err
}

// Detail 帖子详情（含可见性/权限判定）。
func (s *Service) Detail(ctx context.Context, publicID string, viewer *Viewer) (*PostDetail, error) {
	row, err := s.repo.GetByPublicID(ctx, publicID, viewer.UserID)
	if err != nil {
		return nil, err
	}
	// 版块可见性：private 需版主/管理员；hidden 版块同理
	cat, err := s.deps.Cats.GetByID(ctx, row.CategoryID)
	if err != nil {
		return nil, err
	}
	canViewPrivate := viewer.IsStaff || viewer.ModeratedAll
	if cat.Visibility != "public" && !canViewPrivate {
		// 版主范围校验
		if len(viewer.ModeratedCategories) > 0 {
			for _, id := range viewer.ModeratedCategories {
				if id == row.CategoryID {
					canViewPrivate = true
				}
			}
		}
	}
	if cat.Visibility != "public" && !canViewPrivate {
		return nil, apperr.NotFound("帖子不存在")
	}

	// 状态可见性：非公开状态仅作者与可管理者可见
	isAuthor := viewer.UserID == row.AuthorID
	if !isAuthor && row.Status != StatusPublished {
		if !canViewPrivate {
			return nil, apperr.NotFound("帖子不存在")
		}
	}
	tags := []TagRef{}
	tagList, err := s.deps.Tags.TagsForPost(ctx, row.ID)
	if err == nil {
		for _, t := range tagList {
			tags = append(tags, TagRef{ID: t.ID, Name: t.Name, Slug: t.Slug})
		}
	}
	detail := &PostDetail{
		PostItem: toItem(*row, tags),
	}
	detail.BodyRendered = row.BodyRendered
	if isAuthor || canViewPrivate {
		detail.RejectReason = row.RejectReason
	}
	detail.IsAuthor = isAuthor
	detail.CanModerate = canViewPrivate || viewer.IsStaff
	detail.EditedAt = row.EditedAt
	return detail, nil
}

// Viewer 观察者上下文（handler 组装）。
type Viewer struct {
	UserID              int64
	IsStaff             bool // admin/superadmin（具备全局 post.moderate）
	ModeratedCategories []int64
	ModeratedAll        bool // 版主全部版块（暂无此概念，保留）
}

// CanModerateCategory 判断观察者是否可管理指定版块。
func (v *Viewer) CanModerateCategory(categoryID int64) bool {
	if v.IsStaff {
		return true
	}
	for _, id := range v.ModeratedCategories {
		if id == categoryID {
			return true
		}
	}
	return false
}

// ---- 写入 ----

func validateContent(title, body string, tagSlugs []string, maxTags int) error {
	tl := len([]rune(strings.TrimSpace(title)))
	if tl < 5 || tl > 200 {
		return invalid("标题长度需为 5-200 字")
	}
	if len([]rune(strings.TrimSpace(body))) < 1 || len(body) > 100000 {
		return invalid("正文不能为空且不超过 100000 字符")
	}
	if len(tagSlugs) > maxTags {
		return invalid("标签最多 " + itoa(int64(maxTags)) + " 个")
	}
	return nil
}

// resolveStatus 依据版块审核策略与敏感词检查决定初始状态。
// 返回 (status, err)；pending 时不视为错误，由 handler 决定响应。
func (s *Service) resolveStatus(ctx context.Context, cat *categories.Category, title, body string, isDraft bool, accountAge time.Duration) (string, error) {
	if isDraft {
		return StatusDraft, nil
	}
	// 敏感词检查
	action, err := s.deps.Gate.Check(ctx, title+" "+body)
	if err != nil {
		return "", apperr.Internal(err)
	}
	switch action {
	case "block":
		return "", apperr.ErrContentBlocked
	case "review":
		return StatusPendingReview, nil
	}
	// 版块审核策略
	switch cat.ReviewPolicy {
	case "all":
		return StatusPendingReview, nil
	case "new_users":
		if accountAge < 7*24*time.Hour {
			return StatusPendingReview, nil
		}
	}
	return StatusPublished, nil
}

// Create 创建帖子/草稿。
func (s *Service) Create(ctx context.Context, authorID int64, authorName string, req CreateRequest) (publicID string, status string, err error) {
	accountAge := s.accountAge(ctx, authorID)
	if err := validateContent(req.Title, req.BodyMarkdown, req.TagSlugs, s.deps.MaxTags); err != nil {
		return "", "", err
	}
	switch req.PostType {
	case "", TypeDiscussion, TypeArticle, TypeQuestion, TypePoll:
	default:
		return "", "", invalid("不支持的帖子类型")
	}
	cat, err := s.deps.Cats.GetBySlug(ctx, req.CategorySlug)
	if err != nil {
		return "", "", err
	}
	if cat.Status != "active" {
		return "", "", invalid("版块已归档，无法发帖")
	}
	if cat.Visibility != "public" {
		return "", "", apperr.ErrForbidden
	}
	if cat.PostingPolicy == "staff_only" {
		ok, err := s.deps.RBAC.HasPermission(ctx, authorID, "post.moderate")
		if err != nil {
			return "", "", apperr.Internal(err)
		}
		if !ok {
			return "", "", apperr.Forbidden("该版块仅限版主及以上发布")
		}
	}
	status, err = s.resolveStatus(ctx, cat, req.Title, req.BodyMarkdown, req.IsDraft, accountAge)
	if err != nil {
		return "", "", err
	}

	rendered, err := markdown.Render(req.BodyMarkdown)
	if err != nil {
		return "", "", apperr.Internal(err)
	}
	excerpt := req.Excerpt
	if excerpt == "" {
		excerpt = markdown.Excerpt(req.BodyMarkdown, 120)
	}

	tagIDs, err := s.deps.Tags.ResolveIDs(ctx, req.TagSlugs)
	if err != nil {
		return "", "", err
	}

	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return "", "", apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	id, publicID, err := s.repo.CreateTx(ctx, tx, authorID, cat.ID, &req, status, excerpt, rendered)
	if err != nil {
		return "", "", apperr.Internal(err)
	}
	if err := s.deps.Tags.AttachTx(ctx, tx, id, tagIDs); err != nil {
		return "", "", apperr.Internal(err)
	}
	if req.PostType == TypePoll {
		if s.deps.Polls == nil {
			return "", "", apperr.Internal(errPollsMissing)
		}
		if err := s.deps.Polls.CreateForPostTx(ctx, tx, id, req.Poll); err != nil {
			return "", "", err
		}
	}
	if status == StatusPublished {
		if _, err := tx.Exec(ctx, `UPDATE categories SET post_count = post_count + 1 WHERE id = $1`, cat.ID); err != nil {
			return "", "", apperr.Internal(err)
		}
		if s.deps.Points != nil {
			_ = s.deps.Points.RewardPostPublished(ctx, tx, id, authorID)
			_ = s.deps.Points.AwardBadgeTx(ctx, tx, authorID, "first_post", 0)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", apperr.Internal(err)
	}
	s.deps.Logger.Info("post created", "author", authorName, "status", status, "public_id", publicID)
	return publicID, status, nil
}

// Update 编辑帖子（作者本人；保留修订历史）。
func (s *Service) Update(ctx context.Context, actor *Viewer, actorID int64, publicID string, req UpdateRequest, editReason string) error {
	row, err := s.repo.GetByPublicID(ctx, publicID, 0)
	if err != nil {
		return err
	}
	if row.AuthorID != actorID {
		return apperr.Forbidden("只能编辑自己的帖子")
	}
	if row.Status == StatusDeleted {
		return apperr.NotFound("帖子不存在")
	}

	title, body := row.Title, ""
	// 需要读取原 markdown 以便校验：从 body_rendered 拿不到原文，这里读取 body_markdown
	orig, err := s.repo.GetMarkdown(ctx, row.ID)
	if err != nil {
		return apperr.Internal(err)
	}
	body = orig
	newTitle, newBody := title, body
	if req.Title != nil {
		newTitle = *req.Title
	}
	if req.BodyMarkdown != nil {
		newBody = *req.BodyMarkdown
	}
	maxTags := s.deps.MaxTags
	var tagSlugs []string
	if req.TagSlugs != nil {
		tagSlugs = *req.TagSlugs
		if len(tagSlugs) > maxTags {
			return invalid("标签最多 " + itoa(int64(maxTags)) + " 个")
		}
	}
	if err := validateContent(newTitle, newBody, tagSlugs, maxTags); err != nil {
		return err
	}

	// 敏感词复查
	action, err := s.deps.Gate.Check(ctx, newTitle+" "+newBody)
	if err != nil {
		return apperr.Internal(err)
	}
	if action == "block" {
		return apperr.ErrContentBlocked
	}

	rendered, err := markdown.Render(newBody)
	if err != nil {
		return apperr.Internal(err)
	}
	excerpt := markdown.Excerpt(newBody, 120)

	fields := map[string]any{
		"title": newTitle, "body_markdown": newBody, "body_rendered": rendered, "excerpt": excerpt,
	}
	// 被隐藏/驳回的帖子编辑后重新进入待审核
	if row.Status == StatusHidden || row.Status == StatusRejected {
		fields["status"] = StatusPendingReview
	}

	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.UpdateTx(ctx, tx, row.ID, fields); err != nil {
		return apperr.Internal(err)
	}
	if err := s.repo.SaveRevision(ctx, tx, row.ID, actorID, newTitle, newBody, editReason); err != nil {
		return apperr.Internal(err)
	}
	if req.TagSlugs != nil {
		tagIDs, err := s.deps.Tags.ResolveIDs(ctx, tagSlugs)
		if err != nil {
			return err
		}
		if err := s.deps.Tags.AttachTx(ctx, tx, row.ID, tagIDs); err != nil {
			return apperr.Internal(err)
		}
	}
	return tx.Commit(ctx)
}

// Publish 发布草稿（走审核策略）。
func (s *Service) Publish(ctx context.Context, actorID int64, publicID string) (string, error) {
	accountAge := s.accountAge(ctx, actorID)
	row, err := s.repo.GetByPublicID(ctx, publicID, 0)
	if err != nil {
		return "", err
	}
	if row.AuthorID != actorID {
		return "", apperr.Forbidden("只能发布自己的帖子")
	}
	if row.Status != StatusDraft {
		return "", apperr.Conflict("CONFLICT", "仅草稿可以发布")
	}
	cat, err := s.deps.Cats.GetByID(ctx, row.CategoryID)
	if err != nil {
		return "", err
	}
	bodyMd, err := s.repo.GetMarkdown(ctx, row.ID)
	if err != nil {
		return "", apperr.Internal(err)
	}
	status, err := s.resolveStatus(ctx, cat, row.Title, bodyMd, false, accountAge)
	if err != nil {
		return "", err
	}
	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return "", apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.SetStatusTx(ctx, tx, row.ID, status, ""); err != nil {
		return "", apperr.Internal(err)
	}
	if status == StatusPublished {
		if _, err := tx.Exec(ctx, `UPDATE categories SET post_count = post_count + 1 WHERE id = $1`, cat.ID); err != nil {
			return "", apperr.Internal(err)
		}
		if s.deps.Points != nil {
			_ = s.deps.Points.RewardPostPublished(ctx, tx, row.ID, actorID)
		}
	}
	return status, tx.Commit(ctx)
}

// Delete 作者软删除。
func (s *Service) Delete(ctx context.Context, actor *Viewer, actorID int64, publicID string) error {
	row, err := s.repo.GetByPublicID(ctx, publicID, 0)
	if err != nil {
		return err
	}
	isMod := actor.CanModerateCategory(row.CategoryID)
	if row.AuthorID != actorID && !isMod {
		return apperr.Forbidden("只能删除自己的帖子")
	}
	if row.Status == StatusDeleted {
		return nil
	}
	tx, err := s.deps.Pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.repo.SetStatusTx(ctx, tx, row.ID, StatusDeleted, ""); err != nil {
		return apperr.Internal(err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE posts SET deleted_at = now_utc() WHERE id = $1`, row.ID); err != nil {
		return apperr.Internal(err)
	}
	if row.Status == StatusPublished {
		if _, err := tx.Exec(ctx,
			`UPDATE categories SET post_count = GREATEST(0, post_count - 1) WHERE id = $1`, row.CategoryID); err != nil {
			return apperr.Internal(err)
		}
	}
	s.deps.Audit.LogInTx(ctx, tx, moderation.Detail{
		Action: "post.delete", TargetType: "post", TargetID: row.ID, ActorID: actorID, Note: "软删除帖子",
	})
	return tx.Commit(ctx)
}

// Revisions 修订历史（作者/管理）。
func (s *Service) Revisions(ctx context.Context, actor *Viewer, actorID int64, publicID string) ([]Revision, error) {
	row, err := s.repo.GetByPublicID(ctx, publicID, 0)
	if err != nil {
		return nil, err
	}
	if row.AuthorID != actorID && !actor.CanModerateCategory(row.CategoryID) {
		return nil, apperr.ErrForbidden
	}
	list, err := s.repo.Revisions(ctx, row.ID)
	if list == nil {
		list = []Revision{}
	}
	return list, err
}

// RecordView 浏览计数（Redis 去重，1h 窗口）。
func (s *Service) RecordView(ctx context.Context, postPublicID string, viewerKey string) {
	row, err := s.repo.GetByPublicID(ctx, postPublicID, 0)
	if err != nil || row.Status != StatusPublished {
		return
	}
	if s.deps.Redis != nil {
		key := "postview:" + postPublicID + ":" + viewerKey
		ok, err := s.deps.Redis.SetNX(ctx, key, 1, time.Hour).Result()
		if err != nil || !ok {
			return
		}
	}
	_ = s.repo.IncView(ctx, row.ID)
}

// RefForComment 供评论模块取帖子引用。
func (s *Service) RefForComment(ctx context.Context, publicID string) (*PostRef, error) {
	return s.repo.RefForComment(ctx, publicID)
}

// TouchActivity 供评论模块更新帖子活跃时间。
func (s *Service) TouchActivity(ctx context.Context, tx pgx.Tx, postID int64) error {
	return s.repo.TouchActivity(ctx, tx, postID)
}

// accountAge 查询账号注册时长（新用户审核策略用）。
func (s *Service) accountAge(ctx context.Context, userID int64) time.Duration {
	var createdAt time.Time
	_ = s.deps.Pool.QueryRow(ctx, `SELECT created_at FROM users WHERE id = $1`, userID).Scan(&createdAt)
	return time.Since(createdAt)
}

// GetMarkdown 读取原始 Markdown（repo 补充方法）。
func (s *Service) GetMarkdown(ctx context.Context, postID int64) (string, error) {
	return s.repo.GetMarkdown(ctx, postID)
}

var errPollsMissing = apperr.Internal(nil)
