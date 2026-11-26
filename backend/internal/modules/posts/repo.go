package posts

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// Repo 帖子数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// postRow 查询行（与 itemCols 对应）。
type postRow struct {
	ID             int64
	PublicID       string
	Title          string
	Excerpt        string
	PostType       string
	Status         string
	BodyRendered   string
	RejectReason   string
	AuthorID       int64
	AuthorPID      string
	AuthorName     string
	AuthorDisplay  string
	AuthorAvatar   string
	CategoryID     int64
	CategoryName   string
	CategorySlug   string
	IsPinned       bool
	IsFeatured     bool
	ViewCount      int64
	LikeCount      int64
	CommentCount   int64
	BookmarkCount  int64
	IsLiked        bool
	IsBookmarked   bool
	PublishedAt    *time.Time
	LastActivityAt *time.Time
	CreatedAt      time.Time
	EditedAt       *time.Time
}

// viewerJoin 认可/收藏状态联查片段；viewerID=0 时恒为 false。
func viewerJoin(viewerID int64) string {
	if viewerID <= 0 {
		return "FALSE, FALSE"
	}
	return `EXISTS(SELECT 1 FROM post_likes pl WHERE pl.post_id = p.id AND pl.user_id = ` + itoa(viewerID) + `),
	        EXISTS(SELECT 1 FROM bookmarks b WHERE b.post_id = p.id AND b.user_id = ` + itoa(viewerID) + `)`
}

// itemCols 列表查询列。
const itemCols = `p.id, p.public_id::text, p.title, p.excerpt, p.post_type, p.status, '' AS body_rendered,
	COALESCE(p.reject_reason,''), p.author_id, u.public_id::text, u.username,
	COALESCE(up.display_name, u.username), up.avatar_key,
	p.category_id, c.name, c.slug,
	p.is_pinned, p.is_featured, p.view_count, p.like_count, p.comment_count, p.bookmark_count, `

