// Package uploads 文件上传：白名单校验 + 魔数校验 + 大小限制 + 随机存储名。
package uploads

import (
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

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

// Upload multipart 上传（字段名 file；入口强制 MaxBytesReader 防止大请求写满磁盘）。
func (h *Handler) Upload(c *gin.Context) {
	u := httpx.CurrentUser(c)
	// 预留 1MB multipart 边界头开销，超出直接中止读取
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.maxBytes+(1<<20))
	fh, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "上传文件体积超过服务端限制"))
			return
		}
		respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", "请选择要上传的文件"))
		return
	}
	ext, err := storage.ValidateImage(fh, h.svc.maxBytes)
	if err != nil {
		if errors.Is(err, storage.ErrInvalidUpload) {
			respond.Fail(c, apperr.BadRequest("VALIDATION_ERROR", strings.TrimPrefix(err.Error(), storage.ErrInvalidUpload.Error()+": ")))
			return
		}
		respond.Fail(c, apperr.Internal(err))
		return
	}
	canonicalMime := storage.ImageExtWhitelist[ext]
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
	// 记录附件（使用白名单推导的可信 MIME，不信任客户端上传头）
	_, err = h.svc.pool.Exec(c.Request.Context(), `
		INSERT INTO post_attachments (uploader_id, storage_key, original_name, mime_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5)`,
		u.ID, key, filepath.Base(fh.Filename), canonicalMime, fh.Size)
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
		ext := filepath.Ext(key)
		ct, ok := storage.ImageExtWhitelist[ext]
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		path, ok := store.LocalPath(key)
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Type", ct)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "default-src 'none'; img-src 'self'; sandbox")
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(path)
	}
}

var _ multipart.FileHeader
