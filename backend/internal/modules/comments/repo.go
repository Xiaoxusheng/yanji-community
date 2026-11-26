// Package comments 评论：楼中楼（深度 3）、引用回复、编辑删除、@提及。
package comments

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
	"github.com/yanji-community/backend/internal/shared/markdown"
)

// MaxDepth 最大楼层深度：0/1/2 共 3 层。
const MaxDepth = 2

var mentionRe = regexp.MustCompile(`@([a-zA-Z0-9_]{3,20})`)

// Comment 评论。
type Comment struct {
	ID           string     `json:"id"`
	Accepted     bool       `json:"is_accepted_answer"`
	PostID       string     `json:"post_id"`
	ParentID     string     `json:"parent_id,omitempty"`
	QuoteID      string     `json:"quote_id,omitempty"`
	QuoteAuthor  string     `json:"quote_author,omitempty"`
	QuoteBody    string     `json:"quote_body,omitempty"`
	Depth        int        `json:"depth"`
	Author       Author     `json:"author"`
	BodyMarkdown string     `json:"body_markdown,omitempty"` // 仅作者本人返回，供无损编辑
	BodyRendered string     `json:"body_rendered"`
	Status       string     `json:"status"`
	LikeCount    int64      `json:"like_count"`
	ReplyCount   int64      `json:"reply_count"`
	IsLiked      bool       `json:"is_liked"`
	IsAuthor     bool       `json:"is_author"`
	EditedAt     *time.Time `json:"edited_at"`
	CreatedAt    time.Time  `json:"created_at"`
	Replies      []Comment  `json:"replies,omitempty"`
}

// Author 评论作者。
type Author struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
}

// Repo 评论数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

type row struct {
	ID            int64
	PublicID      string
	PostID        int64
	RootID        *int64
	ParentID      *int64
	QuoteID       *int64
	QuoteAuthor   *string
	QuoteBody     *string
	Depth         int
	AuthorID      int64
	AuthorPID     string
	AuthorName    string
	AuthorDisplay string
	AuthorAvatar  string
	BodyMarkdown  string
	BodyRendered  string
	Status        string
	LikeCount     int64
	ReplyCount    int64
	Accepted      bool
	CreatedAt     time.Time
	EditedAt      *time.Time
}

const cols = `cm.id, cm.public_id::text, cm.post_id, cm.root_id, cm.parent_id, cm.quote_comment_id,
	COALESCE(qa.username, ''),
	CASE WHEN q.status = 'published' THEN COALESCE(q.body_markdown, '') ELSE '' END,
	cm.depth, cm.author_id, u.public_id::text, u.username,
	COALESCE(up.display_name, u.username), up.avatar_key,
	cm.body_markdown, cm.body_rendered, cm.status, cm.like_count, cm.reply_count, cm.is_accepted_answer, cm.created_at, cm.edited_at`

const joins = `FROM comments cm
	JOIN users u ON u.id = cm.author_id AND u.deleted_at IS NULL
	JOIN user_profiles up ON up.user_id = u.id
	LEFT JOIN comments q ON q.id = cm.quote_comment_id
	LEFT JOIN users qa ON qa.id = q.author_id`

