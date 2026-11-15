// Package interactions 点赞与收藏。幂等依赖数据库唯一约束 + ON CONFLICT。
package interactions

import (
	"context"
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/modules/notifications"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/pagination"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Repo 互动数据访问。
type Repo struct{ pool *pgxpool.Pool }

// NewRepo 构造。
func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// LikePost 点赞帖子，返回是否为新点赞。
func (r *Repo) LikePost(ctx context.Context, userID, postID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO post_likes (user_id, post_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, postID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil // 幂等：已存在
	}
	_, err = r.pool.Exec(ctx, `UPDATE posts SET like_count = like_count + 1 WHERE id = $1`, postID)
	return true, err
}

// UnlikePost 取消点赞。
func (r *Repo) UnlikePost(ctx context.Context, userID, postID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM post_likes WHERE user_id = $1 AND post_id = $2`, userID, postID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE posts SET like_count = GREATEST(0, like_count - 1) WHERE id = $1`, postID)
	return true, err
}

// LikeComment 点赞评论。
func (r *Repo) LikeComment(ctx context.Context, userID, commentID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO comment_likes (user_id, comment_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, commentID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = r.pool.Exec(ctx, `UPDATE comments SET like_count = like_count + 1 WHERE id = $1`, commentID)
	return true, err
}

// UnlikeComment 取消评论点赞。
func (r *Repo) UnlikeComment(ctx context.Context, userID, commentID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2`, userID, commentID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE comments SET like_count = GREATEST(0, like_count - 1) WHERE id = $1`, commentID)
	return true, err
}

// PostRef 帖子引用（通知用）。
type PostRef struct {
	ID       int64
	AuthorID int64
	Title    string
}

// ResolvePost 解析帖子。
func (r *Repo) ResolvePost(ctx context.Context, publicID string) (*PostRef, error) {
	var ref PostRef
	err := r.pool.QueryRow(ctx,
		`SELECT id, author_id, title FROM posts WHERE public_id = $1 AND deleted_at IS NULL`, publicID).
		Scan(&ref.ID, &ref.AuthorID, &ref.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("帖子不存在")
	}
	return &ref, err
}

// ResolveComment 解析评论。
type CommentRef struct {
	ID       int64
	AuthorID int64
	PostID   int64
}

func (r *Repo) ResolveComment(ctx context.Context, publicID string) (*CommentRef, error) {
	var ref CommentRef
	err := r.pool.QueryRow(ctx,
		`SELECT id, author_id, post_id FROM comments WHERE public_id = $1`, publicID).
		Scan(&ref.ID, &ref.AuthorID, &ref.PostID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("评论不存在")
	}
	return &ref, err
}

// ---- 收藏夹 ----

// Folder 收藏夹。
type Folder struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Count     int64  `json:"count"`
}

// ListFolders 收藏夹列表（含默认夹：folder_id IS NULL 的收藏归入 id=0）。
func (r *Repo) ListFolders(ctx context.Context, userID int64) ([]Folder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.id, f.name, f.sort_order, COUNT(b.post_id)
		FROM bookmark_folders f
		LEFT JOIN bookmarks b ON b.folder_id = f.id
		WHERE f.user_id = $1
		GROUP BY f.id ORDER BY f.sort_order, f.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Folder
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.SortOrder, &f.Count); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 默认收藏夹
	var defCount int64
	_ = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM bookmarks WHERE user_id = $1 AND folder_id IS NULL`, userID).Scan(&defCount)
	out = append([]Folder{{ID: 0, Name: "默认收藏夹", SortOrder: -1, Count: defCount}}, out...)
	return out, nil
}

// CreateFolder 创建收藏夹。
func (r *Repo) CreateFolder(ctx context.Context, userID int64, name string, sort int) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx,
		`INSERT INTO bookmark_folders (user_id, name, sort_order) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, sort).Scan(&id)
	return id, err
}

// RenameFolder 重命名。
func (r *Repo) RenameFolder(ctx context.Context, userID, folderID int64, name string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE bookmark_folders SET name = $3 WHERE id = $1 AND user_id = $2`, folderID, userID, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("收藏夹不存在")
	}
	return nil
}

// DeleteFolder 删除收藏夹（收藏回落默认夹）。
func (r *Repo) DeleteFolder(ctx context.Context, userID, folderID int64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx,
		`UPDATE bookmarks SET folder_id = NULL WHERE folder_id = $1 AND user_id = $2`, folderID, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM bookmark_folders WHERE id = $1 AND user_id = $2`, folderID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("收藏夹不存在")
	}
	return tx.Commit(ctx)
}

// BookmarkItem 收藏条目。
type BookmarkItem struct {
	PostID     string `json:"post_id"`
	Title      string `json:"title"`
	Excerpt    string `json:"excerpt"`
	FolderID   int64  `json:"folder_id"`
	FolderName string `json:"folder_name"`
	AuthorName string `json:"author_name"`
	CreatedAt  string `json:"created_at"`
}