// scanPostRow 扫描行。
func scanPostRow(rows pgx.Rows) ([]postRow, error) {
	var out []postRow
	for rows.Next() {
		var r postRow
		if err := rows.Scan(&r.ID, &r.PublicID, &r.Title, &r.Excerpt, &r.PostType, &r.Status,
			&r.BodyRendered, &r.RejectReason, &r.AuthorID, &r.AuthorPID, &r.AuthorName, &r.AuthorDisplay,
			&r.AuthorAvatar, &r.CategoryID, &r.CategoryName, &r.CategorySlug,
			&r.IsPinned, &r.IsFeatured, &r.ViewCount, &r.LikeCount, &r.CommentCount, &r.BookmarkCount,
			&r.IsLiked, &r.IsBookmarked, &r.PublishedAt, &r.LastActivityAt, &r.CreatedAt, &r.EditedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func toItem(r postRow, tags []TagRef) PostItem {
	return PostItem{
		ID: r.PublicID, Title: r.Title, Excerpt: r.Excerpt, PostType: r.PostType, Status: r.Status,
		Author:   Author{ID: r.AuthorPID, Username: r.AuthorName, DisplayName: r.AuthorDisplay, AvatarKey: r.AuthorAvatar},
		Category: CategoryRef{ID: r.CategoryID, Name: r.CategoryName, Slug: r.CategorySlug},
		Tags:     tags,
		IsPinned: r.IsPinned, IsFeatured: r.IsFeatured,
		ViewCount: r.ViewCount, LikeCount: r.LikeCount, CommentCount: r.CommentCount, BookmarkCount: r.BookmarkCount,
		IsLiked: r.IsLiked, IsBookmarked: r.IsBookmarked,
		PublishedAt: r.PublishedAt, LastActivityAt: r.LastActivityAt, CreatedAt: r.CreatedAt,
	}
}

// HotScoreSQL 热度公式（服务端唯一实现，权重集中在 system_configs 可调）。
const HotScoreSQL = `(
	(p.like_count * 3.0 + p.comment_count * 2.0 + p.bookmark_count * 4.0 + p.view_count * 0.1)
	/ POWER(EXTRACT(EPOCH FROM (now_utc() - COALESCE(p.published_at, p.created_at))) / 3600.0 + 2, 1.2)
)`

// publicBase 公开内容基础条件（已发布与锁定帖均公开可读，锁定仅限制新评论）。
const publicBase = `p.status IN ('published', 'locked') AND p.deleted_at IS NULL AND c.status = 'active' AND c.visibility = 'public'`

// FeedParams Feed 查询参数。
type FeedParams struct {
	Tab         string // latest|hot|featured|recommended|following
	ViewerID    int64
	FollowUsers []int64
	FollowCats  []int64
	FollowTags  []int64
	Blocked     []int64
	Limit       int
	Offset      int
}

// Feed 信息流查询。
func (r *Repo) Feed(ctx context.Context, p FeedParams) ([]postRow, int64, error) {
	where := publicBase
	args := []any{}
	switch p.Tab {
	case "following":
		if p.ViewerID == 0 {
			return nil, 0, apperr.ErrUnauthenticated
		}
		where += ` AND (p.author_id = ANY($2) OR p.category_id = ANY($3) OR p.id IN (
			SELECT pt.post_id FROM post_tags pt WHERE pt.tag_id = ANY($4)))
			AND NOT (p.author_id = ANY($5))`
		args = append(args, p.ViewerID, p.FollowUsers, p.FollowCats, p.FollowTags, p.Blocked)
	case "hot", "recommended":
		// 热门/推荐在排序上区分，条件一致
	default: // latest / featured 条件
		if p.Tab == "featured" {
			where += ` AND p.is_featured`
		}
	}
	if p.ViewerID > 0 && p.Tab != "following" {
		args = append(args, p.ViewerID)
		idx := itoa(int64(len(args)))
		where += ` AND NOT EXISTS (SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = $` + idx + ` AND ub.blocked_id = p.author_id)
			   OR (ub.blocked_id = $` + idx + ` AND ub.blocker_id = p.author_id))`
	}

	var countArgs []any
	countArgs = append(countArgs, args...)
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts p JOIN categories c ON c.id = p.category_id WHERE `+where, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := `p.published_at DESC`
	switch p.Tab {
	case "hot":
		order = HotScoreSQL + ` DESC`
	case "recommended":
		order = `(` + HotScoreSQL + ` * CASE WHEN p.published_at > now_utc() - interval '30 days' THEN 1.5 ELSE 1 END) DESC`
	case "featured":
		order = `p.is_pinned DESC, p.published_at DESC`
	}

	args = append(args, p.Limit, p.Offset)
	q := `SELECT ` + itemCols + viewerJoin(p.ViewerID) + `, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id AND u.deleted_at IS NULL
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE ` + where + `
		ORDER BY ` + order + `
		LIMIT $` + itoa(int64(len(args)-1)) + ` OFFSET $` + itoa(int64(len(args)))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, serr := scanPostRow(rows)
	return list, total, serr
}

// ListByCategory 版块帖子列表（置顶优先）。
func (r *Repo) ListByCategory(ctx context.Context, categoryID int64, viewerID int64, sort string, limit, offset int) ([]postRow, int64, error) {
	where := `p.category_id = $1 AND p.status IN ('published', 'locked') AND p.deleted_at IS NULL`
	args := []any{categoryID}
	if viewerID > 0 {
		args = append(args, viewerID)
		idx := itoa(int64(len(args)))
		where += ` AND NOT EXISTS (SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = $` + idx + ` AND ub.blocked_id = p.author_id)
			   OR (ub.blocked_id = $` + idx + ` AND ub.blocker_id = p.author_id))`
	}
	order := `p.is_pinned DESC, p.published_at DESC`
	if sort == "hot" {
		order = `p.is_pinned DESC, ` + HotScoreSQL + ` DESC`
	} else if sort == "latest" {
		order = `p.is_pinned DESC, p.published_at DESC`
	}
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts p WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemCols+viewerJoin(viewerID)+`, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id AND u.deleted_at IS NULL
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY `+order+`
		LIMIT $`+itoa(int64(len(args)-1))+` OFFSET $`+itoa(int64(len(args))), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, serr := scanPostRow(rows)
	return list, total, serr
}

// ListByTag 标签帖子列表。
func (r *Repo) ListByTag(ctx context.Context, tagID int64, viewerID int64, limit, offset int) ([]postRow, int64, error) {
	where := `p.id IN (SELECT post_id FROM post_tags WHERE tag_id = $1) AND ` + publicBase
	args := []any{tagID}
	if viewerID > 0 {
		args = append(args, viewerID)
		idx := itoa(int64(len(args)))
		where += ` AND NOT EXISTS (SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = $` + idx + ` AND ub.blocked_id = p.author_id)
			   OR (ub.blocked_id = $` + idx + ` AND ub.blocker_id = p.author_id))`
	}
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts p JOIN categories c ON c.id = p.category_id WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemCols+viewerJoin(viewerID)+`, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id AND u.deleted_at IS NULL
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY p.published_at DESC LIMIT $`+itoa(int64(len(args)-1))+` OFFSET $`+itoa(int64(len(args))), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, serr := scanPostRow(rows)
	return list, total, serr
}

// ListByAuthor 用户公开帖子（尊重主页隐私设置与拉黑屏蔽）。
func (r *Repo) ListByAuthor(ctx context.Context, authorID, viewerID int64, limit, offset int) ([]postRow, int64, error) {
	if viewerID != authorID {
		var vis string
		if err := r.pool.QueryRow(ctx, `SELECT profile_visibility FROM user_profiles WHERE user_id = $1`, authorID).Scan(&vis); err == nil {
			if vis == "private" {
				return nil, 0, nil
			}
			if vis == "followers" {
				if viewerID == 0 {
					return nil, 0, nil
				}
				var ok bool
				_ = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM user_follows WHERE follower_id = $1 AND following_id = $2)`, viewerID, authorID).Scan(&ok)
				if !ok {
					return nil, 0, nil
				}
			}
		}
		if viewerID > 0 {
			var blocked bool
			_ = r.pool.QueryRow(ctx, `
				SELECT EXISTS(SELECT 1 FROM user_blocks WHERE (blocker_id = $1 AND blocked_id = $2)
				                                          OR (blocker_id = $2 AND blocked_id = $1))`, viewerID, authorID).Scan(&blocked)
			if blocked {
				return nil, 0, nil
			}
		}
	}
	where := `p.author_id = $1 AND ` + publicBase
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts p JOIN categories c ON c.id = p.category_id WHERE `+where, authorID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemCols+viewerJoin(viewerID)+`, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id AND u.deleted_at IS NULL
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY p.published_at DESC LIMIT $2 OFFSET $3`, authorID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, serr := scanPostRow(rows)
	return list, total, serr
}

// GetByPublicID 按 public_id 查询（viewerID 控制互动状态；includeUnpublished 供作者/管理）。
func (r *Repo) GetByPublicID(ctx context.Context, publicID string, viewerID int64) (*postRow, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT p.id, p.public_id::text, p.title, p.excerpt, p.post_type, p.status, p.body_rendered,
		       COALESCE(p.reject_reason,''), p.author_id, u.public_id::text, u.username,
		       COALESCE(up.display_name, u.username), up.avatar_key,
		       p.category_id, c.name, c.slug,
		       p.is_pinned, p.is_featured, p.view_count, p.like_count, p.comment_count, p.bookmark_count,
		       `+viewerJoin(viewerID)+`, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE p.public_id = $1 AND p.deleted_at IS NULL`, publicID)
	var r2 postRow
	err := row.Scan(&r2.ID, &r2.PublicID, &r2.Title, &r2.Excerpt, &r2.PostType, &r2.Status, &r2.BodyRendered,
		&r2.RejectReason, &r2.AuthorID, &r2.AuthorPID, &r2.AuthorName, &r2.AuthorDisplay,
		&r2.AuthorAvatar, &r2.CategoryID, &r2.CategoryName, &r2.CategorySlug,
		&r2.IsPinned, &r2.IsFeatured, &r2.ViewCount, &r2.LikeCount, &r2.CommentCount, &r2.BookmarkCount,
		&r2.IsLiked, &r2.IsBookmarked, &r2.PublishedAt, &r2.LastActivityAt, &r2.CreatedAt, &r2.EditedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("帖子不存在")
	}
	if err != nil {
		return nil, err
	}
	return &r2, nil
}

