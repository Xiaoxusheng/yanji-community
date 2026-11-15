// Package logger 提供结构化日志（slog）。
// 安全约定：禁止记录密码、Token、Cookie 与私信正文等敏感信息。
package logger

import (
	"log/slog"
	"os"
)

// New 按环境初始化 slog 默认 logger。
func New(env string) *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	if env == "development" {
		opts.Level = slog.LevelDebug
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}
	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