// ListBookmarks 收藏列表。
func (r *Repo) ListBookmarks(ctx context.Context, userID, folderID int64, limit, offset int) ([]BookmarkItem, int64, error) {
	where := `b.user_id = $1`
	args := []any{userID}
	if folderID > 0 {
		where += ` AND b.folder_id = $2`
		args = append(args, folderID)
	} else if folderID == 0 {
		// 默认夹：folder_id IS NULL
		where += ` AND b.folder_id IS NULL`
	}
	var total int64
	cnt := "SELECT COUNT(*) FROM bookmarks b WHERE " + where
	if folderID < 0 {
		// -1 表示全部
		where = "b.user_id = $1"
		cnt = "SELECT COUNT(*) FROM bookmarks b WHERE " + where
		args = args[:1]
	}
	if err := r.pool.QueryRow(ctx, cnt, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT p.public_id::text, p.title, p.excerpt, COALESCE(b.folder_id, 0), COALESCE(f.name, '默认收藏夹'),
	             COALESCE(up.display_name, u.username), b.created_at
		FROM bookmarks b
		JOIN posts p ON p.id = b.post_id AND p.status = 'published' AND p.deleted_at IS NULL
		JOIN users u ON u.id = p.author_id
		JOIN user_profiles up ON up.user_id = u.id
		LEFT JOIN bookmark_folders f ON f.id = b.folder_id
		WHERE ` + where + `
		ORDER BY b.created_at DESC LIMIT ` + strconv.Itoa(limit) + ` OFFSET ` + strconv.Itoa(offset)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []BookmarkItem
	for rows.Next() {
		var it BookmarkItem
		if err := rows.Scan(&it.PostID, &it.Title, &it.Excerpt, &it.FolderID, &it.FolderName, &it.AuthorName, &it.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}

// AddBookmark 收藏（幂等）。
func (r *Repo) AddBookmark(ctx context.Context, userID, postID, folderID int64) (bool, error) {
	var fid *int64
	if folderID > 0 {
		// 校验归属
		var owner int64
		err := r.pool.QueryRow(ctx,
			`SELECT user_id FROM bookmark_folders WHERE id = $1`, folderID).Scan(&owner)
		if err != nil {
			return false, apperr.NotFound("收藏夹不存在")
		}
		if owner != userID {
			return false, apperr.ErrForbidden
		}
		fid = &folderID
	}
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO bookmarks (user_id, post_id, folder_id) VALUES ($1, $2, $3) ON CONFLICT (user_id, post_id) DO NOTHING`,
		userID, postID, fid)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = r.pool.Exec(ctx, `UPDATE posts SET bookmark_count = bookmark_count + 1 WHERE id = $1`, postID)
	return true, err
}

// RemoveBookmark 取消收藏。
func (r *Repo) RemoveBookmark(ctx context.Context, userID, postID int64) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM bookmarks WHERE user_id = $1 AND post_id = $2`, userID, postID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE posts SET bookmark_count = GREATEST(0, bookmark_count - 1) WHERE id = $1`, postID)
	return true, err
}

// ---- HTTP ----

// Handler 互动 HTTP 层。
type Handler struct {
	repo *Repo
}

// NewHandler 构造。
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{repo: NewRepo(pool)}
}

// Register 注册互动路由。
func (h *Handler) Register(rg *gin.RouterGroup) {
	g := rg.Group("")
	g.Use(httpx.RequireAuth(), httpx.RequireVerified())
	{
		g.POST("/posts/:id/like", h.likePost)
		g.DELETE("/posts/:id/like", h.unlikePost)
		g.POST("/comments/:id/like", h.likeComment)
		g.DELETE("/comments/:id/like", h.unlikeComment)
		g.POST("/posts/:id/bookmark", h.addBookmark)
		g.DELETE("/posts/:id/bookmark", h.removeBookmark)
		g.GET("/me/bookmarks", h.listBookmarks)
		g.GET("/me/bookmark-folders", h.listFolders)
		g.POST("/me/bookmark-folders", h.createFolder)
		g.PATCH("/me/bookmark-folders/:id", h.renameFolder)
		g.DELETE("/me/bookmark-folders/:id", h.deleteFolder)
	}
}

func (h *Handler) likePost(c *gin.Context) {
	u := httpx.CurrentUser(c)
	ref, err := h.repo.ResolvePost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	inserted, err := h.repo.LikePost(c.Request.Context(), u.ID, ref.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if inserted {
		tx, _ := h.repo.pool.Begin(c.Request.Context())
		_ = notifications.EmitTx(c.Request.Context(), tx, notifications.Payload{
			RecipientID: ref.AuthorID, ActorID: u.ID, Type: notifications.TypeLikePost,
			ResourceType: "post", ResourceID: ref.ID,
			Title:     "有人赞了你的帖子「" + truncateT(ref.Title, 40) + "」",
			DedupeKey: "likepost:" + strconv.FormatInt(u.ID, 10) + ":" + strconv.FormatInt(ref.ID, 10),
		})
		_ = tx.Commit(c.Request.Context())
	}
	respond.OK(c, gin.H{"liked": true, "already": !inserted})
}

func (h *Handler) unlikePost(c *gin.Context) {
	u := httpx.CurrentUser(c)
	ref, err := h.repo.ResolvePost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	removed, err := h.repo.UnlikePost(c.Request.Context(), u.ID, ref.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"liked": false, "removed": removed})
}