// GetInternalByPublicID 取内部 ID 与作者/版块（评论等模块使用）。
type PostRef struct {
	ID                 int64
	PublicID           string
	AuthorID           int64
	CategoryID         int64
	CategoryVisibility string
	CategoryStatus     string
	Status             string
	Title              string
}

// RefForComment 评论模块用：取帖子引用（含锁定状态与版块可见性）。
func (r *Repo) RefForComment(ctx context.Context, publicID string) (*PostRef, error) {
	var ref PostRef
	err := r.pool.QueryRow(ctx, `
		SELECT p.id, p.public_id::text, p.author_id, p.category_id, c.visibility, c.status, p.status, p.title
		FROM posts p
		JOIN categories c ON c.id = p.category_id
		WHERE p.public_id = $1 AND p.deleted_at IS NULL`, publicID).
		Scan(&ref.ID, &ref.PublicID, &ref.AuthorID, &ref.CategoryID, &ref.CategoryVisibility, &ref.CategoryStatus, &ref.Status, &ref.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("帖子不存在")
	}
	return &ref, err
}

// CreateTx 事务内创建帖子。
func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, authorID, categoryID int64, req *CreateRequest, status, excerpt, rendered string) (int64, string, error) {
	var id int64
	var publicID string
	err := tx.QueryRow(ctx, `
		INSERT INTO posts (author_id, category_id, post_type, title, body_markdown, body_rendered, excerpt, status, published_at, last_activity_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
		        CASE WHEN $8 = 'published' THEN now_utc() ELSE NULL END,
		        CASE WHEN $8 = 'published' THEN now_utc() ELSE NULL END)
		RETURNING id, public_id::text`,
		authorID, categoryID, req.PostType, req.Title, req.BodyMarkdown, rendered, excerpt, status).Scan(&id, &publicID)
	return id, publicID, err
}

