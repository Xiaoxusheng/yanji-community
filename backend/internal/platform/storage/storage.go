// Package storage 提供文件存储抽象：开发环境本地磁盘，生产可切换 S3 兼容对象存储。
package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrInvalidUpload 表示上传校验失败（扩展名/MIME/内容/大小）。
var ErrInvalidUpload = errors.New("invalid upload")

// Storage 文件存储统一接口。
type Storage interface {
	// Put 保存文件并返回存储 key。
	Put(ctx context.Context, key string, r io.Reader) error
	Delete(ctx context.Context, key string) error
	PublicURL(key string) string
}

// ---- 内容校验 ----

// ImageExtWhitelist 允许上传的图片扩展名。
var ImageExtWhitelist = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".gif": "image/gif", ".webp": "image/webp",
}

// ValidateImage 对上传文件执行四重校验：扩展名白名单、multipart MIME、实际文件头（magic bytes）、大小。
// 返回规范化后的扩展名（含点）。
func ValidateImage(fh *multipart.FileHeader, maxBytes int64) (string, error) {
	if fh.Size <= 0 || fh.Size > maxBytes {
		return "", fmt.Errorf("%w: 文件大小超出限制", ErrInvalidUpload)
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	wantMime, ok := ImageExtWhitelist[ext]
	if !ok {
		return "", fmt.Errorf("%w: 不支持的文件类型 %q", ErrInvalidUpload, ext)
	}
	if fh.Header.Get("Content-Type") != "" {
		ct := fh.Header.Get("Content-Type")
		if i := strings.Index(ct, ";"); i > 0 {
			ct = ct[:i]
		}
		ct = strings.TrimSpace(strings.ToLower(ct))
		if ct != "application/octet-stream" && ct != wantMime {
			return "", fmt.Errorf("%w: Content-Type 与扩展名不一致", ErrInvalidUpload)
		}
	}
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	defer f.Close()
	head := make([]byte, 12)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("读取上传文件失败: %w", err)
	}
	if n == 0 {
		return "", fmt.Errorf("%w: 文件为空", ErrInvalidUpload)
	}
	if !matchMagic(head[:n], ext) {
		return "", fmt.Errorf("%w: 文件内容与声明类型不符", ErrInvalidUpload)
	}
	return ext, nil
}

func matchMagic(b []byte, ext string) bool {
	switch ext {
	case ".jpg", ".jpeg":
		return len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
	case ".png":
		// 完整 8 字节 PNG 文件头签名
		return len(b) >= 8 &&
			b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G' &&
			b[4] == 0x0D && b[5] == 0x0A && b[6] == 0x1A && b[7] == 0x0A
	case ".gif":
		return len(b) >= 6 && (string(b[:6]) == "GIF87a" || string(b[:6]) == "GIF89a")
	case ".webp":
		return len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP"
	}
	return false
}

// RandomKey 生成随机存储 key：uploads/2006/01/ab12cd....{ext}
// 不信任用户原始文件名，杜绝路径穿越。
func RandomKey(ext string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// 时间兜底，正常不会到达
		return fmt.Sprintf("uploads/%s/fallback%s", time.Now().UTC().Format("2006/01"), ext)
	}
	now := time.Now().UTC()
	return fmt.Sprintf("uploads/%s/%s%s", now.Format("2006/01"), hex.EncodeToString(buf), ext)
}

// ---- 本地磁盘实现 ----

// LocalStorage 本地磁盘存储（开发环境默认）。
type LocalStorage struct {
	dir     string
	urlBase string // 如 /files
}

// NewLocalStorage 创建本地存储，目录自动创建。
func NewLocalStorage(dir, urlBase string) (*LocalStorage, error) {
	if urlBase == "" {
		urlBase = "/files"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建上传目录失败: %w", err)
	}
	return &LocalStorage{dir: dir, urlBase: urlBase}, nil
}

func (s *LocalStorage) absPath(key string) (string, error) {
	clean := filepath.Clean("/" + key) // 强制以根相对，阻断 ../ 穿越
	p := filepath.Join(s.dir, clean)
	if !strings.HasPrefix(p, filepath.Clean(s.dir)+string(os.PathSeparator)) {
		return "", errors.New("illegal storage key")
	}
	return p, nil
}

func (s *LocalStorage) Put(ctx context.Context, key string, r io.Reader) error {
	p, err := s.absPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func (s *LocalStorage) Delete(_ context.Context, key string) error {
	p, err := s.absPath(key)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func (s *LocalStorage) PublicURL(key string) string {
	return s.urlBase + "/" + strings.TrimPrefix(key, "/")
}

// LocalPath 返回本地文件绝对路径（供静态文件服务使用）。
func (s *LocalStorage) LocalPath(key string) (string, bool) {
	p, err := s.absPath(key)
	if err != nil {
		return "", false
	}
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}