func (h *Handler) likeComment(c *gin.Context) {
	u := httpx.CurrentUser(c)
	ref, err := h.repo.ResolveComment(c.Request.Context(), c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	inserted, err := h.repo.LikeComment(c.Request.Context(), u.ID, ref.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if inserted {
		tx, _ := h.repo.pool.Begin(c.Request.Context())
		_ = notifications.EmitTx(c.Request.Context(), tx, notifications.Payload{
			RecipientID: ref.AuthorID, ActorID: u.ID, Type: notifications.TypeLikeComment,
			ResourceType: "comment", ResourceID: ref.ID,
			Title:     "有人赞了你的评论",
			DedupeKey: "likecmt:" + strconv.FormatInt(u.ID, 10) + ":" + strconv.FormatInt(ref.ID, 10),
		})
		_ = tx.Commit(c.Request.Context())
	}
	respond.OK(c, gin.H{"liked": true, "already": !inserted})
}

func (h *Handler) unlikeComment(c *gin.Context) {
	u := httpx.CurrentUser(c)
	ref, err := h.repo.ResolveComment(c.Request.Context(), c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	removed, err := h.repo.UnlikeComment(c.Request.Context(), u.ID, ref.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"liked": false, "removed": removed})
}

type bookmarkReq struct {
	FolderID int64 `json:"folder_id"`
}

func (h *Handler) addBookmark(c *gin.Context) {
	u := httpx.CurrentUser(c)
	ref, err := h.repo.ResolvePost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	var req bookmarkReq
	_ = c.ShouldBindJSON(&req) // body 可选
	inserted, err := h.repo.AddBookmark(c.Request.Context(), u.ID, ref.ID, req.FolderID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"bookmarked": true, "already": !inserted, "message": "已收藏"})
}

func (h *Handler) removeBookmark(c *gin.Context) {
	u := httpx.CurrentUser(c)
	ref, err := h.repo.ResolvePost(c.Request.Context(), c.Param("id"))
	if err != nil {
		respond.Fail(c, err)
		return
	}
	removed, err := h.repo.RemoveBookmark(c.Request.Context(), u.ID, ref.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"bookmarked": false, "removed": removed, "message": "已取消收藏"})
}

func (h *Handler) listFolders(c *gin.Context) {
	u := httpx.CurrentUser(c)
	items, err := h.repo.ListFolders(c.Request.Context(), u.ID)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"items": items})
}

func (h *Handler) listBookmarks(c *gin.Context) {
	u := httpx.CurrentUser(c)
	pq, err := pagination.Parse(c)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	folderID := int64(-1)
	if f := c.Query("folder_id"); f != "" {
		if v, err := strconv.ParseInt(f, 10, 64); err == nil {
			folderID = v
		}
	}
	items, total, err := h.repo.ListBookmarks(c.Request.Context(), u.ID, folderID, pq.PageSize, pq.Offset)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	if items == nil {
		items = []BookmarkItem{}
	}
	respond.Page(c, items, pq.Page, pq.PageSize, total)
}

func (h *Handler) createFolder(c *gin.Context) {
	u := httpx.CurrentUser(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请求体格式不正确"))
		return
	}
	name := trimSpace(req.Name)
	if name == "" || len([]rune(name)) > 50 {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "收藏夹名需为 1-50 字"))
		return
	}
	id, err := h.repo.CreateFolder(c.Request.Context(), u.ID, name, 0)
	if err != nil {
		respond.Fail(c, err)
		return
	}
	respond.Created(c, gin.H{"id": id})
}

func (h *Handler) renameFolder(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "ID 不合法"))
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请求体格式不正确"))
		return
	}
	name := trimSpace(req.Name)
	if name == "" || len([]rune(name)) > 50 {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "收藏夹名需为 1-50 字"))
		return
	}
	if err := h.repo.RenameFolder(c.Request.Context(), u.ID, id, name); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "收藏夹已更新"})
}

func (h *Handler) deleteFolder(c *gin.Context) {
	u := httpx.CurrentUser(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "ID 不合法"))
		return
	}
	if err := h.repo.DeleteFolder(c.Request.Context(), u.ID, id); err != nil {
		respond.Fail(c, err)
		return
	}
	respond.OK(c, gin.H{"message": "收藏夹已删除，收藏已移入默认收藏夹"})
}

func truncateT(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
