// Package uploads 文件上传：白名单校验 + 魔数校验 + 大小限制 + 随机存储名。
package uploads

import (
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/platform/storage"
	"github.com/yanji-community/backend/internal/shared/apperr"
	httpx "github.com/yanji-community/backend/internal/shared/httpx"
	"github.com/yanji-community/backend/internal/shared/respond"
)

// Service 上传服务。
type Service struct {
	store    storage.Storage
	pool     *pgxpool.Pool
	logger   *slog.Logger
	maxBytes int64
}

// New 构造。
func New(store storage.Storage, pool *pgxpool.Pool, logger *slog.Logger, maxBytes int64) *Service {
	return &Service{store: store, pool: pool, logger: logger, maxBytes: maxBytes}
}

// Handler 上传 HTTP 层。
type Handler struct{ svc *Service }

// NewHandler 构造。
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register POST /uploads
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.POST("/uploads", httpx.RequireAuth(), httpx.RequireVerified(), h.Upload)
}

// Upload multipart 上传（字段名 file）。
func (h *Handler) Upload(c *gin.Context) {
	u := httpx.CurrentUser(c)
	fh, err := c.FormFile("file")
	if err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请选择要上传的文件"))
		return
	}
	ext, err := storage.ValidateImage(fh, h.svc.maxBytes)
	if err != nil {
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	key := storage.RandomKey(ext)
	f, err := fh.Open()
	if err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	defer f.Close()
	if err := h.svc.store.Put(c.Request.Context(), key, f); err != nil {
		respond.Fail(c, apperr.Internal(err))
		return
	}
	// 记录附件（未关联帖子；发布时通过 Markdown 引用）
	_, err = h.svc.pool.Exec(c.Request.Context(), `
		INSERT INTO post_attachments (uploader_id, storage_key, original_name, mime_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5)`,
		u.ID, key, filepath.Base(fh.Filename), fh.Header.Get("Content-Type"), fh.Size)
	if err != nil {
		h.svc.logger.Error("附件记录失败", "err", err)
	}
	respond.Created(c, gin.H{
		"key": key,
		"url": h.svc.store.PublicURL(key),
	})
}

// ServeLocal 本地存储静态文件服务（/files/{key}）。
func ServeLocal(store *storage.LocalStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		path, ok := store.LocalPath(key)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		// 依据扩展名推断内容类型（随机存储名保证安全）
		ext := filepath.Ext(key)
		ct := "application/octet-stream"
		if m, ok := storage.ImageExtWhitelist[ext]; ok {
			ct = m
		}
		c.Header("Content-Type", ct)
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(path)
	}
}

var _ multipart.FileHeader