func (r *row) toComment(viewerID int64, postAuthorID int64) Comment {
	bodyRendered := r.BodyRendered
	if r.Status == "deleted" || r.Status == "hidden" {
		bodyRendered = ""
	}
	c := Comment{
		ID: r.PublicID, Depth: r.Depth,
		Author:       Author{ID: r.AuthorPID, Username: r.AuthorName, DisplayName: r.AuthorDisplay, AvatarKey: r.AuthorAvatar},
		BodyRendered: bodyRendered, Status: r.Status,
		LikeCount: r.LikeCount, ReplyCount: r.ReplyCount, Accepted: r.Accepted,
		CreatedAt: r.CreatedAt, EditedAt: r.EditedAt,
	}
	if r.ParentID != nil {
		c.ParentID = strconv.FormatInt(*r.ParentID, 10)
	}
	if r.QuoteID != nil {
		c.QuoteID = strconv.FormatInt(*r.QuoteID, 10)
		c.QuoteAuthor = deref(r.QuoteAuthor)
		if r.QuoteBody != nil {
			c.QuoteBody = truncateText(*r.QuoteBody, 100)
		}
	}
	if viewerID > 0 {
		c.IsAuthor = r.AuthorID == viewerID
		if c.IsAuthor && r.Status == "published" {
			c.BodyMarkdown = r.BodyMarkdown
		}
	}
	return c
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func truncateText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// ListTree 顶层分页 + 楼中楼展开。
func (r *Repo) ListTree(ctx context.Context, postID int64, viewerID int64, sort string, limit, offset int) ([]row, int64, error) {
	where := `cm.post_id = $1 AND cm.parent_id IS NULL AND cm.status IN ('published','hidden','deleted')`
	order := `cm.created_at ASC`
	if sort == "hot" {
		order = `cm.like_count DESC, cm.created_at ASC`
	}
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM comments cm WHERE cm.post_id = $1 AND cm.parent_id IS NULL AND cm.status = 'published'`, postID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+cols+` `+joins+`
		WHERE `+where+`
		ORDER BY `+order+` LIMIT $2 OFFSET $3`, postID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var roots []row
	for rows.Next() {
		var cm row
		if err := rows.Scan(&cm.ID, &cm.PublicID, &cm.PostID, &cm.RootID, &cm.ParentID, &cm.QuoteID,
			&cm.QuoteAuthor, &cm.QuoteBody, &cm.Depth, &cm.AuthorID, &cm.AuthorPID, &cm.AuthorName,
			&cm.AuthorDisplay, &cm.AuthorAvatar, &cm.BodyMarkdown, &cm.BodyRendered, &cm.Status, &cm.LikeCount,
			&cm.ReplyCount, &cm.Accepted, &cm.CreatedAt, &cm.EditedAt); err != nil {
			return nil, 0, err
		}
		roots = append(roots, cm)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(roots) == 0 {
		return roots, total, nil
	}
	rootIDs := make([]int64, 0, len(roots))
	for _, rt := range roots {
		rootIDs = append(rootIDs, rt.ID)
	}
	replyRows, err := r.pool.Query(ctx, `
		SELECT `+cols+` `+joins+`
		WHERE cm.root_id = ANY($1) AND cm.parent_id IS NOT NULL AND cm.status IN ('published','hidden','deleted')
		ORDER BY cm.created_at ASC`, rootIDs)
	if err != nil {
		return nil, 0, err
	}
	defer replyRows.Close()
	var replies []row
	for replyRows.Next() {
		var cm row
		if err := replyRows.Scan(&cm.ID, &cm.PublicID, &cm.PostID, &cm.RootID, &cm.ParentID, &cm.QuoteID,
			&cm.QuoteAuthor, &cm.QuoteBody, &cm.Depth, &cm.AuthorID, &cm.AuthorPID, &cm.AuthorName,
			&cm.AuthorDisplay, &cm.AuthorAvatar, &cm.BodyMarkdown, &cm.BodyRendered, &cm.Status, &cm.LikeCount,
			&cm.ReplyCount, &cm.Accepted, &cm.CreatedAt, &cm.EditedAt); err != nil {
			return nil, 0, err
		}
		replies = append(replies, cm)
	}
	// 合并：根行 + 其楼中楼
	all := append(roots, replies...)
	return all, total, replyRows.Err()
}

// GetByPublicID 按 public_id 取评论。
func (r *Repo) GetByPublicID(ctx context.Context, publicID string) (*row, error) {
	var cm row
	err := r.pool.QueryRow(ctx, `SELECT `+cols+` `+joins+` WHERE cm.public_id = $1`, publicID).
		Scan(&cm.ID, &cm.PublicID, &cm.PostID, &cm.RootID, &cm.ParentID, &cm.QuoteID,
			&cm.QuoteAuthor, &cm.QuoteBody, &cm.Depth, &cm.AuthorID, &cm.AuthorPID, &cm.AuthorName,
			&cm.AuthorDisplay, &cm.AuthorAvatar, &cm.BodyMarkdown, &cm.BodyRendered, &cm.Status, &cm.LikeCount,
			&cm.ReplyCount, &cm.Accepted, &cm.CreatedAt, &cm.EditedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("评论不存在")
	}
	if err != nil {
		return nil, err
	}
	return &cm, nil
}

// InsertTx 事务内创建评论（显式落盘审核状态），返回 id + public_id。
func (r *Repo) InsertTx(ctx context.Context, tx pgx.Tx, postID, authorID int64, parentID, rootID, quoteID *int64, depth int, bodyMarkdown, rendered, status string) (int64, string, error) {
	if status == "" {
		status = "published"
	}
	var id int64
	var publicID string
	err := tx.QueryRow(ctx, `
		INSERT INTO comments (post_id, author_id, parent_id, root_id, quote_comment_id, depth, body_markdown, body_rendered, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, public_id::text`,
		postID, authorID, parentID, rootID, quoteID, depth, bodyMarkdown, rendered, status).Scan(&id, &publicID)
	return id, publicID, err
}

// UpdateTx 编辑评论（newStatus 非空时同步更新审核状态）。
func (r *Repo) UpdateTx(ctx context.Context, tx pgx.Tx, id int64, bodyMarkdown, rendered, newStatus string) error {
	if newStatus != "" {
		_, err := tx.Exec(ctx, `
			UPDATE comments SET body_markdown = $2, body_rendered = $3, status = $4, edited_at = now_utc(), updated_at = now_utc()
			WHERE id = $1`, id, bodyMarkdown, rendered, newStatus)
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE comments SET body_markdown = $2, body_rendered = $3, edited_at = now_utc(), updated_at = now_utc()
		WHERE id = $1`, id, bodyMarkdown, rendered)
	return err
}

// SoftDeleteTx 软删除评论并维护计数。
func (r *Repo) SoftDeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	if _, err := tx.Exec(ctx,
		`UPDATE comments SET status = 'deleted', deleted_at = now_utc() WHERE id = $1 AND status <> 'deleted'`, id); err != nil {
		return err
	}
	return nil
}

// IncReplyCount 父评论回复计数。
func (r *Repo) IncReplyCount(ctx context.Context, tx pgx.Tx, commentID int64, delta int) error {
	_, err := tx.Exec(ctx,
		`UPDATE comments SET reply_count = GREATEST(0, reply_count + $2) WHERE id = $1`, commentID, delta)
	return err
}

// ResolveMentions 解析 @提及 的用户 ID。
func (r *Repo) ResolveMentions(ctx context.Context, usernames []string) ([]int64, error) {
	if len(usernames) == 0 {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM users WHERE username = ANY($1) AND deleted_at IS NULL`, usernames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Render 评论 Markdown 渲染（导出供 service 使用）。
func Render(body string) (string, error) { return markdown.Render(body) }

// HasRoot 是否有根评论 ID。
func (r *row) HasRoot() bool { return r.RootID != nil }

// RootVal 返回根评论 ID。
func (r *row) RootVal() int64 {
	if r.RootID == nil {
		return 0
	}
	return *r.RootID
}
