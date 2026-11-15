// Package search 统一搜索：帖子（全文检索+高亮）、用户、版块、标签。
package search

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Service 搜索服务。
type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{pool: pool, logger: logger}
}

// Handler 搜索 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register GET /search
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/search", h.Search)
}

// PostResult 帖子搜索结果（含高亮）。
type PostResult struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Highlight    string    `json:"highlight"`
	AuthorName   string    `json:"author_name"`
	CategorySlug string    `json:"category_slug"`
	CategoryName string    `json:"category_name"`
	LikeCount    int64     `json:"like_count"`
	CommentCount int64     `json:"comment_count"`
	PublishedAt  time.Time `json:"published_at"`
}

// UserResult 用户结果。
type UserResult struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
	Bio         string `json:"bio"`
}

// CategoryResult / TagResult。
type CategoryResult struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	PostCount   int64  `json:"post_count"`
}

type TagResult struct {
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	PostCount int64  `json:"post_count"`
}

// Search GET /search?q=&type=&sort=&category=&tag=&author=&from=&to=
func (h *Handler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	stype := c.DefaultQuery("type", "post")
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if q == "" {
		respond.OK(c, emptyResult(stype))
		return
	}
	if len([]rune(q)) > 100 {
		respond.Fail(c, errInvalidQuery)
		return
	}
	var viewerID int64
	if u := httpx.CurrentUser(c); u != nil {
		viewerID = u.ID
	}

	switch stype {
	case "post":
		h.searchPosts(c, q, pq, viewerID)
	case "user":
		h.searchUsers(c, q, pq)
	case "category":
		h.searchCategories(c, q)
	case "tag":
		h.searchTags(c, q)
	default:
		respond.Fail(c, errInvalidType)
	}
}