// UpdateTx 事务内更新帖子内容。
func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	set := "updated_at = now_utc(), edited_at = now_utc()"
	args := []any{}
	i := 1
	for col, val := range fields {
		set += ", " + col + " = $" + itoa(int64(i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	_, err := tx.Exec(ctx, `UPDATE posts SET `+set+` WHERE id = $`+itoa(int64(i)), args...)
	return err
}

// SaveRevision 保存修订记录。
func (r *Repo) SaveRevision(ctx context.Context, tx pgx.Tx, postID, editorID int64, title, body, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO post_revisions (post_id, editor_id, title, body_markdown, edit_reason)
		VALUES ($1, $2, $3, $4, $5)`, postID, editorID, title, body, reason)
	return err
}

// GetForUpdate 行锁读取帖子。
func (r *Repo) GetForUpdate(ctx context.Context, tx pgx.Tx, id int64) (*postRow, error) {
	row := tx.QueryRow(ctx, `
		SELECT p.id, p.public_id::text, p.title, p.excerpt, p.post_type, p.status, p.body_rendered,
		       COALESCE(p.reject_reason,''), p.author_id, u.public_id::text, u.username,
		       COALESCE(up.display_name, u.username), up.avatar_key,
		       p.category_id, c.name, c.slug,
		       p.is_pinned, p.is_featured, p.view_count, p.like_count, p.comment_count, p.bookmark_count,
		       FALSE, FALSE, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE p.id = $1 FOR UPDATE OF p`, id)
	var r2 postRow
	err := row.Scan(&r2.ID, &r2.PublicID, &r2.Title, &r2.Excerpt, &r2.PostType, &r2.Status, &r2.BodyRendered,
		&r2.RejectReason, &r2.AuthorID, &r2.AuthorPID, &r2.AuthorName, &r2.AuthorDisplay,
		&r2.AuthorAvatar, &r2.CategoryID, &r2.CategoryName, &r2.CategorySlug,
		&r2.IsPinned, &r2.IsFeatured, &r2.ViewCount, &r2.LikeCount, &r2.CommentCount, &r2.BookmarkCount,
		&r2.IsLiked, &r2.IsBookmarked, &r2.PublishedAt, &r2.LastActivityAt, &r2.CreatedAt, &r2.EditedAt)
	if err != nil {
		return nil, err
	}
	return &r2, nil
}

// MyPosts 我的帖子（含各状态）。
func (r *Repo) MyPosts(ctx context.Context, authorID int64, status string, limit, offset int) ([]postRow, int64, error) {
	where := `p.author_id = $1 AND p.deleted_at IS NULL`
	args := []any{authorID}
	if status != "" {
		where += ` AND p.status = $2`
		args = append(args, status)
	}
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts p WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemCols+`FALSE, FALSE, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY p.created_at DESC LIMIT $`+itoa(int64(len(args)-1))+` OFFSET $`+itoa(int64(len(args))), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, serr := scanPostRow(rows)
	return list, total, serr
}

// Revisions 修订历史。
func (r *Repo) Revisions(ctx context.Context, postID int64) ([]Revision, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT pr.id, pr.title, pr.body_markdown, pr.edit_reason, COALESCE(up.display_name, u.username), pr.created_at
		FROM post_revisions pr
		JOIN users u ON u.id = pr.editor_id
		JOIN user_profiles up ON up.user_id = u.id
		WHERE pr.post_id = $1 ORDER BY pr.created_at DESC`, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Revision
	for rows.Next() {
		var rev Revision
		if err := rows.Scan(&rev.ID, &rev.Title, &rev.BodyMarkdown, &rev.EditReason, &rev.EditorName, &rev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, rows.Err()
}

// IncView 原子增加浏览量。
func (r *Repo) IncView(ctx context.Context, postID int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE posts SET view_count = view_count + 1 WHERE id = $1`, postID)
	return err
}

// TouchActivity 更新最后活跃时间。
func (r *Repo) TouchActivity(ctx context.Context, tx pgx.Tx, postID int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE posts SET last_activity_at = now_utc() WHERE id = $1`, postID)
	return err
}

// SetStatusTx 状态流转（服务端状态机专用）。
func (r *Repo) SetStatusTx(ctx context.Context, tx pgx.Tx, id int64, status, rejectReason string) error {
	setExtra := ""
	switch status {
	case StatusPublished:
		setExtra = `, published_at = COALESCE(published_at, now_utc()), last_activity_at = COALESCE(last_activity_at, now_utc())`
	case StatusRejected:
		setExtra = `, reject_reason = $3`
	}
	q := `UPDATE posts SET status = $2` + setExtra + `, updated_at = now_utc() WHERE id = $1`
	if status == StatusRejected {
		_, err := tx.Exec(ctx, q, id, status, rejectReason)
		return err
	}
	_, err := tx.Exec(ctx, q, id, status)
	return err
}

// ModerateTx 管理动作字段更新（不触碰 edited_at，避免误标「已编辑」）。
func (r *Repo) ModerateTx(ctx context.Context, tx pgx.Tx, id int64, fields map[string]any) error {
	set := "updated_at = now_utc()"
	args := []any{}
	i := 1
	for col, val := range fields {
		set += ", " + col + " = $" + itoa(int64(i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	_, err := tx.Exec(ctx, `UPDATE posts SET `+set+` WHERE id = $`+itoa(int64(i)), args...)
	return err
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string { return likeEscaper.Replace(s) }

// AdminList 后台帖子列表（全状态；allowedCategoryIDs 非 nil 时限定版块范围）。
func (r *Repo) AdminList(ctx context.Context, status, categorySlug, q string, allowedCategoryIDs []int64, limit, offset int) ([]postRow, int64, error) {
	where := `1=1`
	args := []any{}
	if status != "" {
		args = append(args, status)
		where += ` AND p.status = $` + itoa(int64(len(args)))
	}
	if categorySlug != "" {
		args = append(args, categorySlug)
		where += ` AND c.slug = $` + itoa(int64(len(args)))
	}
	if allowedCategoryIDs != nil {
		args = append(args, allowedCategoryIDs)
		where += ` AND p.category_id = ANY($` + itoa(int64(len(args))) + `)`
	}
	if q != "" {
		args = append(args, "%"+escapeLike(q)+"%")
		where += ` AND p.title ILIKE $` + itoa(int64(len(args))) + ` ESCAPE '\'`
	}
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM posts p JOIN categories c ON c.id = p.category_id WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	rows, err := r.pool.Query(ctx, `
		SELECT `+itemCols+`FALSE, FALSE, p.published_at, p.last_activity_at, p.created_at, p.edited_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY p.created_at DESC LIMIT $`+itoa(int64(len(args)-1))+` OFFSET $`+itoa(int64(len(args))), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, serr := scanPostRow(rows)
	return list, total, serr
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

// GetMarkdown 读取原始 Markdown 正文。
func (r *Repo) GetMarkdown(ctx context.Context, postID int64) (string, error) {
	var body string
	err := r.pool.QueryRow(ctx, `SELECT body_markdown FROM posts WHERE id = $1`, postID).Scan(&body)
	return body, err
}