func (h *Handler) searchPosts(c *gin.Context, q string, pq pagination.Query, viewerID int64) {
	sort := c.DefaultQuery("sort", "relevance")
	order := `ts_rank(p.tsv, query) DESC`
	switch sort {
	case "latest":
		order = `p.published_at DESC`
	case "hot":
		order = `(p.like_count*3.0 + p.comment_count*2.0 + p.bookmark_count*4.0 + p.view_count*0.1)
		         / POWER(EXTRACT(EPOCH FROM (now_utc() - p.published_at))/3600.0 + 2, 1.2) DESC`
	}
	where := `p.status = 'published' AND p.deleted_at IS NULL
	          AND c.status = 'active' AND c.visibility = 'public'
	          AND p.tsv @@ websearch_to_tsquery('simple', $1)`
	args := []any{q}
	if cat := c.Query("category"); cat != "" {
		args = append(args, cat)
		where += ` AND c.slug = $` + strconv.Itoa(len(args))
	}
	if tag := c.Query("tag"); tag != "" {
		args = append(args, tag)
		where += ` AND p.id IN (SELECT pt.post_id FROM post_tags pt JOIN tags t ON t.id = pt.tag_id WHERE t.slug = $` + strconv.Itoa(len(args)) + `)`
	}
	if author := c.Query("author"); author != "" {
		args = append(args, author)
		where += ` AND u.username = $` + strconv.Itoa(len(args))
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse("2006-01-02", from); err == nil {
			args = append(args, t)
			where += ` AND p.published_at >= $` + strconv.Itoa(len(args))
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse("2006-01-02", to); err == nil {
			args = append(args, t.AddDate(0, 0, 1))
			where += ` AND p.published_at < $` + strconv.Itoa(len(args))
		}
	}
	if viewerID > 0 {
		args = append(args, viewerID)
		where += ` AND NOT EXISTS (SELECT 1 FROM user_blocks ub
			WHERE (ub.blocker_id = $` + strconv.Itoa(len(args)) + ` AND ub.blocked_id = p.author_id)
			   OR (ub.blocked_id = $` + strconv.Itoa(len(args)) + ` AND ub.blocker_id = p.author_id))`
	}

	var total int64
	if err := h.svc.pool.QueryRow(c.Request.Context(),
		`SELECT COUNT(*) FROM posts p JOIN users u ON u.id = p.author_id JOIN categories c ON c.id = p.category_id WHERE `+where, args...).Scan(&total); err != nil {
		respond.Fail(c, err)
		return
	}
	args = append(args, pq.PageSize, pq.Offset)
	headline := `ts_headline('simple', COALESCE(p.body_markdown, ''), websearch_to_tsquery('simple', $1),
	              'StartSel=<mark>,StopSel=</mark>,MaxFragments=2,MaxWords=32,MinWords=16,FragmentDelimiter= … ')`
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT p.public_id::text, p.title, `+headline+`,
		       COALESCE(up.display_name, u.username), c.slug, c.name,
		       p.like_count, p.comment_count, p.published_at
		FROM posts p
		JOIN users u ON u.id = p.author_id
		JOIN user_profiles up ON up.user_id = u.id
		JOIN categories c ON c.id = p.category_id
		WHERE `+where+`
		ORDER BY `+order+`
		LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	items := []PostResult{}
	for rows.Next() {
		var it PostResult
		if err := rows.Scan(&it.ID, &it.Title, &it.Highlight, &it.AuthorName, &it.CategorySlug,
			&it.CategoryName, &it.LikeCount, &it.CommentCount, &it.PublishedAt); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, it)
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

func (h *Handler) searchUsers(c *gin.Context, q string, pq pagination.Query) {
	pattern := `%` + q + `%`
	var total int64
	if err := h.svc.pool.QueryRow(c.Request.Context(), `
		SELECT COUNT(*) FROM users u JOIN user_profiles p ON p.user_id = u.id
		WHERE u.deleted_at IS NULL AND u.status IN ('active','pending')
		  AND (u.username ILIKE $1 OR p.display_name ILIKE $1)`, pattern).Scan(&total); err != nil {
		respond.Fail(c, err)
		return
	}
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT u.public_id::text, u.username, COALESCE(p.display_name, u.username), p.avatar_key, p.bio
		FROM users u JOIN user_profiles p ON p.user_id = u.id
		WHERE u.deleted_at IS NULL AND u.status IN ('active','pending')
		  AND (u.username ILIKE $1 OR p.display_name ILIKE $1)
		ORDER BY u.username LIMIT $2 OFFSET $3`, pattern, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	items := []UserResult{}
	for rows.Next() {
		var it UserResult
		if err := rows.Scan(&it.ID, &it.Username, &it.DisplayName, &it.AvatarKey, &it.Bio); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, it)
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

func (h *Handler) searchCategories(c *gin.Context, q string) {
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT name, slug, description, post_count
		FROM categories
		WHERE deleted_at IS NULL AND status = 'active' AND visibility = 'public'
		  AND (name ILIKE '%' || $1 || '%' OR description ILIKE '%' || $1 || '%')
		ORDER BY post_count DESC LIMIT 20`, q)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	items := []CategoryResult{}
	for rows.Next() {
		var it CategoryResult
		if err := rows.Scan(&it.Name, &it.Slug, &it.Description, &it.PostCount); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, it)
	}
	respond.OK(c, gin.H{"items": items, "total": len(items)})
}

func (h *Handler) searchTags(c *gin.Context, q string) {
	rows, err := h.svc.pool.Query(c.Request.Context(), `
		SELECT name, slug, post_count FROM tags
		WHERE status = 'active' AND (name ILIKE '%' || $1 || '%' OR slug ILIKE '%' || $1 || '%')
		ORDER BY post_count DESC LIMIT 20`, q)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	defer rows.Close()
	items := []TagResult{}
	for rows.Next() {
		var it TagResult
		if err := rows.Scan(&it.Name, &it.Slug, &it.PostCount); err != nil {
			respond.Fail(c, err)
			return
		}
		items = append(items, it)
	}
	respond.OK(c, gin.H{"items": items, "total": len(items)})
}

func emptyResult(stype string) map[string]any {
	base := map[string]any{"items": []any{}, "page": 1, "page_size": 20, "total": int64(0), "has_more": false}
	return map[string]any{"type": stype, "items": []any{}, "page": 1, "page_size": 20, "total": int64(0), "has_more": false, "base": base}
}

var (
	errInvalidQuery = apperr.BadRequest("VALIDATION_ERROR", "搜索词过长")
	errInvalidType  = apperr.BadRequest("VALIDATION_ERROR", "搜索类型不合法")
)
